package jellyfin

import (
	"testing"

	"diswatch/internal/config"
)

func TestPresenceFromEpisodeSession(t *testing.T) {
	cfg := config.Default()
	session := Session{
		NowPlayingItem: &Item{
			Name:              "The Beginning",
			Type:              "Episode",
			SeriesName:        "Example Show",
			ParentIndexNumber: 1,
			IndexNumber:       2,
			RunTimeTicks:      1_800_000_000,
		},
		PlayState: PlayState{PositionTicks: 300_000_000},
	}
	presence, title, active := PresenceFromSessions([]Session{session}, cfg)
	if !active {
		t.Fatal("expected active presence")
	}
	if title != "The Beginning" {
		t.Fatalf("unexpected title: %s", title)
	}
	if got := presence.Activities[0].Details; got != "Example Show" {
		t.Fatalf("unexpected details: %s", got)
	}
	if got := presence.Activities[0].State; got != "S01E02 - The Beginning" {
		t.Fatalf("unexpected state: %s", got)
	}
}
