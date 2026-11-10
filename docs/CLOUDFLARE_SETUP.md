# Panduan Konfigurasi Cloudflare & Keputusan Buffer Durable (Phase 0)

Dokumen ini mencakup langkah konfigurasi Cloudflare Email Routing, Email Worker, Cloudflare Tunnel, serta keputusan arsitektur buffer durable dari Phase 0.

## 1. Keputusan Strategi Retry & Buffer Durable (Hasil Evaluasi Phase 0)

1. **Inline Retry di Email Worker:**
   Untuk gangguan jaringan atau restart singkat backend, Worker mencoba mengirim ulang hingga **3 kali secara inline** dengan jeda bertahap (600 ms, 1200 ms) hanya ketika respons backend adalah `429`, `5xx`, atau error jaringan.
2. **Buffer Durable dengan Cloudflare Queues + R2:**
   - Cloudflare Queues memiliki batas ukuran pesan sebesar **128 KB**, sedangkan email dengan header/HTML panjang dapat mencapai beberapa megabyte (hingga `MAX_EMAIL_SIZE_BYTES`, default 5 MB).
   - Karena itu, Worker menerapkan strategi hibrida:
     - Jika ukuran raw MIME ≤ 96 KB, payload dikirim langsung (base64) ke `BARAMAIL_RETRY_QUEUE`.
     - Jika ukuran raw MIME > 96 KB, raw MIME disimpan sementara di bucket R2 (`BARAMAIL_DLQ_BUCKET`) dengan prefix `retry/`, lalu hanya kunci objek R2 (`r2Key`) beserta metadata envelope yang dimasukkan ke `BARAMAIL_RETRY_QUEUE`.
     - Ketika consumer Queue berhasil mengirim email ke backend (`200`/`201`) atau mendapat penolakan permanen (`404`/`413`), objek sementara di R2 dihapus.
3. **Jalur Dead-Letter Queue (DLQ) & Fallback Tanpa Buffer:**
   - Jika backend mengembalikan `400`/`422` (`INVALID_MESSAGE`) atau `401`/`403` (`UNAUTHORIZED`), Worker tidak melakukan retry berulang, melainkan langsung menyimpan raw MIME ke prefix `dlq/` di R2 agar operator dapat meninjau dan memproses ulang.
   - Jika akun Cloudflare belum mengaktifkan Queues/R2 (mis. paket Free tanpa add-on), kegagalan setelah 3 percobaan inline akan melempar exception (`throw new Error(...)`) sehingga MTA Cloudflare mencatat kegagalan pengiriman dan tidak membuang email secara diam-diam.

## 2. Langkah Konfigurasi Domain Mailbox

Untuk **setiap** domain mailbox (mis. `domain-a.tld`):

1. Tambahkan domain ke akun Cloudflare dan arahkan nameserver ke Cloudflare DNS.
2. Buka menu **Email** &rarr; **Email Routing** dan aktifkan record MX serta TXT SPF yang disarankan Cloudflare.
3. Pada tab **Routing rules**:
   - Buat **Custom address** eksplisit untuk `abuse@domain-a.tld` dan `postmaster@domain-a.tld` dengan aksi **Forward to** alamat email operasional administrator yang nyata. Pastikan aturan ini berada di atas aturan catch-all.
   - Aktifkan aturan **Catch-all address** dengan aksi **Send to a Worker** &rarr; pilih `baramail-email-worker`.

## 3. Deploy Email Worker

```bash
cd cloudflare/email-worker
npm install
npx wrangler secret put BARAMAIL_WEBHOOK_SECRET
npx wrangler deploy
```

Untuk merotasi secret tanpa downtime:
1. Isi `WEBHOOK_SECRET_PREVIOUS` di `backend/.env` dengan nilai `WEBHOOK_SECRET` lama, dan isi `WEBHOOK_SECRET` dengan secret baru, lalu restart backend.
2. Perbarui `BARAMAIL_WEBHOOK_SECRET` di Worker menggunakan `wrangler secret put`.
3. Setelah seluruh traffic memakai secret baru, kosongkan kembali `WEBHOOK_SECRET_PREVIOUS` di backend.

