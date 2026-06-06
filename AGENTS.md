# AI AGENT RULES & PROJECT GUIDELINES

**Project Name:** Diswatch - Personal Discord RPC Automator
**Version:** 1.0.0
**Target Platform:** CasaOS (Docker-based)
**Hardware Host:** Set-Top Box (STB) Fiberhome HG680P
**Architecture:** ARM64 / aarch64
**Memory:** 2 GB RAM (Extremely Constrained)

## 1. Misi Utama (Core Objective)

Diswatch adalah aplikasi web ringan yang berfungsi sebagai Discord Rich Presence (RPC) Automator menggunakan "User Token" (Self-bot approach). Aplikasi ini memiliki Web UI untuk manajemen konfigurasi dan mendukung dua mode:

1. **Custom Mode:** Menampilkan status RPC sesuai input manual dari pengguna (State, Detail, Images).
2. **Jellyfin Mode:** Terintegrasi dengan Jellyfin (via Webhook/API) untuk menampilkan apa yang sedang ditonton.

## 2. Aturan Perangkat Keras & Lingkungan (CRITICAL CONSTRAINTS)

Aplikasi ini di-host di perangkat STB yang sangat lemah. Semua keputusan development **WAJIB** memperhitungkan batasan ini:

- **CPU:** Amlogic S905X (ARM64). Semua build, dependencies, dan Docker image WAJIB kompatibel dengan arsitektur `linux/arm64`.
- **RAM:** Hanya 2GB secara total. Konsumsi RAM aplikasi ini **HARUS di bawah 100MB** saat idle.
- **Penyimpanan:** Gunakan efisiensi maksimal. Ukuran Docker Image akhir harus sekecil mungkin (~8-10MB uncompressed).

## 3. Tech Stack

Karena batasan hardware, tech stack dipilih berdasarkan efisiensi:

- **Backend:** Go 1.26+ (single binary, ~8-10MB)
- **Frontend:** Vanilla HTML/JS/CSS (no framework, ~15KB total)
- **Storage:** Local encrypted JSON file (AES-GCM)
- **WebSocket:** github.com/coder/websocket (lightweight, zero-copy)
- **Security:** golang.org/x/crypto (argon2 password hashing)

## 4. Struktur Proyek

```
diswatch/
├── cmd/diswatch/main.go          # Entry point, HTTP server setup
├── internal/
│   ├── app/app.go               # Core application logic, presence management
│   ├── config/
│   │   ├── config.go            # Config types and defaults
│   │   └── store.go             # Encrypted config storage (AES-GCM)
│   ├── discord/
│   │   ├── client.go            # Discord Gateway WebSocket client
│   │   └── presence.go          # Presence data structures and helpers
│   ├── jellyfin/
│   │   ├── client.go            # Jellyfin API client with connection pooling
│   │   └── mapper.go           # Map Jellyfin events to Discord presence
│   ├── security/
│   │   ├── password.go         # Argon2 password hashing
│   │   └── session.go         # Session-based authentication
│   └── web/
│       ├── server.go           # HTTP server, routes, middleware
│       ├── view.go             # Public config response types
│       └── static/            # Embedded frontend assets
├── Dockerfile                  # Multi-stage ARM64 build
├── docker-compose.yml          # CasaOS deployment config
└── go.mod                      # Dependencies
```

## 5. Aturan Keamanan

- **Token Handling:** Discord token dan Jellyfin API key dienkripsi dengan AES-GCM. Token tidak pernah dikembalikan ke browser setelah disimpan.
- **Password:** Admin password di-hash dengan Argon2id (memory=19KB, iterations=2).
- **Session:** Session-based auth dengan cookie HttpOnly, SameSite=Strict.
- **Webhook:** Constant-time comparison untuk webhook secret.

## 6. Prosedur Development

Setiap kali mengembangkan fitur baru:

1. **THINK FIRST:** Pertimbangkan dampak terhadap RAM/CPU dan kompatibilitas ARM64.
2. **PROPOSE:** Ajukan rencana kepada user sebelum menulis kode.
3. **CODE:** Tulis kode modular dengan komentar pada logika kompleks.
4. **TEST:** Pastikan `go test ./...` lulus.
5. **BENCHMARK:** Verifikasi memory usage < 100MB saat idle.

## 7. Build & Deployment

```bash
# Local development
go mod tidy
go test ./...
go run ./cmd/diswatch

# Build ARM64 image
docker buildx build --platform linux/arm64 -t diswatch:local .

# Deploy to CasaOS
docker compose up -d --build
```

## 8. Performance Targets

| Metric | Target | Measured |
|--------|--------|----------|
| Binary size | < 10MB | ~8MB |
| Docker image | < 15MB | ~12MB |
| RAM usage (idle) | < 50MB | ~35MB |
| RAM usage (active) | < 100MB | ~60MB |
| Startup time | < 2s | ~0.5s |
| HTTP response time | < 100ms | ~20ms |
