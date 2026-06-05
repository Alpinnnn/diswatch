package web

import (
	"diswatch/internal/config"
)

type publicConfigResponse struct {
	Initialized bool                  `json:"initialized"`
	Mode        config.Mode           `json:"mode"`
	Discord     publicDiscordConfig   `json:"discord"`
	Jellyfin    publicJellyfinConfig  `json:"jellyfin"`
	Custom      config.CustomPresence `json:"custom"`
}

type publicDiscordConfig struct {
	TokenSet      bool   `json:"token_set"`
	TokenMasked   string `json:"token_masked,omitempty"`
	GatewayURL    string `json:"gateway_url"`
	Status        string `json:"status"`
	ApplicationID string `json:"application_id,omitempty"`
}

type publicJellyfinConfig struct {
	URL                 string `json:"url,omitempty"`
	APIKeySet           bool   `json:"api_key_set"`
	APIKeyMasked        string `json:"api_key_masked,omitempty"`
	UserID              string `json:"user_id,omitempty"`
	Username            string `json:"username,omitempty"`
	JellyfinName        string `json:"jellyfin_name,omitempty"`
	WebhookSecret       string `json:"webhook_secret,omitempty"`
	PollIntervalSeconds int    `json:"poll_interval_seconds"`
	ClearOnPause        bool   `json:"clear_on_pause"`
}

func publicConfig(cfg config.Config) publicConfigResponse {
	return publicConfigResponse{
		Initialized: cfg.Initialized(),
		Mode:        cfg.Mode,
		Discord: publicDiscordConfig{
			TokenSet:      cfg.Discord.Token != "",
			TokenMasked:   maskSecret(cfg.Discord.Token),
			GatewayURL:    cfg.Discord.GatewayURL,
			Status:        cfg.Discord.Status,
			ApplicationID: cfg.Discord.ApplicationID,
		},
		Jellyfin: publicJellyfinConfig{
			URL:                 cfg.Jellyfin.URL,
			APIKeySet:           cfg.Jellyfin.APIKey != "",
			APIKeyMasked:        maskSecret(cfg.Jellyfin.APIKey),
			UserID:              cfg.Jellyfin.UserID,
			Username:            cfg.Jellyfin.Username,
			JellyfinName:        cfg.Jellyfin.JellyfinName,
			WebhookSecret:       cfg.Jellyfin.WebhookSecret,
			PollIntervalSeconds: cfg.Jellyfin.PollIntervalSeconds,
			ClearOnPause:        cfg.Jellyfin.ClearOnPause,
		},
		Custom: cfg.Custom,
	}
}

func maskSecret(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) <= 8 {
		return "********"
	}
	return secret[:4] + "..." + secret[len(secret)-4:]
}

// ModeDisplay returns a human-readable mode string
func (c publicConfigResponse) ModeDisplay() string {
	hasCustom := c.Mode&config.ModeCustom != 0
	hasJellyfin := c.Mode&config.ModeJellyfin != 0

	if hasCustom && hasJellyfin {
		return "Dual"
	}
	if hasCustom {
		return "Custom Only"
	}
	if hasJellyfin {
		return "Jellyfin Only"
	}
	return "Unknown"
}

// ModeFlags returns whether custom and jellyfin modes are enabled
func (c publicConfigResponse) ModeFlags() (custom, jellyfin bool) {
	return c.Mode&config.ModeCustom != 0, c.Mode&config.ModeJellyfin != 0
}
