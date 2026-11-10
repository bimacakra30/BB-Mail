# Dokumentasi API BaraMail (v1)

Seluruh endpoint berada di bawah prefix `/api/v1` dan dilayani pada satu origin bersama frontend melalui Caddy.

## Format Respons Standar

**Sukses:**

```json
{
  "success": true,
  "data": {},
  "message": "OK"
}
```

**Error:**

```json
{
  "success": false,
  "error": {
    "code": "MAILBOX_NOT_AVAILABLE",
    "message": "Mailbox tidak tersedia."
  }
}
```

---

## 1. Admin Authentication

### `POST /api/v1/admin/login`
Login menggunakan username dan password. Menerbitkan cookie `baramail_admin_session` (`HttpOnly`, `Secure`, `SameSite=Lax`) dan mengembalikan token CSRF.

- **Rate limit:** `RATE_LIMIT_LOGIN_PER_MIN` per IP klien + lockout sementara setelah percobaan gagal berulang.
- **Request:**
  ```json
  { "username": "admin", "password": "PasswordKuat!" }
  ```
- **Response (`200`):**
  ```json
  {
    "success": true,
    "data": {
      "user": { "id": 1, "username": "admin" },
      "csrf_token": "64_HEX_CSRF_TOKEN",
      "expires_at": "2026-11-10T20:00:00Z"
    }
  }
  ```

### `POST /api/v1/admin/logout`
Menghapus sesi dari database dan membersihkan cookie. Wajib menyertakan header `X-CSRF-Token`.

### `GET /api/v1/admin/me`
Mengambil informasi sesi admin yang sedang aktif, token CSRF, dan parameter konfigurasi default.

---

## 2. Admin: Domain, Mailbox, Statistik, Audit Log

Seluruh endpoint yang mengubah data (`POST`, `PATCH`, `DELETE`) wajib menyertakan cookie sesi dan header `X-CSRF-Token`.

### Domain
- `GET /api/v1/admin/domains` — Daftar domain beserta jumlah mailbox.
- `POST /api/v1/admin/domains` — Tambah domain baru (`{ "name": "domain-a.tld" }`).
- `PATCH /api/v1/admin/domains/:id` — Ubah status domain (`{ "status": "active" | "retired" }`).

### Mailbox
- `GET /api/v1/admin/mailboxes` — Daftar mailbox (`search`, `status`, `domain_id`, `owner_ref`, `page`, `limit`).
- `POST /api/v1/admin/mailboxes` — Buat mailbox tunggal:
  - Mode custom:
    ```json
    {
      "domain_id": 1,
      "mode": "custom",
      "local_part": "budi",
      "expires_in_days": 7,
      "owner_ref": "cust-0012",
      "note": "paket 1 minggu"
    }
    ```
  - Mode acak:
    ```json
    {
      "domain_id": 1,
      "mode": "random",
      "length": 12,
      "expires_in_days": 7
    }
    ```
- `POST /api/v1/admin/mailboxes/bulk` — Pembuatan massal (`mode`: `"random"` | `"sequence"` | `"list"`, maksimum `MAX_BULK_CREATE`).
- `POST /api/v1/admin/mailboxes/bulk-action` — Aksi massal (`action`: `"disable"` | `"enable"` | `"extend"` | `"delete"` berdasarkan `ids` atau `filter.owner_ref` / `filter.domain_id`).
- `GET /api/v1/admin/mailboxes/export` — Unduh daftar mailbox dalam format CSV (tidak menyertakan isi email).
- `GET /api/v1/admin/mailboxes/:id` — Detail satu mailbox.
- `PATCH /api/v1/admin/mailboxes/:id` — Perbarui status (`active`/`disabled`), masa aktif (`extend_days` / `expires_in_days` / `expires_at`), `owner_ref`, atau `note`.
- `DELETE /api/v1/admin/mailboxes/:id` — Hapus mailbox beserta seluruh pesannya (`ON DELETE CASCADE`).
- `GET /api/v1/admin/mailboxes/:id/messages` — Daftar pesan pada mailbox (tambahkan `?message_id=<id>` untuk membaca detail satu pesan).

### Statistik & Audit Log
- `GET /api/v1/admin/stats` — Ringkasan statistik mailbox, pesan, dan metrik webhook.
- `GET /api/v1/admin/audit-logs` — Daftar audit log (`page`, `limit`, `action`).

---

## 3. Public Inbox

### `POST /api/v1/inbox/access`
Memvalidasi ketersediaan alamat email dan menerbitkan token inbox berumur pendek.
- **Request:** `{ "email": "budi@domain.tld" }`
- **Response Sukses (`200`):**
  ```json
  {
    "success": true,
    "data": {
      "email": "budi@domain.tld",
      "expires_at": "2026-11-17T10:00:00Z",
      "access_token": "HMAC_SIGNED_TOKEN",
      "token_expires_at": "2026-11-10T10:30:00Z",
      "poll_interval_seconds": 5
    }
  }
  ```
- **Response Gagal (`404` — Seragam untuk tidak ditemukan, disabled, expired, atau domain retired):**
  ```json
  {
    "success": false,
    "error": {
      "code": "MAILBOX_NOT_AVAILABLE",
      "message": "Mailbox tidak tersedia."
    }
  }
  ```

### `GET /api/v1/inbox/messages`
Mengambil daftar pesan pada mailbox yang terikat pada token `Authorization: Bearer <token>`.
- Query: `limit` (default 20), `before_id` (kursor halaman lama), `after_id` (polling inkremental).

### `GET /api/v1/inbox/messages/:id`
Mengambil detail plaintext satu pesan milik mailbox pada token.

---

## 4. Email Ingestion Webhook

### `POST /api/v1/webhooks/email`
Menerima raw MIME dari Cloudflare Email Worker.
- **Headers:**
  - `Content-Type: message/rfc822`
  - `X-BaraMail-Secret: <shared-secret>`
  - `X-BaraMail-Envelope-To: <envelope-recipient>`
  - `X-BaraMail-Envelope-From: <envelope-sender>`
  - `X-BaraMail-Received-At: <ISO-8601 UTC>`
- **Kode Status:**
  - `201`: Tersimpan (`status: "stored"`)
  - `200`: Duplikat idempotent (`status: "duplicate"`)
  - `404`: Mailbox tidak tersedia (`MAILBOX_NOT_AVAILABLE` — penolakan permanen, tanpa retry)
  - `413`: Ukuran melebihi batas (`PAYLOAD_TOO_LARGE` — tanpa retry)
  - `422`: MIME rusak dan tidak dapat diproses (`INVALID_MESSAGE` — kirim ke DLQ)
  - `401`: Secret salah (`UNAUTHORIZED` — kirim ke DLQ + alert)
  - `500`: Kesalahan sementara database/server (`INTERNAL_ERROR` — Worker melakukan retry)

