# BaraMail — Admin-Managed OTP Inbox

BaraMail adalah aplikasi penerima email (*inbound-only*) berbasis domain milik sendiri yang memungkinkan administrator membuat dan mengelola banyak alamat email (mode custom maupun acak) beserta masa aktifnya. Pengguna publik dapat membuka inbox dan membaca email OTP hanya dengan memasukkan alamat email yang masih aktif.

## Fitur Utama (MVP v1.1)

- **Admin-Only Provisioning:** Pembuatan mailbox tunggal maupun massal (*bulk create*: daftar custom, prefix berurutan, atau acak CSPRNG) oleh administrator.
- **Multi-Domain Management:** Tambah domain dan *retire*/*activate* domain dengan satu klik tanpa perlu mengubah mailbox satu per satu.
- **Email-Only Public Inbox:** Akses inbox publik tanpa login pengguna, dilengkapi proteksi anti-enumerasi (`MAILBOX_NOT_AVAILABLE` seragam), token inbox berumur pendek yang dicek ulang ke database pada setiap request, dan polling inkremental (`after_id`).
- **Cloudflare Email Worker + Raw MIME Ingestion:** Kontrak webhook raw MIME (`message/rfc822`) dengan autentikasi shared secret (mendukung rotasi dua secret), deduplikasi `SHA-256(raw MIME || 0x0A || recipient)`, dan parser MIME dengan fallback konversi HTML-only ke plaintext yang mempertahankan URL verifikasi.
- **Retensi Otomatis:** Pemangkasan kuota pesan per mailbox secara transaksional, pembersihan TTL pesan terjadwal, serta pembersihan pesan mailbox kedaluwarsa.
- **Keamanan & Observabilitas:** Session admin HttpOnly + proteksi CSRF & Origin, lockout brute-force login, ekstraksi IP klien via `TRUSTED_PROXIES` (`CF-Connecting-IP`), tampilan email plaintext-only, audit log admin, dan metrik webhook.

## Struktur Proyek

```text
├── backend/                  # Go + chi (net/http) API, CLI bootstrap, MIME parser, MySQL migrations
├── frontend/                 # Vue 3 + Vite + TypeScript + Pinia SPA (Public Inbox & Admin Dashboard)
├── cloudflare/email-worker/  # Cloudflare Email Worker (raw MIME forwarding, inline retry, Queues/R2 DLQ)
├── deploy/                   # Docker Compose & Caddyfile (satu origin untuk frontend dan /api)
└── docs/                     # Dokumentasi API, Arsitektur, Cloudflare Setup, Terms, dan Abuse Policy
```

## Cara Menjalankan secara Lokal (Development)

### 1. Database & Backend

1. Buat database MySQL 8.x (`utf8mb4_0900_ai_ci`) dan salin `backend/.env.example` ke `backend/.env`.
2. Jalankan migrasi dan buat akun admin pertama lewat CLI:
   ```bash
   cd backend
   go run ./cmd/cli migrate
   go run ./cmd/cli admin create --username admin --password "PasswordKuat123!"
   ```
3. Jalankan server API:
   ```bash
   go run ./cmd/api
   ```

### 2. Frontend

```bash
cd frontend
npm install
npm run dev
```

Buka `http://localhost:5173` untuk halaman publik atau `http://localhost:5173/admin/login` untuk dashboard admin.

## Menjalankan Unit Test

```bash
cd backend
go test -v ./tests/...
```

## Dokumentasi Lengkap

- [Dokumentasi API](docs/API.md)
- [Arsitektur Sistem](docs/ARCHITECTURE.md)
- [Setup Cloudflare & Keputusan Buffer Phase 0](docs/CLOUDFLARE_SETUP.md)
- [Panduan Deployment Produksi](deploy/README.md)
- [Syarat Layanan](docs/TERMS.md) & [Kebijakan Abuse](docs/ABUSE_POLICY.md)

