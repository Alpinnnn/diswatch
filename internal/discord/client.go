package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	opDispatch       = 0
	opHeartbeat      = 1
	opIdentify       = 2
	opPresenceUpdate = 3
	opReconnect      = 7
	opInvalidSession = 9
	opHello          = 10
	opHeartbeatACK   = 11

	minPresenceInterval = 15 * time.Second
)

var errReconnect = errors.New("gateway requested reconnect")

type Client struct {
	logger *slog.Logger

	mu         sync.RWMutex
	token      string
	gatewayURL string
	cancelRun  context.CancelFunc
	desired    Presence

	wakeCh   chan struct{}
	updateCh chan struct{}

	statusMu sync.RWMutex
	status   Status
}

type Status struct {
	Configured bool      `json:"configured"`
	Connected  bool      `json:"connected"`
	LastError  string    `json:"last_error,omitempty"`
	LastUpdate time.Time `json:"last_update,omitempty"`

	// Track connection state more reliably
	connectedSince int64 // Unix timestamp when connection was established
}

type gatewayPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
	S  *int64          `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

func NewClient(logger *slog.Logger) *Client {
	return &Client{
		logger:     logger.With("component", "discord"),
		wakeCh:     make(chan struct{}, 1),
		updateCh:   make(chan struct{}, 1),
		gatewayURL: "wss://gateway.discord.gg/?v=10&encoding=json",
		desired:    ClearPresence("online"),
	}
}

func (c *Client) Configure(token, gatewayURL string) {
	if gatewayURL == "" {
		gatewayURL = "wss://gateway.discord.gg/?v=10&encoding=json"
	}

	c.mu.Lock()
	changed := c.token != token || c.gatewayURL != gatewayURL
	c.token = token
	c.gatewayURL = gatewayURL
	if changed && c.cancelRun != nil {
		c.cancelRun()
	}
	c.mu.Unlock()

	c.setStatus(func(s *Status) {
		s.Configured = token != ""
		if token == "" {
			s.Connected = false
			s.connectedSince = 0
			s.LastError = ""
		}
	})
	c.notify(c.wakeCh)
}

func (c *Client) SetPresence(p Presence) {
	c.mu.Lock()
	c.desired = p
	c.mu.Unlock()
	c.notify(c.updateCh)
}

func (c *Client) Status() Status {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.status
}

func (c *Client) Run(ctx context.Context) {
	backoff := 5 * time.Second
	for {
		token, gatewayURL := c.connectionConfig()
		if token == "" {
			select {
			case <-ctx.Done():
				return
			case <-c.wakeCh:
				continue
			}
		}

		runCtx, cancel := context.WithCancel(ctx)
		c.mu.Lock()
		c.cancelRun = cancel
		c.mu.Unlock()

		err := c.runConnection(runCtx, token, gatewayURL)
		cancel()
		c.setStatus(func(s *Status) {
			s.Connected = false
			s.connectedSince = 0
			if err != nil && !errors.Is(err, context.Canceled) {
				s.LastError = err.Error()
			}
		})

		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, context.Canceled) {
			backoff = 5 * time.Second
			continue
		}
		c.logger.Warn("discord gateway disconnected", "err", err, "retry_in", backoff)
		select {
		case <-ctx.Done():
			return
		case <-c.wakeCh:
			backoff = 5 * time.Second
		case <-time.After(backoff):
			if backoff < time.Minute {
				backoff *= 2
			}
		}
	}
}

func (c *Client) runConnection(ctx context.Context, token, gatewayURL string) error {
	dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(dialCtx, gatewayURL, &websocket.DialOptions{
		CompressionMode: websocket.CompressionContextTakeover,
	})
	if err != nil {
		return fmt.Errorf("dial gateway: %w", err)
	}

	// Set higher read limit for large messages (READYY payload can be several MB)
	conn.SetReadLimit(16 * 1024 * 1024) // 16MB
	defer conn.CloseNow()

	var writeMu sync.Mutex
	send := func(op int, data any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return wsjson.Write(writeCtx, conn, map[string]any{
			"op": op,
			"d":  data,
		})
	}

	var hello gatewayPayload
	if err := wsjson.Read(ctx, conn, &hello); err != nil {
		return fmt.Errorf("read hello: %w", err)
	}
	if hello.Op != opHello {
		return fmt.Errorf("unexpected gateway hello opcode %d", hello.Op)
	}
	var helloData struct {
		HeartbeatInterval int `json:"heartbeat_interval"`
	}
	if err := json.Unmarshal(hello.D, &helloData); err != nil {
		return fmt.Errorf("decode hello: %w", err)
	}
	if helloData.HeartbeatInterval <= 0 {
		return errors.New("gateway sent invalid heartbeat interval")
	}

	var seq atomic.Int64
	var hasSeq atomic.Bool
	var acked atomic.Bool
	acked.Store(true)

	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()
	go c.heartbeatLoop(heartbeatCtx, time.Duration(helloData.HeartbeatInterval)*time.Millisecond, send, &seq, &hasSeq, &acked, conn.CloseNow)

	initial := c.desiredPresence()
	identify := map[string]any{
		"token": token,
		"properties": map[string]string{
			"os":      "linux",
			"browser": "diswatch",
			"device":  "diswatch",
		},
		"compress":        false,
		"large_threshold": 50,
		"intents":         0,
		"presence":        initial,
	}
	if err := send(opIdentify, identify); err != nil {
		return fmt.Errorf("identify gateway: %w", err)
	}

	errCh := make(chan error, 1)
	go c.presenceWriter(ctx, send, HashPresence(initial), errCh)

	c.setStatus(func(s *Status) {
		s.Configured = true
		s.Connected = true
		s.LastError = ""
		s.connectedSince = time.Now().Unix()
	})

	for {
		var payload gatewayPayload
		if err := wsjson.Read(ctx, conn, &payload); err != nil {
			return err
		}
		if payload.S != nil {
			seq.Store(*payload.S)
			hasSeq.Store(true)
		}
		select {
		case err := <-errCh:
			return err
		default:
		}
		switch payload.Op {
		case opHeartbeatACK:
			acked.Store(true)
		case opHeartbeat:
			if err := send(opHeartbeat, currentSeq(&seq, &hasSeq)); err != nil {
				return fmt.Errorf("send requested heartbeat: %w", err)
			}
		case opReconnect:
			return errReconnect
		case opInvalidSession:
			return errors.New("gateway invalid session")
		}
	}
}

func (c *Client) heartbeatLoop(ctx context.Context, interval time.Duration, send func(int, any) error, seq *atomic.Int64, hasSeq *atomic.Bool, acked *atomic.Bool, closeConn func() error) {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	missedHeartbeats := 0
	maxMissedHeartbeats := 2

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if !acked.Load() {
				missedHeartbeats++
				c.logger.Warn("heartbeat ack missing", "missed", missedHeartbeats)
				// Only close after multiple missed heartbeats (more tolerant)
				if missedHeartbeats >= maxMissedHeartbeats {
					c.logger.Warn("too many missed heartbeats; closing gateway session")
					_ = closeConn()
					return
				}
			} else {
				missedHeartbeats = 0 // Reset on successful ACK
			}
			acked.Store(false)
			if err := send(opHeartbeat, currentSeq(seq, hasSeq)); err != nil {
				c.logger.Warn("heartbeat send failed", "err", err)
				_ = closeConn()
				return
			}
			timer.Reset(interval)
		}
	}
}

func (c *Client) presenceWriter(ctx context.Context, send func(int, any) error, lastSent string, errCh chan<- error) {
	var lastUpdate time.Time
	var timer *time.Timer
	var timerC <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-c.updateCh:
		case <-timerC:
			timerC = nil
		}

		presence := c.desiredPresence()
		hash := HashPresence(presence)
		if hash == lastSent {
			continue
		}
		if wait := time.Until(lastUpdate.Add(minPresenceInterval)); !lastUpdate.IsZero() && wait > 0 {
			if timer == nil {
				timer = time.NewTimer(wait)
			} else {
				timer.Reset(wait)
			}
			timerC = timer.C
			continue
		}
		// Debug: log the presence JSON being sent
		presenceJSON, _ := json.Marshal(presence)
		c.logger.Debug("sending presence update", "presence", string(presenceJSON))
		if err := send(opPresenceUpdate, presence); err != nil {
			select {
			case errCh <- fmt.Errorf("send presence: %w", err):
			default:
			}
			return
		}
		lastSent = hash
		lastUpdate = time.Now()
		c.setStatus(func(s *Status) {
			s.LastUpdate = lastUpdate
		})
	}
}

func currentSeq(seq *atomic.Int64, hasSeq *atomic.Bool) any {
	if !hasSeq.Load() {
		return nil
	}
	return seq.Load()
}

func (c *Client) connectionConfig() (string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token, c.gatewayURL
}

func (c *Client) desiredPresence() Presence {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.desired
}

// CurrentPresence returns the current presence that will be sent to Discord
func (c *Client) CurrentPresence() Presence {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.desired
}

func (c *Client) setStatus(fn func(*Status)) {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	fn(&c.status)
}

func (c *Client) notify(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
