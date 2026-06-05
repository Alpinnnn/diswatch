# Diswatch

Personal Discord RPC Automator for Custom Presence and Jellyfin watch activity, built as a lightweight Go single binary for CasaOS on ARM64 devices.

## Important Discord Notice

Discord states that automating normal user accounts outside the OAuth2/bot API is forbidden and may lead to account termination. This app keeps the implementation local, rate-limited, and focused only on presence updates, but it cannot remove that platform risk.

## Features

- Custom RPC mode with details, state, assets, elapsed timer, and up to two buttons.
- Jellyfin mode using a webhook endpoint plus `/Sessions` polling fallback.
- First-run admin password setup.
- Encrypted local config at `/app/data/config.enc.json`.
- Discord token and Jellyfin API key are never returned to the browser after saving.
- Docker-ready for `linux/arm64`.

## Local Development

```powershell
go mod tidy
go test ./...
go run ./cmd/diswatch
```

Open `http://localhost:8080`.

## CasaOS / Docker Compose

```powershell
docker compose up -d --build
```

The dashboard is exposed on port `8080`, and persistent data is stored in `./data`.

For best security, set `DISWATCH_SECRET_KEY` to a stable base64-encoded 32-byte key. Without it, Diswatch generates `/app/data/master.key`; keep that file with your backup or the encrypted config cannot be decrypted after data loss.

## Jellyfin Webhook

Configure Jellyfin Webhook Plugin to send JSON to:

```text
http://<casaos-host>:8080/api/jellyfin/webhook?secret=<secret-from-dashboard>
```

Enable playback-related notification types such as playback start, progress, pause, unpause, and stop.

Use a JSON template with these fields:

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

## Build ARM64 Image

```powershell
docker buildx build --platform linux/arm64 -t diswatch:local .
```
