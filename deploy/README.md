# Panduan Deployment BaraMail (Full Otomatis dengan Docker Compose)

Seluruh komponen (MySQL 8.4 di dalam container, Backend Go + migrasi database + bootstrap admin, serta build Frontend Vue 3 + Caddy) dijalankan **otomatis** cukup dengan satu perintah `docker compose up -d --build`.

## 1. Siapkan `backend/.env`

Salin `backend/.env.example` menjadi `backend/.env` dan isi nilainya:

```bash
cp ~/BB-Mail/backend/.env.example ~/BB-Mail/backend/.env
```

Pastikan variabel berikut sudah diatur di `backend/.env`:
- `WEB_PORT=8085` (port web yang dibuka keluar oleh Caddy; semua port internal container lainnya bebas dari bentrok).
- `DB_NAME=baramail`, `DB_USER=baramail`, `DB_PASSWORD=<password-db-anda>` (akan dibuat dan disinkronkan otomatis ke container `baramail-mysql`).
- `ADMIN_INITIAL_USERNAME=admin` dan `ADMIN_INITIAL_PASSWORD=<password-admin-min-8-karakter>` (akun admin akan dibuat otomatis saat container backend menyala).
- `ADMIN_SESSION_SECRET`, `INBOX_TOKEN_SECRET`, `WEBHOOK_SECRET` (isi dengan string acak minimal 32 karakter).
- `ALLOWED_EMAIL_DOMAINS=domain-a.tld,domain-b.tld` (otomatis dimasukkan ke tabel `domains` saat startup).

## 2. Jalankan Build & Start Otomatis

```bash
cd ~/BB-Mail/deploy
ln -sf ../backend/.env .env
docker compose up -d --build
```

Apa yang terjadi secara otomatis saat perintah di atas dijalankan:
1. **`baramail-mysql`** menyala dan lewat *healthcheck*-nya otomatis membuat database `DB_NAME` serta user `DB_USER` + `DB_PASSWORD` sesuai isi `backend/.env` (meskipun volume MySQL sudah pernah dibuat sebelumnya).
2. **`baramail-backend`** meng-compile aplikasi Go, menunggu MySQL siap, menjalankan migrasi seluruh tabel (`migrations/001_initial_schema.sql`), mengisi daftar domain awal (`ALLOWED_EMAIL_DOMAINS`), dan membuat akun admin (`ADMIN_INITIAL_USERNAME` & `ADMIN_INITIAL_PASSWORD`).
3. **`baramail-caddy`** meng-compile frontend Vue 3 (`deploy/Dockerfile.caddy`) dan langsung menyajikan web + proxy `/api` pada port `WEB_PORT` (default `8085`).

## 3. Pindah Server (Backup & Restore 1 Perintah)

Gunakan script `migrate.sh` di root project untuk mem-backup seluruh isi database MySQL sekaligus file `.env` ke dalam 1 file `.tar.gz`, lalu me-restore-nya di VPS baru:

```bash
chmod +x ~/BB-Mail/migrate.sh

# Di server lama (menghasilkan baramail-backup-YYYYMMDD-HHMMSS.tar.gz):
~/BB-Mail/migrate.sh backup

# Di server baru (otomatis restore .env, nyalakan Docker, dan import database MySQL):
~/BB-Mail/migrate.sh restore baramail-backup-20261110-103500.tar.gz
```

## 4. Reset Password / Tambah Admin Manual via CLI (Opsional)

Jika di kemudian hari Anda ingin mereset password atau menambah akun admin lain:

```bash
docker exec -it baramail-backend /app/baramail-cli admin reset-password \
  --username admin \
  --password "PasswordBaruYangKuat!"
```
