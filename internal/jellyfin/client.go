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

// Shared HTTP transport for connection pooling across all jellyfin clients
var sharedTransport = &http.Transport{
	MaxIdleConns:        5,
	MaxIdleConnsPerHost: 3,
	IdleConnTimeout:     90 * time.Second,
	DisableKeepAlives:   false,
	MaxConnsPerHost:     3,
}

var sharedClient = &http.Client{
	Transport: sharedTransport,
	Timeout:  10 * time.Second,
}

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
		http: sharedClient,
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
