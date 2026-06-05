package discord

import (
	"testing"

	"diswatch/internal/config"
)

func TestFormatImageKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Empty string
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},

		// Already has mp: prefix
		{
			name:     "already has mp prefix",
			input:    "mp:https://example.com/image.png",
			expected: "mp:https://example.com/image.png",
		},

		// Standard uploaded asset keys (no scheme)
		{
			name:     "uploaded asset key",
			input:    "large_image_key",
			expected: "large_image_key",
		},
		{
			name:     "uploaded asset key with underscores",
			input:    "application_asset_123",
			expected: "application_asset_123",
		},

		// External URLs with https
		{
			name:     "tenor gif url",
			input:    "https://media.tenor.com/L0UOT9KSNQ0AAAAj/miyabi-spin.gif",
			expected: "mp:https://media.tenor.com/L0UOT9KSNQ0AAAAj/miyabi-spin.gif",
		},
		{
			name:     "giphy url",
			input:    "https://media.giphy.com/media/xxx/giphy.gif",
			expected: "mp:https://media.giphy.com/media/xxx/giphy.gif",
		},
		{
			name:     "imgur url",
			input:    "https://i.imgur.com/abc123.png",
			expected: "mp:https://i.imgur.com/abc123.png",
		},

		// External URLs with http (should still work)
		{
			name:     "http url",
			input:    "http://example.com/image.png",
			expected: "mp:http://example.com/image.png",
		},

		// URLs with non-standard port (should keep port)
		{
			name:     "url with non-standard port kept",
			input:    "https://example.com:8080/image.png",
			expected: "mp:https://example.com:8080/image.png",
		},

		// URLs with whitespace should be trimmed
		{
			name:     "url with leading whitespace",
			input:    "  https://example.com/image.png",
			expected: "mp:https://example.com/image.png",
		},
		{
			name:     "url with trailing whitespace",
			input:    "https://example.com/image.png  ",
			expected: "mp:https://example.com/image.png",
		},

		// Invalid URLs (no scheme)
		{
			name:     "no scheme treated as asset",
			input:    "example.com/image.png",
			expected: "example.com/image.png",
		},

		// Other schemes should be returned as-is
		{
			name:     "ftp scheme returns as-is",
			input:    "ftp://example.com/image.png",
			expected: "ftp://example.com/image.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatImageKey(tt.input)
			if result != tt.expected {
				t.Errorf("FormatImageKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFromCustomAt_ExternalImages(t *testing.T) {
	// Test that FormatImageKey is properly called for image fields
	cfg := config.Config{
		Discord: config.DiscordConfig{
			ApplicationID: "123456789",
			Status:        "online",
		},
		Custom: config.CustomPresence{
			Name:       "Test",
			Type:       0,
			Details:    "Test Details",
			State:      "Test State",
			LargeImage: "https://media.tenor.com/test.gif",
			LargeText:  "Large",
			SmallImage: "https://media.giphy.com/test.png",
			SmallText:  "Small",
			UseElapsed: true,
		},
	}

	presence := FromCustomAt(cfg, 1000000)

	if len(presence.Activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(presence.Activities))
	}

	activity := presence.Activities[0]

	// Verify images are formatted correctly with mp: prefix
	if activity.Assets == nil {
		t.Fatal("expected Assets to be set")
	}

	if activity.Assets.LargeImage != "mp:https://media.tenor.com/test.gif" {
		t.Errorf("LargeImage = %q, want mp:https://media.tenor.com/test.gif", activity.Assets.LargeImage)
	}

	if activity.Assets.SmallImage != "mp:https://media.giphy.com/test.png" {
		t.Errorf("SmallImage = %q, want mp:https://media.giphy.com/test.png", activity.Assets.SmallImage)
	}
}
