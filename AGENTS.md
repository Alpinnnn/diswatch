# AI AGENT RULES & PROJECT GUIDELINES
**Project Name:** Personal Discord RPC Automator (Custom + Jellyfin Mode)
**Target Platform:** CasaOS (Docker-based)
**Hardware Host:** Set-Top Box (STB) Fiberhome HG680P
**Architecture:** ARM64 / aarch64
**Memory:** 2 GB RAM (Extremely Constrained)

## 1. Misi Utama (Core Objective)
Tugas Anda sebagai AI Agent adalah membantu mengembangkan aplikasi web ringan yang berfungsi sebagai Discord Rich Presence (RPC) Automator menggunakan "User Token" (Self-bot approach). Aplikasi ini harus memiliki Web UI untuk manajemen konfigurasi dan mendukung dua mode:
1. **Custom Mode:** Menampilkan status RPC sesuai input manual dari pengguna (State, Detail, Images).
2. **Jellyfin Mode:** Terintegrasi dengan Jellyfin (via Webhook/API) untuk menampilkan apa yang sedang ditonton.

## 2. Aturan Perangkat Keras & Lingkungan (CRITICAL CONSTRAINTS)
Anda **WAJIB** mengingat bahwa aplikasi ini akan di-host di perangkat STB yang sangat lemah.
- **CPU:** Amlogic S905X (ARM64). Semua build, dependencies, dan Docker image WAJIB kompatibel dengan arsitektur `linux/arm64`. JANGAN berikan solusi x86_64/amd64-only.
- **RAM:** Hanya 2GB secara total, dan sudah terbagi dengan OS + aplikasi lain. Konsumsi RAM aplikasi ini (Backend + Web UI saat idle) **HARUS di bawah 100MB**.
- **Penyimpanan:** Gunakan efisiensi maksimal. Ukuran Docker Image akhir harus sekecil mungkin.

## 3. Panduan Tech Stack
Karena batasan hardware di atas, pilih teknologi berdasarkan kriteria berikut:
- **Backend:** Gunakan bahasa/runtime yang sangat efisien. Direkomendasikan: **Go**, **Rust**, atau **Bun/Node.js** dengan framework minimalis (contoh: Hono, Elysia, atau Express murni). JANGAN gunakan framework berat seperti NestJS, Spring Boot, atau Django.
- **Frontend:** Gunakan Vanilla HTML/JS/CSS, Alpine.js, HTMX, atau Svelte (dikompilasi ke statik). JANGAN gunakan React/Next.js/Angular yang membutuhkan resource server besar untuk SSR.
- **Database/Storage:** Gunakan local file (JSON/YAML) atau SQLite. JANGAN gunakan database eksternal seperti PostgreSQL/MySQL/MongoDB.

## 4. Aturan Keamanan & API (Security Guidelines)
- **Token Handling:** Discord User Token bersifat sangat rahasia. Token harus disimpan dengan aman (misalnya di-enkripsi atau ditaruh di file environment/local database tertutup) dan HANYA diekspos/dimanipulasi melalui Web UI yang memiliki autentikasi (opsional namun disarankan: basic auth untuk akses Web UI).
- **Anti-Ban/TOS:** Interaksi dengan API Discord harus hati-hati. Gunakan interval update yang wajar (jangan spam API/Websocket). Hindari istilah "Self-bot spamming" dalam penamaan fungsi.

## 5. Standar Docker & CasaOS
Aplikasi ini akan diinstal via **CasaOS Custom Install**.
- **Dockerfile:** Gunakan multi-stage build. Gunakan base image yang sangat ringan (`alpine`, `scratch`, atau `distroless`). Set platform target eksplisit jika diperlukan (`--platform=linux/arm64`).
- **docker-compose.yml:** 
  - Buat struktur standar.
  - Sediakan konfigurasi `ports` (misal `8080:8080`) untuk Web UI.
  - Sediakan konfigurasi `volumes` (misal `./data:/app/data`) agar data config/token persisten ketika container di-restart/update.

## 6. Prosedur Eksekusi AI (Workflow)
Setiap kali Anda menerima prompt baru untuk mengembangkan fitur proyek ini, ikuti alur kerja berikut:
1. **THINK FIRST:** Pikirkan dampaknya terhadap penggunaan RAM/CPU dan kompatibilitas ARM64.
2. **PROPOSE:** Ajukan rencana (arsitektur, library yang dipilih, struktur data) kepada user *sebelum* menulis ratusan baris kode.
3. **CODE:** Tulis kode modular, rapi, dan berikan komentar pada logika yang rumit.
4. **REVIEW:** Pastikan tidak ada *memory leak* dan resource yang tidak tertutup (misal: websocket connection).

**Pesan Penting:** Jika Anda memahami aturan ini, mulailah setiap respon yang berkaitan dengan sistem dengan konfirmasi singkat bahwa Anda mengingat batasan "ARM64 & RAM 2GB".