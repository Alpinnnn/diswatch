package jellyfin

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"diswatch/internal/config"
	"diswatch/internal/discord"
)

type WebhookEvent struct {
	NotificationType      string
	Name                  string
	ItemType              string
	ItemID                string
	SeriesName            string
	SeasonNumber          int
	EpisodeNumber         int
	Year                  int
	IsPaused              bool
	PlaybackPositionTicks int64
	RunTimeTicks          int64
}

func PresenceFromSessions(sessions []Session, cfg config.Config) (discord.Presence, string, bool) {
	for _, session := range sessions {
		if session.NowPlayingItem == nil {
			continue
		}
		if cfg.Jellyfin.ClearOnPause && session.PlayState.IsPaused {
			return discord.ClearPresence(cfg.Discord.Status), "", false
		}
		presence := presenceFromItem(*session.NowPlayingItem, session.PlayState.PositionTicks, session.PlayState.IsPaused, cfg)
		return presence, session.NowPlayingItem.Name, true
	}
	return discord.ClearPresence(cfg.Discord.Status), "", false
}

func WebhookEventFromMap(raw map[string]any) WebhookEvent {
	return WebhookEvent{
		NotificationType:      stringValue(raw, "NotificationType"),
		Name:                  stringValue(raw, "Name"),
		ItemType:              stringValue(raw, "ItemType"),
		ItemID:                stringValue(raw, "ItemId"),
		SeriesName:            stringValue(raw, "SeriesName"),
		SeasonNumber:          intValue(raw, "SeasonNumber"),
		EpisodeNumber:         intValue(raw, "EpisodeNumber"),
		Year:                  intValue(raw, "Year"),
		IsPaused:              boolValue(raw, "IsPaused"),
		PlaybackPositionTicks: int64Value(raw, "PlaybackPositionTicks"),
		RunTimeTicks:          int64Value(raw, "RunTimeTicks"),
	}
}

func PresenceFromWebhook(event WebhookEvent, cfg config.Config) (discord.Presence, bool) {
	switch strings.ToLower(event.NotificationType) {
	case "playbackstop":
		return discord.ClearPresence(cfg.Discord.Status), false
	case "playbackpause":
		event.IsPaused = true
	case "playbackstart", "playbackprogress", "playbackunpause":
	default:
		if event.Name == "" {
			return discord.ClearPresence(cfg.Discord.Status), false
		}
	}
	if cfg.Jellyfin.ClearOnPause && event.IsPaused {
		return discord.ClearPresence(cfg.Discord.Status), false
	}
	item := Item{
		ID:                event.ItemID,
		Name:              event.Name,
		Type:              event.ItemType,
		SeriesName:        event.SeriesName,
		IndexNumber:       event.EpisodeNumber,
		ParentIndexNumber: event.SeasonNumber,
		ProductionYear:    event.Year,
		RunTimeTicks:      event.RunTimeTicks,
	}
	return presenceFromItem(item, event.PlaybackPositionTicks, event.IsPaused, cfg), true
}

func presenceFromItem(item Item, positionTicks int64, paused bool, cfg config.Config) discord.Presence {
	// Use custom name if set, otherwise default to "Jellyfin"
	activityName := cfg.Jellyfin.JellyfinName
	if strings.TrimSpace(activityName) == "" {
		activityName = "Jellyfin"
	}

	activity := discord.Activity{
		Name:          activityName,
		Type:          3,
		ApplicationID: cfg.Discord.ApplicationID,
	}

	// Use custom assets if available
	if cfg.Custom.LargeImage != "" || cfg.Custom.LargeText != "" || cfg.Custom.SmallImage != "" || cfg.Custom.SmallText != "" {
		activity.Assets = &discord.Assets{
			LargeImage: discord.FormatImageKey(cfg.Custom.LargeImage),
			LargeText:  cfg.Custom.LargeText,
			SmallImage: discord.FormatImageKey(cfg.Custom.SmallImage),
			SmallText:  cfg.Custom.SmallText,
		}
	}

	if strings.EqualFold(item.Type, "Episode") {
		activity.Details = fallback(item.SeriesName, "Watching series")
		episode := item.Name
		if item.ParentIndexNumber > 0 && item.IndexNumber > 0 {
			episode = fmt.Sprintf("S%02dE%02d - %s", item.ParentIndexNumber, item.IndexNumber, item.Name)
		}
		activity.State = episode
	} else {
		activity.Details = fallback(item.Name, "Watching media")
		if item.ProductionYear > 0 {
			activity.State = fmt.Sprintf("%s (%d)", fallback(item.Type, "Video"), item.ProductionYear)
		} else {
			activity.State = fallback(item.Type, "Video")
		}
	}
	if paused {
		activity.State = "Paused - " + activity.State
	} else if item.RunTimeTicks > 0 {
		start, end := timestamps(positionTicks, item.RunTimeTicks)
		activity.Timestamps = &discord.Timestamps{Start: start, End: end}
	}
	return discord.Presence{
		Activities: []discord.Activity{activity},
		Status:     cfg.Discord.Status,
		AFK:        false,
	}
}

func timestamps(positionTicks, runtimeTicks int64) (int64, int64) {
	now := time.Now().UnixMilli()
	positionMs := ticksToMillis(positionTicks)
	runtimeMs := ticksToMillis(runtimeTicks)
	start := now - positionMs
	return start, start + runtimeMs
}

func ticksToMillis(ticks int64) int64 {
	return ticks / 10_000
}

func fallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func stringValue(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func intValue(raw map[string]any, key string) int {
	return int(int64Value(raw, key))
}

func int64Value(raw map[string]any, key string) int64 {
	value, ok := raw[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	default:
		return 0
	}
}

func boolValue(raw map[string]any, key string) bool {
	value, ok := raw[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, _ := strconv.ParseBool(typed)
		return parsed
	default:
		return false
	}
}
