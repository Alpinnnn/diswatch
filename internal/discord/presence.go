package discord

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"

	"diswatch/internal/config"
)

type Presence struct {
	Since      *int64     `json:"since"`
	Activities []Activity `json:"activities"`
	Status     string     `json:"status"`
	AFK        bool       `json:"afk"`
}

type Activity struct {
	Name          string      `json:"name"`
	Type          int         `json:"type"`
	ApplicationID string      `json:"application_id,omitempty"`
	Details       string      `json:"details,omitempty"`
	DetailsURL    string      `json:"details_url,omitempty"`
	State         string      `json:"state,omitempty"`
	StateURL      string      `json:"state_url,omitempty"`
	Timestamps    *Timestamps `json:"timestamps,omitempty"`
	Assets        *Assets     `json:"assets,omitempty"`
	Buttons       []Button    `json:"buttons,omitempty"`
}

type Timestamps struct {
	Start int64 `json:"start,omitempty"`
	End   int64 `json:"end,omitempty"`
}

type Assets struct {
	LargeImage string `json:"large_image,omitempty"`
	LargeText  string `json:"large_text,omitempty"`
	SmallImage string `json:"small_image,omitempty"`
	SmallText  string `json:"small_text,omitempty"`
}

type Button struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

func ClearPresence(status string) Presence {
	return Presence{
		Since:      nil,
		Activities: []Activity{},
		Status:     normalizeStatus(status),
		AFK:        false,
	}
}

func FromCustomAt(cfg config.Config, elapsedStart int64) Presence {
	activity := Activity{
		Name:          cfg.Custom.Name,
		Type:          cfg.Custom.Type,
		ApplicationID: cfg.Discord.ApplicationID,
		Details:       cfg.Custom.Details,
		DetailsURL:    cfg.Custom.DetailsURL,
		State:         cfg.Custom.State,
		StateURL:      cfg.Custom.StateURL,
	}
	if cfg.Custom.UseElapsed && elapsedStart > 0 {
		activity.Timestamps = &Timestamps{Start: elapsedStart}
	}
	if cfg.Custom.LargeImage != "" || cfg.Custom.LargeText != "" || cfg.Custom.SmallImage != "" || cfg.Custom.SmallText != "" {
		activity.Assets = &Assets{
			LargeImage: FormatImageKey(cfg.Custom.LargeImage),
			LargeText:  cfg.Custom.LargeText,
			SmallImage: FormatImageKey(cfg.Custom.SmallImage),
			SmallText:  cfg.Custom.SmallText,
		}
	}
	for _, button := range cfg.Custom.Buttons {
		activity.Buttons = append(activity.Buttons, Button{Label: button.Label, URL: button.URL})
	}
	return Presence{
		Activities: []Activity{activity},
		Status:     normalizeStatus(cfg.Discord.Status),
		AFK:        false,
	}
}

// FormatImageKey normalizes uploaded asset keys and external image URLs.
// - If value starts with "mp:", it's returned as-is (media proxy prefix)
// - If value is a valid http/https URL, it's wrapped with "mp:" prefix for Discord media proxy
// - Otherwise, value is returned as-is (uploaded asset key)
func FormatImageKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "mp:") {
		return value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return value
	}
	// External URL - use Discord's media proxy prefix
	host := parsed.Hostname()
	if port := parsed.Port(); port != "" {
		host += ":" + port
	}
	result := "mp:" + parsed.Scheme + "://" + host + parsed.Path
	if parsed.RawQuery != "" {
		result += "?" + parsed.RawQuery
	}
	return result
}

func HashPresence(p Presence) string {
	raw, _ := json.Marshal(p)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func normalizeStatus(status string) string {
	switch status {
	case "online", "idle", "dnd", "invisible":
		return status
	default:
		return "online"
	}
}
