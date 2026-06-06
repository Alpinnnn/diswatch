# Diswatch

> Personal Discord RPC Automator for Custom Presence and Jellyfin watch activity

A lightweight, self-contained Discord Rich Presence manager designed for resource-constrained environments like ARM64 set-top boxes running CasaOS.

[![ARM64](https://img.shields.io/badge/Architecture-ARM64-brightgreen)](https://github.com/Alpinnnn/diswatch)
[![Go](https://img.shields.io/badge/Go-1.26-blue)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue)](https://www.docker.com/)

## Features

### 🎯 Dual Mode Operation

- **Custom Mode:** Set your own Discord presence with details, state, images, elapsed timer, and up to two buttons
- **Jellyfin Mode:** Automatically display what you're watching from Jellyfin as your Discord status
- **Dual Mode:** Run both simultaneously - Jellyfin activity overlays your custom presence when watching

### 🔒 Security First

- Encrypted local config storage (AES-256-GCM)
- Argon2id password hashing
- Session-based authentication
- Tokens never exposed after saving
- Constant-time webhook secret comparison

### 🚀 Lightweight & Efficient

| Resource | Usage |
|----------|-------|
| Binary Size | ~8 MB |
| Docker Image | ~12 MB |
| RAM (idle) | ~35 MB |
| RAM (active) | ~60 MB |
| Startup Time | ~0.5s |

Optimized for 2GB RAM devices like Fiberhome HG680P STB.

### 🎨 Web Dashboard

- Clean, responsive UI
- Real-time status monitoring
- Mode switching
- Discord/Jellyfin configuration
- Reconnection handling

## Quick Start

### Docker Compose (Recommended)

```yaml
services:
  diswatch:
    image: diswatch:local
    container_name: diswatch
    platform: linux/arm64
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      DISWATCH_ADDR: ":8080"
      DISWATCH_DATA_DIR: "/app/data"
      # Recommended: set a stable base64-encoded 32-byte key
      # DISWATCH_SECRET_KEY: "your-32-byte-key-here"
    volumes:
      - ./data:/app/data
    read_only: true
    tmpfs:
      - /tmp:size=16m,mode=1777
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
```

### Build from Source

```bash
# Clone and build
git clone https://github.com/Alpinnnn/diswatch.git
cd diswatch

# Build ARM64 image
docker buildx build --platform linux/arm64 -t diswatch:local .

# Or run locally
go mod tidy
go run ./cmd/diswatch
```

## Configuration

### First-Time Setup

1. Open `http://your-host:8080`
2. Create your admin password (min 8 characters)
3. Configure Discord token and Jellyfin (optional)

### Discord Token

1. Open Discord in browser
2. Press `Ctrl+Shift+I` to open Developer Tools
3. Go to **Network** tab
4. Filter by `api.discord.com`
5. Find any request → Headers → Authorization
6. Copy the token (User token, not Bot token)
7. Paste into Diswatch dashboard

### Jellyfin Webhook Setup

1. Install Jellyfin Webhook Plugin
2. Configure webhook endpoint:

```
http://<your-host>:8080/api/jellyfin/webhook?secret=<your-secret>
```

3. Use this JSON template:

```json
{
  "NotificationType": "{{NotificationType}}",
  "Name": "{{Name}}",
  "ItemType": "{{ItemType}}",
  "ItemId": "{{ItemId}}",
  "SeriesName": "{{SeriesName}}",
  "SeasonNumber": "{{SeasonNumber}}",
  "EpisodeNumber": "{{EpisodeNumber}}",
  "Year": "{{Year}}",
  "IsPaused": "{{IsPaused}}",
  "PlaybackPositionTicks": "{{PlaybackPositionTicks}}",
  "RunTimeTicks": "{{RunTimeTicks}}"
}
```

4. Enable notification types: playback start, progress, pause, unpause, stop

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/healthz` | GET | Health check |
| `/api/bootstrap` | GET | Check initialization status |
| `/api/setup` | POST | Initial admin password setup |
| `/api/login` | POST | Authenticate |
| `/api/logout` | POST | End session |
| `/api/config` | GET/PUT | Get/update configuration |
| `/api/status` | GET | Current system status |
| `/api/mode` | POST | Switch mode (custom/jellyfin/dual) |
| `/api/discord/reconnect` | POST | Reconnect to Discord |
| `/api/jellyfin/test` | POST | Test Jellyfin connection |
| `/api/jellyfin/webhook` | POST | Jellyfin webhook receiver |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DISWATCH_ADDR` | `:8080` | Listen address |
| `DISWATCH_DATA_DIR` | `./data` | Config storage directory |
| `DISWATCH_SECRET_KEY` | (generated) | Base64 32-byte encryption key |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Web UI (Vanilla)                       │
│                   index.html + app.js                       │
└─────────────────────────┬───────────────────────────────────┘
                          │ HTTP
┌─────────────────────────▼───────────────────────────────────┐
│                   HTTP Server (Go)                          │
│               Session Auth + Security Headers               │
└─────────────────────────┬───────────────────────────────────┘
                          │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
    ┌────▼────┐      ┌─────▼────┐     ┌─────▼─────┐
    │ Config  │      │  App     │     │  Webhook  │
    │ Store   │      │ Presence │     │  Handler  │
    │ (AES)   │      │ Manager  │     │           │
    └─────────┘      └─────┬────┘     └───────────┘
                          │
         ┌────────────────┼────────────────┐
         │                │                │
    ┌────▼────┐     ┌─────▼────┐    ┌──────▼──────┐
    │ Discord │     │ Jellyfin │    │  Security   │
    │Gateway  │     │  Client  │    │   (Auth)    │
    │ (WS)    │     │ (HTTP)   │    │  (Argon2)   │
    └─────────┘     └──────────┘    └─────────────┘
```

## Performance Benchmarks

Tested on Fiberhome HG680P (ARM64, 2GB RAM):

```
=== System Resource Usage ===
Binary size:        6.5 MB
Memory (RSS):       6 MB
Memory (%MEM):      0.3%
File Descriptors:   7
Startup time:       ~60ms

=== HTTP Response Times ===
First request:      60ms
Subsequent:         ~52ms average

=== Discord Gateway ===
Connection time:    ~800ms
Heartbeat:          30s interval
Reconnect:          5s backoff (doubles)
```

### Real-World Results

```
 PID    VSZ   RSS %MEM COMMAND
 562506 1265352 6044  0.3 diswatch
```

This means Diswatch uses only **0.3% of available RAM** on a 2GB device while maintaining full functionality.

## Important Discord Notice

> Discord states that automating normal user accounts outside the OAuth2/bot API is forbidden and may lead to account termination. This app keeps the implementation local, rate-limited, and focused only on presence updates, but it cannot remove that platform risk. Use at your own discretion.

## License

MIT License - See [LICENSE](LICENSE) for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes with tests
4. Submit a pull request

## Support

For issues and feature requests, please open a GitHub issue.