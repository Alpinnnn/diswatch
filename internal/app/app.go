package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"diswatch/internal/config"
	"diswatch/internal/discord"
	"diswatch/internal/jellyfin"
	"diswatch/internal/security"
)

type App struct {
	store    *config.Store
	logger   *slog.Logger
	discord  *discord.Client
	jellyfin *jellyfin.Client

	wakePoll chan struct{}

	statusMu       sync.RWMutex
	jellyfinStatus JellyfinStatus

	presenceMu             sync.Mutex
	customElapsedStart     int64
	customElapsedSignature string

	// Cached jellyfin presence to prevent race condition
	// When jellyfin session is active, cache it so fallback doesn't revert to custom
	jellyfinCache struct {
		presence  discord.Presence
		timestamp time.Time
		valid     bool
	}
}

type JellyfinStatus struct {
	LastPoll    time.Time `json:"last_poll,omitempty"`
	LastWebhook time.Time `json:"last_webhook,omitempty"`
	LastTitle   string    `json:"last_title,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	Active      bool      `json:"active"`
}

type Status struct {
	Mode     config.Mode    `json:"mode"`
	Discord  discord.Status `json:"discord"`
	Jellyfin JellyfinStatus `json:"jellyfin"`
}

func New(store *config.Store, logger *slog.Logger) *App {
	return &App{
		store:    store,
		logger:   logger,
		discord:  discord.NewClient(logger),
		jellyfin: jellyfin.NewClient(),
		wakePoll: make(chan struct{}, 1),
	}
}

func (a *App) Run(ctx context.Context) {
	cfg := a.store.Get()
	a.applyDiscordConfig(cfg)
	a.applyModePresence(cfg)

	go a.discord.Run(ctx)
	go a.pollLoop(ctx)

	<-ctx.Done()
}

func (a *App) Config() config.Config {
	return a.store.Get()
}

func (a *App) Status() Status {
	a.statusMu.RLock()
	jf := a.jellyfinStatus
	a.statusMu.RUnlock()
	return Status{
		Mode:     a.store.Get().Mode,
		Discord:  a.discord.Status(),
		Jellyfin: jf,
	}
}

// CurrentPresence returns the current presence being sent to Discord (for debugging)
func (a *App) CurrentPresence() discord.Presence {
	return a.discord.CurrentPresence()
}

func (a *App) Setup(password string) error {
	_, err := a.store.Update(func(cfg *config.Config) error {
		if cfg.Initialized() {
			return errors.New("application is already initialized")
		}
		hash, err := security.HashPassword(password)
		if err != nil {
			return err
		}
		cfg.AdminPasswordHash = hash
		return nil
	})
	return err
}

func (a *App) VerifyPassword(password string) bool {
	hash := a.store.Get().AdminPasswordHash
	return hash != "" && security.VerifyPassword(hash, password)
}

func (a *App) UpdateConfig(fn func(*config.Config) error) (config.Config, error) {
	cfg, err := a.store.Update(fn)
	if err != nil {
		return config.Config{}, err
	}
	a.applyDiscordConfig(cfg)
	a.applyModePresence(cfg)
	a.notifyPoll()
	return cfg, nil
}

func (a *App) ApplyWebhook(raw map[string]any, secret string) bool {
	cfg := a.store.Get()
	if cfg.Jellyfin.WebhookSecret == "" || secret != cfg.Jellyfin.WebhookSecret {
		return false
	}

	event := jellyfin.WebhookEventFromMap(raw)
	_, active := jellyfin.PresenceFromWebhook(event, cfg)
	a.setJellyfinStatus(func(status *JellyfinStatus) {
		status.LastWebhook = time.Now()
		status.LastTitle = event.Name
		status.Active = active
		status.LastError = ""
	})

	// Invalidate jellyfin cache when playback stops (to properly revert to custom)
	// or when paused (if ClearOnPause is enabled)
	if !active {
		a.presenceMu.Lock()
		a.jellyfinCache.valid = false
		a.presenceMu.Unlock()
	}

	// Update presence based on current mode
	a.applyModePresence(cfg)
	return true
}

func (a *App) ReconnectDiscord() {
	cfg := a.store.Get()
	token := cfg.Discord.Token

	// Clear token to force disconnect, then reconnect
	a.discord.Configure("", cfg.Discord.GatewayURL)

	// Wait for old connection to fully close
	time.Sleep(200 * time.Millisecond)

	// Reconfigure with token
	a.discord.Configure(token, cfg.Discord.GatewayURL)
}

func (a *App) TestJellyfin(ctx context.Context) error {
	cfg := a.store.Get()
	if strings.TrimSpace(cfg.Jellyfin.URL) == "" || strings.TrimSpace(cfg.Jellyfin.APIKey) == "" {
		return errors.New("jellyfin url and api key are required")
	}
	_, err := a.jellyfin.Sessions(ctx, jellyfin.Settings{
		URL:    cfg.Jellyfin.URL,
		APIKey: cfg.Jellyfin.APIKey,
		UserID: cfg.Jellyfin.UserID,
		User:   cfg.Jellyfin.Username,
	})
	return err
}

func (a *App) pollLoop(ctx context.Context) {
	for {
		cfg := a.store.Get()
		interval := time.Duration(cfg.Jellyfin.PollIntervalSeconds) * time.Second
		if interval < 15*time.Second {
			interval = 30 * time.Second
		}
		// Poll if jellyfin mode is enabled and configured
		if cfg.Mode&config.ModeJellyfin != 0 && cfg.Jellyfin.URL != "" && cfg.Jellyfin.APIKey != "" {
			a.pollOnce(ctx, cfg)
		}

		select {
		case <-ctx.Done():
			return
		case <-a.wakePoll:
		case <-time.After(interval):
		}
	}
}

func (a *App) pollOnce(ctx context.Context, cfg config.Config) {
	pollCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	sessions, err := a.jellyfin.Sessions(pollCtx, jellyfin.Settings{
		URL:    cfg.Jellyfin.URL,
		APIKey: cfg.Jellyfin.APIKey,
		UserID: cfg.Jellyfin.UserID,
		User:   cfg.Jellyfin.Username,
	})
	a.setJellyfinStatus(func(status *JellyfinStatus) {
		status.LastPoll = time.Now()
	})

	if err != nil {
		a.setJellyfinStatus(func(status *JellyfinStatus) {
			status.LastError = err.Error()
			status.Active = false
		})
		return
	}

	// Get active session info
	_, title, active := jellyfin.PresenceFromSessions(sessions, cfg)
	a.setJellyfinStatus(func(status *JellyfinStatus) {
		status.LastTitle = title
		status.LastError = ""
		status.Active = active
	})

	// Invalidate jellyfin cache when poll confirms no active session
	// This ensures cache expires properly when playback actually stops
	if !active {
		a.presenceMu.Lock()
		a.jellyfinCache.valid = false
		a.presenceMu.Unlock()
	}

	// Update presence with jellyfin overlay
	a.applyModePresence(cfg)
}

func (a *App) applyDiscordConfig(cfg config.Config) {
	a.discord.Configure(cfg.Discord.Token, cfg.Discord.GatewayURL)
}

const jellyfinCacheTTL = 2 * time.Minute // Keep jellyfin cache for 2 minutes after last seen

func (a *App) applyModePresence(cfg config.Config) {
	// Build presence based on dual mode:
	// - Custom is always the base presence
	// - Jellyfin activity overlays when watching

	// Always start with custom
	presence := discord.FromCustomAt(cfg, a.customElapsedStartFor(cfg))

	// Overlay jellyfin activity only if jellyfin mode is enabled AND configured
	if cfg.Mode&config.ModeJellyfin != 0 && cfg.Jellyfin.URL != "" && cfg.Jellyfin.APIKey != "" {
		// Get current jellyfin sessions to build activity with full details
		sessions, err := a.jellyfin.SessionsWithTimeout(context.Background(), 5*time.Second, jellyfin.Settings{
			URL:    cfg.Jellyfin.URL,
			APIKey: cfg.Jellyfin.APIKey,
			UserID: cfg.Jellyfin.UserID,
			User:   cfg.Jellyfin.Username,
		})
		if err == nil {
			jellyfinPresence, _, active := jellyfin.PresenceFromSessions(sessions, cfg)
			if active && len(jellyfinPresence.Activities) > 0 {
				// Use jellyfin activity but inherit custom settings
				activity := jellyfinPresence.Activities[0]
				activity.ApplicationID = cfg.Discord.ApplicationID

				// Override jellyfin name if custom name is set
				if cfg.Jellyfin.JellyfinName != "" {
					activity.Name = cfg.Jellyfin.JellyfinName
				}

				// Inherit custom buttons if jellyfin doesn't have any
				if len(activity.Buttons) == 0 && len(cfg.Custom.Buttons) > 0 {
					for _, btn := range cfg.Custom.Buttons {
						activity.Buttons = append(activity.Buttons, discord.Button{
							Label: btn.Label,
							URL:   btn.URL,
						})
					}
				}

				// Inherit custom assets
				if activity.Assets == nil && cfg.Custom.LargeImage != "" {
					activity.Assets = &discord.Assets{
						LargeImage: discord.FormatImageKey(cfg.Custom.LargeImage),
						LargeText:  cfg.Custom.LargeText,
						SmallImage: discord.FormatImageKey(cfg.Custom.SmallImage),
						SmallText:  cfg.Custom.SmallText,
					}
				}

				// Cache the jellyfin presence so subsequent calls don't revert to custom
				a.presenceMu.Lock()
				a.jellyfinCache.presence = discord.Presence{
					Activities: []discord.Activity{activity},
					Status:     presence.Status,
					AFK:        false,
				}
				a.jellyfinCache.timestamp = time.Now()
				a.jellyfinCache.valid = true
				a.presenceMu.Unlock()

				presence.Activities = []discord.Activity{activity}
			} else {
				// No active session - check if we should use cached jellyfin presence
				a.presenceMu.Lock()
				if a.jellyfinCache.valid && time.Since(a.jellyfinCache.timestamp) < jellyfinCacheTTL {
					// Still within TTL - use cached jellyfin presence
					presence = a.jellyfinCache.presence
					presence.Status = normalizeStatus(cfg.Discord.Status)
				} else {
					a.jellyfinCache.valid = false
				}
				a.presenceMu.Unlock()
			}
		} else {
			// Sessions fetch failed - check if we have cached jellyfin presence
			a.presenceMu.Lock()
			if a.jellyfinCache.valid && time.Since(a.jellyfinCache.timestamp) < jellyfinCacheTTL {
				// Use cached jellyfin presence to prevent reverting to custom
				presence = a.jellyfinCache.presence
				presence.Status = normalizeStatus(cfg.Discord.Status)
			}
			a.presenceMu.Unlock()
		}
	}

	a.discord.SetPresence(presence)
}

// normalizeStatus normalizes Discord status string
func normalizeStatus(status string) string {
	switch status {
	case "online", "idle", "dnd", "invisible":
		return status
	default:
		return "online"
	}
}

func (a *App) customElapsedStartFor(cfg config.Config) int64 {
	if !cfg.Custom.UseElapsed {
		a.presenceMu.Lock()
		a.customElapsedStart = 0
		a.customElapsedSignature = ""
		a.presenceMu.Unlock()
		return 0
	}

	signature := customElapsedSignature(cfg)
	now := time.Now().UnixMilli()

	a.presenceMu.Lock()
	defer a.presenceMu.Unlock()
	if a.customElapsedStart == 0 || a.customElapsedSignature != signature {
		a.customElapsedStart = now
		a.customElapsedSignature = signature
	}
	return a.customElapsedStart
}

func customElapsedSignature(cfg config.Config) string {
	raw, _ := json.Marshal(struct {
		ApplicationID string
		Custom        config.CustomPresence
	}{
		ApplicationID: cfg.Discord.ApplicationID,
		Custom:        cfg.Custom,
	})
	return string(raw)
}

func (a *App) notifyPoll() {
	select {
	case a.wakePoll <- struct{}{}:
	default:
	}
}

func (a *App) setJellyfinStatus(fn func(*JellyfinStatus)) {
	a.statusMu.Lock()
	defer a.statusMu.Unlock()
	fn(&a.jellyfinStatus)
}
