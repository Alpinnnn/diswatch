package jellyfin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	http *http.Client
}

type Settings struct {
	URL    string
	APIKey string
	UserID string
	User   string
}

type Session struct {
	ID                   string    `json:"Id"`
	UserID               string    `json:"UserId"`
	UserName             string    `json:"UserName"`
	Client               string    `json:"Client"`
	DeviceName           string    `json:"DeviceName"`
	LastActivityDate     string    `json:"LastActivityDate"`
	LastPlaybackCheckIn  string    `json:"LastPlaybackCheckIn"`
	NowPlayingItem       *Item     `json:"NowPlayingItem"`
	PlayState            PlayState `json:"PlayState"`
	SupportsMediaControl bool      `json:"SupportsMediaControl"`
}

type Item struct {
	ID                 string   `json:"Id"`
	Name               string   `json:"Name"`
	Type               string   `json:"Type"`
	SeriesName         string   `json:"SeriesName"`
	SeasonName         string   `json:"SeasonName"`
	IndexNumber        int      `json:"IndexNumber"`
	ParentIndexNumber  int      `json:"ParentIndexNumber"`
	ProductionYear     int      `json:"ProductionYear"`
	RunTimeTicks       int64    `json:"RunTimeTicks"`
	BackdropDropPath   string   `json:"BackdropDropPath"`
	Backdrops          []string `json:"Backdrops"`
}

type PlayState struct {
	PositionTicks int64 `json:"PositionTicks"`
	IsPaused      bool  `json:"IsPaused"`
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Sessions(ctx context.Context, settings Settings) ([]Session, error) {
	base, err := url.Parse(strings.TrimRight(settings.URL, "/"))
	if err != nil {
		return nil, err
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/Sessions"
	query := base.Query()
	query.Set("activeWithinSeconds", "900")
	base.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return nil, err
	}
	addAuthHeaders(req, settings.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("jellyfin sessions returned %s", resp.Status)
	}
	var sessions []Session
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, err
	}
	return filterSessions(sessions, settings), nil
}

// SessionsWithTimeout fetches sessions with a custom timeout context
func (c *Client) SessionsWithTimeout(ctx context.Context, timeout time.Duration, settings Settings) ([]Session, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Sessions(ctx, settings)
}

// GetItemBackdrop fetches the backdrop URL for a given item ID from Jellyfin API
func (c *Client) GetItemBackdrop(ctx context.Context, settings Settings, itemID string) (string, error) {
	base, err := url.Parse(strings.TrimRight(settings.URL, "/"))
	if err != nil {
		return "", err
	}
	base.Path = strings.TrimRight(base.Path, "/") + fmt.Sprintf("/Items/%s", itemID)
	query := base.Query()
	query.Set("api_key", settings.APIKey)
	base.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return "", err
	}
	addAuthHeaders(req, settings.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("jellyfin get item returned %s", resp.Status)
	}

	var item Item
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return "", err
	}

	// Prefer BackdropDropPath, fallback to first element of Backdrops array
	if item.BackdropDropPath != "" {
		return buildBackdropURL(settings.URL, item.BackdropDropPath), nil
	}
	if len(item.Backdrops) > 0 {
		return buildBackdropURL(settings.URL, item.Backdrops[0]), nil
	}
	return "", nil
}

func buildBackdropURL(baseURL, path string) string {
	base := strings.TrimRight(baseURL, "/")
	if strings.HasPrefix(path, "/") {
		return base + path
	}
	return base + "/" + path
}

func addAuthHeaders(req *http.Request, token string) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf(`MediaBrowser Token="%s", Client="Diswatch", Device="Diswatch", DeviceId="diswatch", Version="0.1.0"`, token))
	req.Header.Set("X-MediaBrowser-Token", token)
}

func filterSessions(sessions []Session, settings Settings) []Session {
	userID := strings.TrimSpace(settings.UserID)
	username := strings.TrimSpace(settings.User)
	if userID == "" && username == "" {
		return sessions
	}
	out := make([]Session, 0, len(sessions))
	for _, session := range sessions {
		if userID != "" && session.UserID == userID {
			out = append(out, session)
			continue
		}
		if username != "" && strings.EqualFold(session.UserName, username) {
			out = append(out, session)
		}
	}
	return out
}
