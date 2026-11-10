# Kebijakan Penanganan Penyalahgunaan (Abuse Policy)

BaraMail menerapkan kebijakan *admin-only provisioning* (pengguna publik tidak dapat membuat alamat email sendiri) serta pencatatan referensi pemilik (`owner_ref`) dan audit log untuk mencegah penyalahgunaan.

## 1. Saluran Pelaporan Abuse
Laporan terkait penyalahgunaan alamat email atau domain yang dikelola BaraMail dapat dikirimkan melalui:
- **Email:** `abuse@<domain-mailbox>` atau alamat kontak abuse yang tertera pada halaman publik aplikasi.
- Sertakan alamat email lengkap yang dilaporkan, waktu kejadian (UTC), serta bukti atau deskripsi singkat pelanggaran.

## 2. Prosedur dan Target Waktu Respons
1. **Penerimaan Laporan:** Setiap email ke `abuse@` diteruskan langsung ke inbox operasional administrator melalui route eksplisit di Cloudflare Email Routing (tidak masuk ke catch-all publik).
2. **Tindakan Mitigasi (Target ≤ 24 Jam):**
   - Menonaktifkan mailbox yang dilaporkan (`status = disabled`) atau menghapusnya.
   - Jika pelanggaran dilakukan oleh pelanggan tertentu, menonaktifkan seluruh mailbox dengan `owner_ref` tersebut melalui fitur **Bulk Action** di dashboard admin.
   - Jika seluruh domain disalahgunakan atau terblokir, mengubah status domain menjadi `retired` secara instan.

