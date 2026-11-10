<template>
  <PublicLayout>
    <div class="card" style="max-width: 780px; margin: 0 auto;">
      <template v-if="pageType === 'terms'">
        <h1 style="margin-top: 0;">Syarat Layanan (Terms of Service)</h1>
        <p><strong>Pembaruan Terakhir:</strong> 10 November 2026</p>

        <h3>1. Sifat Layanan</h3>
        <p>
          BaraMail adalah layanan inbox penerima email satu arah (<em>inbound-only</em>) yang dikelola oleh administrator.
          BaraMail tidak menyediakan fitur pengiriman email, balasan, penerusan, maupun penyimpanan jangka panjang.
        </p>

        <h3>2. Akses Publik Tanpa Autentikasi</h3>
        <p>
          Setiap inbox dibuka hanya dengan memasukkan alamat email. <strong>Siapa pun yang mengetahui atau berhasil menebak alamat email dapat membaca seluruh pesan dan kode OTP di dalam inbox tersebut.</strong>
          Jangan gunakan alamat BaraMail untuk perbankan, dompet digital, layanan kesehatan, akun pemerintah, email utama, atau akun sensitif lainnya.
        </p>

        <h3>3. Retensi Data dan Penghapusan Otomatis</h3>
        <p>
          Pesan dihapus secara otomatis berdasarkan batas waktu simpan (default 72 jam), batas jumlah pesan per mailbox (default 100 pesan terbaru),
          atau ketika masa aktif mailbox/domain berakhir. Administrator berhak menonaktifkan atau menghapus mailbox kapan saja tanpa pemberitahuan.
        </p>

        <h3>4. Larangan Penggunaan</h3>
        <p>
          Pengguna dilarang menggunakan alamat BaraMail untuk aktivitas penipuan, phishing, pelanggaran hukum, pendaftaran massal ilegal,
          atau aktivitas yang melanggar ketentuan layanan pihak ketiga.
        </p>
      </template>

      <template v-else>
        <h1 style="margin-top: 0;">Kebijakan Penyalahgunaan (Abuse Policy)</h1>
        <p>
          BaraMail berkomitmen mencegah penyalahgunaan domain dan layanan email kami. Seluruh pembuatan alamat dikendalikan oleh administrator
          dan tidak terbuka untuk pendaftaran publik bebas.
        </p>

        <h3>Pelaporan Penyalahgunaan</h3>
        <p>
          Jika Anda menemukan alamat email pada domain yang dikelola BaraMail digunakan untuk aktivitas berbahaya, penipuan, atau pelanggaran ketentuan layanan Anda,
          silakan kirim laporan ke:
        </p>
        <p>
          <strong>Email Kontak Abuse:</strong> <code>{{ abuseEmail }}</code>
        </p>

        <h3>Tindakan Penanganan</h3>
        <ul>
          <li>Menonaktifkan mailbox terkait secara permanen dalam waktu singkat.</li>
          <li>Menonaktifkan seluruh mailbox milik referensi pelanggan (<code>owner_ref</code>) yang melanggar.</li>
          <li>Me-retire domain apabila diperlukan.</li>
        </ul>
      </template>
    </div>
  </PublicLayout>
</template>

<script setup lang="ts">
defineProps<{
  pageType: "terms" | "abuse";
}>();

const abuseEmail = import.meta.env.VITE_ABUSE_EMAIL || "abuse@example.com";
import PublicLayout from "../layouts/PublicLayout.vue";
</script>

