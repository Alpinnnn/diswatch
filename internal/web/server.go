package web

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"diswatch/internal/app"
	"diswatch/internal/config"
	"diswatch/internal/discord"
	"diswatch/internal/security"
	"diswatch/internal/version"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	app      *app.App
	logger   *slog.Logger
	sessions *security.SessionManager
	static   http.Handler
}

func NewServer(runtime *app.App, logger *slog.Logger) http.Handler {
	sub, _ := fs.Sub(staticFiles, "static")
	server := &Server{
		app:      runtime,
		logger:   logger.With("component", "web"),
		sessions: security.NewSessionManager(12 * time.Hour),
		static:   http.FileServer(http.FS(sub)),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("GET /api/bootstrap", server.bootstrap)
	mux.HandleFunc("GET /api/version", server.version)
	mux.HandleFunc("POST /api/setup", server.setup)
	mux.HandleFunc("POST /api/login", server.login)
	mux.HandleFunc("POST /api/jellyfin/webhook", server.jellyfinWebhook)

	mux.Handle("GET /api/config", server.auth(http.HandlerFunc(server.getConfig)))
	mux.Handle("PUT /api/config", server.auth(http.HandlerFunc(server.updateConfig)))
	mux.Handle("GET /api/status", server.auth(http.HandlerFunc(server.status)))
	mux.Handle("GET /api/presence/debug", server.auth(http.HandlerFunc(server.debugPresence)))
	mux.Handle("POST /api/logout", server.auth(http.HandlerFunc(server.logout)))
	mux.Handle("POST /api/discord/reconnect", server.auth(http.HandlerFunc(server.reconnectDiscord)))
	mux.Handle("POST /api/jellyfin/test", server.auth(http.HandlerFunc(server.testJellyfin)))
	mux.Handle("POST /api/mode", server.auth(http.HandlerFunc(server.switchMode)))
	mux.Handle("GET /", server.securityHeaders(http.HandlerFunc(server.index)))
	mux.Handle("GET /styles.css", server.securityHeaders(server.static))
	mux.Handle("GET /app.js", server.securityHeaders(server.static))
	return server.recover(server.securityHeaders(mux))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version":    version.Version,
		"commit":     version.Commit,
		"build_time": version.BuildTime,
		"full":       version.FullString(),
	})
}

func (s *Server) bootstrap(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"initialized": s.app.Config().Initialized(),
		"version":      version.String(),
		"version_full": version.FullString(),
	})
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	if s.app.Config().Initialized() {
		writeError(w, http.StatusConflict, "already initialized")
		return
	}
	var req passwordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.app.Setup(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.sessions.Create(w, r); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.app.Config().Initialized() {
		writeError(w, http.StatusPreconditionRequired, "setup required")
		return
	}
	var req passwordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !s.app.VerifyPassword(req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}
	if err := s.sessions.Create(w, r); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	s.sessions.Destroy(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) getConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, publicConfig(s.app.Config()))
}

func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	var req updateConfigRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	cfg, err := s.app.UpdateConfig(func(cfg *config.Config) error {
		applyConfigRequest(cfg, req)
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, publicConfig(cfg))
}

func (s *Server) switchMode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode string `json:"mode"` // "dual", "custom", "jellyfin"
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	// Parse mode string to bit flags
	var mode config.Mode
	switch strings.ToLower(req.Mode) {
	case "dual", "":
		mode = config.ModeDual
	case "custom":
		mode = config.ModeCustom
	case "jellyfin":
		mode = config.ModeJellyfin | config.ModeCustom
	default:
		writeError(w, http.StatusBadRequest, "invalid mode")
		return
	}

	cfg, err := s.app.UpdateConfig(func(cfg *config.Config) error {
		cfg.Mode = mode
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, publicConfig(cfg))
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.app.Status())
}

// debugPresence returns the current presence as JSON for debugging
func (s *Server) debugPresence(w http.ResponseWriter, _ *http.Request) {
	presence := s.app.CurrentPresence()

	// Get current config to show what will be sent
	cfg := s.app.Config()

	// Build debug info
	debugInfo := map[string]any{
		"presence": presence,
		"config": map[string]any{
			"custom_name":     cfg.Custom.Name,
			"custom_type":     cfg.Custom.Type,
			"custom_details":  cfg.Custom.Details,
			"custom_state":    cfg.Custom.State,
			"custom_images": map[string]string{
				"large_image_raw": cfg.Custom.LargeImage,
				"large_image_formatted": discord.FormatImageKey(cfg.Custom.LargeImage),
				"small_image_raw": cfg.Custom.SmallImage,
				"small_image_formatted": discord.FormatImageKey(cfg.Custom.SmallImage),
			},
			"application_id": cfg.Discord.ApplicationID,
		},
	}

	writeJSON(w, http.StatusOK, debugInfo)
}

