# Panduan Deployment BaraMail

Dokumen ini menjelaskan langkah deployment produksi menggunakan Docker Compose, Caddy (satu origin untuk frontend dan `/api`), dan Cloudflare Tunnel.

## 1. Persiapan File Konfigurasi

1. Salin `backend/.env.example` menjadi `backend/.env` dan sesuaikan seluruh nilai secret (`ADMIN_SESSION_SECRET`, `INBOX_TOKEN_SECRET`, `WEBHOOK_SECRET`, dan `DB_PASSWORD`) dengan string acak minimal 32 byte (256-bit).
2. Atur `WEB_PORT` di `backend/.env` (mis. `WEB_PORT=8085`) untuk menentukan port yang dibuka keluar oleh Caddy. Port internal di dalam container (`8080` untuk backend, `3306` untuk MySQL, `80` untuk Caddy) berjalan terisolasi di dalam jaringan internal Docker dan tidak bentrok dengan service lain di host.
3. Pastikan `APP_BASE_URL` diisi dengan domain aplikasi (mis. `https://app.example.com`) dan `ALLOWED_EMAIL_DOMAINS` berisi daftar domain mailbox awal (mis. `domain-a.tld,domain-b.tld`).

## 2. Build Frontend Statis

```bash
cd frontend
npm ci
npm run build
```

Hasil build akan berada di `frontend/dist` dan disajikan langsung oleh Caddy.

## 3. Menjalankan Stack

Gunakan `--env-file ../backend/.env` (atau buat symlink `ln -sf ../backend/.env .env` di folder `deploy/`) agar Docker Compose membaca `WEB_PORT`:

```bash
cd deploy
ln -sf ../backend/.env .env
docker compose up -d --build
```

## 4. Bootstrap Akun Admin Pertama

Akun admin pertama hanya dapat dibuat melalui CLI (tidak ada endpoint registrasi publik):

```bash
docker exec -it baramail-backend /app/baramail-cli admin create \
  --username admin \
  --password "PasswordKuatMinimal8Karakter!" \
  --email "admin@example.com"
```

Untuk mereset password admin:

```bash
docker exec -it baramail-backend /app/baramail-cli admin reset-password \
  --username admin \
  --password "PasswordBaruYangKuat!"
```

## 5. Cloudflare Tunnel

Arahkan public hostname aplikasi (mis. `app.example.com`) pada Cloudflare Tunnel ke `http://127.0.0.1:<WEB_PORT>` (mis. `http://127.0.0.1:8085`).
Pastikan `TRUSTED_PROXIES` di `backend/.env` mencakup subnet Docker/loopback tempat Caddy berada sehingga backend membaca `CF-Connecting-IP` untuk rate limiting.

## 6. Backup MySQL & Retensi

Jalankan backup harian dengan `mysqldump --single-transaction` dan batasi masa simpan backup maksimal **14 hari** agar selaras dengan kebijakan retensi pesan:

```bash
docker exec baramail-mysql mysqldump -u root -p"$DB_PASSWORD" --single-transaction baramail \
  | gzip > /var/backups/baramail/baramail-$(date -u +%Y%m%d).sql.gz

find /var/backups/baramail -name "baramail-*.sql.gz" -mtime +14 -delete
```
