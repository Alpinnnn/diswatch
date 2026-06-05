package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
)

type Mode uint8

const (
	ModeJellyfin Mode = 1 << iota // bit 0: jellyfin watching activity
	ModeCustom                    // bit 1: custom presence
)

// ModeCustomOnly = ModeCustom (0b10)
// ModeJellyfinOnly = ModeJellyfin (0b01)
// ModeDual = ModeJellyfin | ModeCustom (0b11)
const (
	ModeDual Mode = ModeJellyfin | ModeCustom // both active
)

// UnmarshalJSON handles both string ("custom", "jellyfin", "dual") and numeric (1, 2, 3) formats
func (m *Mode) UnmarshalJSON(data []byte) error {
	// Try as string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		switch strings.ToLower(s) {
		case "custom":
			*m = ModeCustom
		case "jellyfin":
			*m = ModeJellyfin | ModeCustom
		case "dual":
			*m = ModeDual
		default:
			*m = ModeDual
		}
		return nil
	}

	// Try as number
	var n uint8
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*m = Mode(n)
	return nil
}

// MarshalJSON serializes mode as a number
func (m Mode) MarshalJSON() ([]byte, error) {
	return json.Marshal(uint8(m))
}

const DefaultGatewayURL = "wss://gateway.discord.gg/?v=10&encoding=json"

type Config struct {
	Mode              Mode           `json:"mode"`
	AdminPasswordHash string         `json:"admin_password_hash,omitempty"`
	Discord           DiscordConfig  `json:"discord"`
	Jellyfin          JellyfinConfig `json:"jellyfin"`
	Custom            CustomPresence `json:"custom"`
}

type DiscordConfig struct {
	Token         string `json:"token,omitempty"`
	GatewayURL    string `json:"gateway_url"`
	Status        string `json:"status"`
	ApplicationID string `json:"application_id,omitempty"`
}

type JellyfinConfig struct {
	URL                 string `json:"url,omitempty"`
	APIKey              string `json:"api_key,omitempty"`
	UserID              string `json:"user_id,omitempty"`
	Username            string `json:"username,omitempty"`
	WebhookSecret       string `json:"webhook_secret,omitempty"`
	PollIntervalSeconds int    `json:"poll_interval_seconds"`
	ClearOnPause        bool   `json:"clear_on_pause"`
	JellyfinName        string `json:"jellyfin_name,omitempty"` // Custom activity name for jellyfin mode
}

type CustomPresence struct {
	Name        string   `json:"name"`
	Type        int      `json:"type"`
	Details     string   `json:"details,omitempty"`
	DetailsURL  string   `json:"details_url,omitempty"`
	State       string   `json:"state,omitempty"`
	StateURL    string   `json:"state_url,omitempty"`
	LargeImage  string   `json:"large_image,omitempty"`
	LargeText   string   `json:"large_text,omitempty"`
	SmallImage  string   `json:"small_image,omitempty"`
	SmallText   string   `json:"small_text,omitempty"`
	UseElapsed  bool     `json:"use_elapsed"`
	Buttons     []Button `json:"buttons,omitempty"`
}

type Button struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

func Default() Config {
	return Config{
		Mode: ModeDual, // Both custom and jellyfin can run simultaneously
		Discord: DiscordConfig{
			GatewayURL: DefaultGatewayURL,
			Status:     "online",
		},
		Jellyfin: JellyfinConfig{
			WebhookSecret:       randomToken(24),
			PollIntervalSeconds: 30,
			ClearOnPause:        false,
		},
		Custom: CustomPresence{
			Name:        "Diswatch",
			Type:        0,
			Details:     "Custom presence",
			DetailsURL:  "",
			State:       "Configured locally",
			StateURL:    "",
			UseElapsed:  true,
		},
	}
}

func (c *Config) Normalize() {
	// Default to dual mode if mode is 0 (unset)
	if c.Mode == 0 {
		c.Mode = ModeDual
	}
	if strings.TrimSpace(c.Discord.GatewayURL) == "" {
		c.Discord.GatewayURL = DefaultGatewayURL
	}
	c.Discord.Status = normalizeStatus(c.Discord.Status)

	// Jellyfin settings
	if c.Jellyfin.PollIntervalSeconds < 15 {
		c.Jellyfin.PollIntervalSeconds = 30
	}
	if c.Jellyfin.PollIntervalSeconds > 3600 {
		c.Jellyfin.PollIntervalSeconds = 3600
	}
	if strings.TrimSpace(c.Jellyfin.WebhookSecret) == "" {
		c.Jellyfin.WebhookSecret = randomToken(24)
	}
	if strings.TrimSpace(c.Custom.Name) == "" {
		c.Custom.Name = "Diswatch"
	}
	if c.Custom.Type < 0 || c.Custom.Type > 5 {
		c.Custom.Type = 0
	}
	c.Custom.Buttons = cleanButtons(c.Custom.Buttons)
}

func (c Config) Initialized() bool {
	return c.AdminPasswordHash != ""
}

func normalizeStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "online", "idle", "dnd", "invisible":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return "online"
	}
}

func cleanButtons(buttons []Button) []Button {
	out := make([]Button, 0, 2)
	for _, button := range buttons {
		label := strings.TrimSpace(button.Label)
		url := strings.TrimSpace(button.URL)
		if label == "" || url == "" {
			continue
		}
		if len(label) > 32 {
			label = label[:32]
		}
		if len(url) > 512 {
			url = url[:512]
		}
		out = append(out, Button{Label: label, URL: url})
		if len(out) == 2 {
			break
		}
	}
	return out
}

func randomToken(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