func (s *Server) reconnectDiscord(w http.ResponseWriter, _ *http.Request) {
	go s.app.ReconnectDiscord()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) testJellyfin(w http.ResponseWriter, r *http.Request) {
	if err := s.app.TestJellyfin(r.Context()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) jellyfinWebhook(w http.ResponseWriter, r *http.Request) {
	var raw map[string]any
	if !decodeJSON(w, r, &raw) {
		return
	}
	secret := r.URL.Query().Get("secret")
	if header := r.Header.Get("X-Diswatch-Webhook-Secret"); header != "" {
		secret = header
	}
	if !s.constantTimeWebhookSecret(secret) || !s.app.ApplyWebhook(raw, secret) {
		writeError(w, http.StatusUnauthorized, "invalid webhook secret")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) constantTimeWebhookSecret(secret string) bool {
	expected := s.app.Config().Jellyfin.WebhookSecret
	if expected == "" || secret == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(secret)) == 1
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	raw, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "index not found")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(raw)
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.app.Config().Initialized() {
			writeError(w, http.StatusPreconditionRequired, "setup required")
			return
		}
		if !s.sessions.Valid(r) {
			writeError(w, http.StatusUnauthorized, "login required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("request panic", "err", recovered)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type passwordRequest struct {
	Password string `json:"password"`
}

type updateConfigRequest struct {
	Mode    *config.Mode `json:"mode,omitempty"`
	Discord struct {
		Token         *string `json:"token,omitempty"`
		GatewayURL    *string `json:"gateway_url,omitempty"`
		Status        *string `json:"status,omitempty"`
		ApplicationID *string `json:"application_id,omitempty"`
		ClearToken    bool    `json:"clear_token,omitempty"`
	} `json:"discord"`
	Jellyfin struct {
		URL                 *string `json:"url,omitempty"`
		APIKey              *string `json:"api_key,omitempty"`
		UserID              *string `json:"user_id,omitempty"`
		Username            *string `json:"username,omitempty"`
		JellyfinName        *string `json:"jellyfin_name,omitempty"`
		WebhookSecret       *string `json:"webhook_secret,omitempty"`
		PollIntervalSeconds *int    `json:"poll_interval_seconds,omitempty"`
		ClearOnPause        *bool   `json:"clear_on_pause,omitempty"`
		ClearAPIKey         bool    `json:"clear_api_key,omitempty"`
	} `json:"jellyfin"`
	Custom *config.CustomPresence `json:"custom,omitempty"`
}

func applyConfigRequest(cfg *config.Config, req updateConfigRequest) {
	if req.Mode != nil {
		cfg.Mode = *req.Mode
	}
	if req.Discord.ClearToken {
		cfg.Discord.Token = ""
	} else if req.Discord.Token != nil {
		cfg.Discord.Token = strings.TrimSpace(*req.Discord.Token)
	}
	if req.Discord.GatewayURL != nil {
		cfg.Discord.GatewayURL = strings.TrimSpace(*req.Discord.GatewayURL)
	}
	if req.Discord.Status != nil {
		cfg.Discord.Status = strings.TrimSpace(*req.Discord.Status)
	}
	if req.Discord.ApplicationID != nil {
		cfg.Discord.ApplicationID = strings.TrimSpace(*req.Discord.ApplicationID)
	}

	if req.Jellyfin.URL != nil {
		cfg.Jellyfin.URL = strings.TrimRight(strings.TrimSpace(*req.Jellyfin.URL), "/")
	}
	if req.Jellyfin.ClearAPIKey {
		cfg.Jellyfin.APIKey = ""
	} else if req.Jellyfin.APIKey != nil {
		cfg.Jellyfin.APIKey = strings.TrimSpace(*req.Jellyfin.APIKey)
	}
	if req.Jellyfin.UserID != nil {
		cfg.Jellyfin.UserID = strings.TrimSpace(*req.Jellyfin.UserID)
	}
	if req.Jellyfin.Username != nil {
		cfg.Jellyfin.Username = strings.TrimSpace(*req.Jellyfin.Username)
	}
	if req.Jellyfin.JellyfinName != nil {
		cfg.Jellyfin.JellyfinName = strings.TrimSpace(*req.Jellyfin.JellyfinName)
	}
	if req.Jellyfin.WebhookSecret != nil {
		cfg.Jellyfin.WebhookSecret = strings.TrimSpace(*req.Jellyfin.WebhookSecret)
	}
	if req.Jellyfin.PollIntervalSeconds != nil {
		cfg.Jellyfin.PollIntervalSeconds = *req.Jellyfin.PollIntervalSeconds
	}
	if req.Jellyfin.ClearOnPause != nil {
		cfg.Jellyfin.ClearOnPause = *req.Jellyfin.ClearOnPause
	}
	if req.Custom != nil {
		cfg.Custom = *req.Custom
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}
