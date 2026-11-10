# Syarat Layanan (Terms of Service) — BaraMail

**Tanggal Berlaku:** 10 November 2026

## 1. Gambaran Layanan
BaraMail adalah layanan penerima email satu arah (*inbound-only*) yang dikelola oleh administrator untuk keperluan penerimaan kode verifikasi atau OTP pada layanan berisiko rendah. Layanan ini tidak menyediakan pengiriman email, balasan, penerusan, maupun akses protokol IMAP/POP3/SMTP.

## 2. Model Akses Publik (Tanpa Login)
Setiap inbox pada BaraMail dapat dibuka oleh **siapa pun yang mengetahui alamat emailnya** tanpa password.
- Jangan gunakan alamat email BaraMail untuk rekening bank, dompet digital, layanan keuangan, akun utama, layanan kesehatan, atau layanan kritikal lainnya.
- Alamat dengan nama custom (seperti `nama@domain.tld`) mudah ditebak oleh pihak lain. Gunakan alamat acak panjang apabila membutuhkan tingkat kerahasiaan alamat yang lebih baik.

## 3. Masa Aktif dan Retensi Data
- Setiap pesan disimpan sementara dan dihapus otomatis setelah melewati batas waktu retensi (`MESSAGE_RETENTION_HOURS`, default 72 jam) atau ketika jumlah pesan dalam satu mailbox melampaui batas maksimum (`MAX_MESSAGES_PER_MAILBOX`, default 100 pesan terbaru).
- Setiap mailbox memiliki waktu kedaluwarsa (`expires_at`). Mailbox yang telah kedaluwarsa atau dinonaktifkan tidak dapat diakses dan tidak menerima email baru.
- Administrator dapat menonaktifkan atau me-*retire* domain kapan saja tanpa pemberitahuan sebelumnya.

## 4. Larangan Penggunaan
Pengguna dilarang memanfaatkan layanan BaraMail untuk:
- Penipuan, pencurian identitas, phishing, atau tindak pidana lainnya.
- Pendaftaran akun massal otomatis yang melanggar syarat layanan pihak ketiga.
- Aktivitas apa pun yang melanggar hukum yang berlaku di Republik Indonesia.

