<template>
  <PublicLayout>
    <article class="prose">
      <template v-if="pageType === 'terms'">
        <h1>Syarat layanan</h1>
        <p class="meta">Berlaku sejak 10/11/2026</p>

        <h2>Sifat layanan</h2>
        <p>
          BaraMail adalah inbox yang hanya menerima email dan dikelola oleh admin. Tidak ada fitur kirim, balas, teruskan,
          atau penyimpanan jangka panjang.
        </p>

        <h2>Akses tanpa login</h2>
        <p>
          Inbox dibuka hanya dengan memasukkan alamat email. Siapa pun yang tahu atau berhasil menebak alamat bisa membaca
          seluruh pesan dan kode OTP di dalamnya. Jangan pakai BaraMail untuk perbankan, dompet digital, layanan kesehatan,
          akun pemerintah, email utama, atau akun penting lain.
        </p>

        <h2>Penyimpanan dan penghapusan otomatis</h2>
        <p>
          Pesan dihapus otomatis setelah 72 jam, saat jumlah pesan di satu mailbox melebihi 100, atau saat masa aktif mailbox
          maupun domain berakhir. Admin dapat menonaktifkan atau menghapus mailbox kapan saja.
        </p>

        <h2>Larangan</h2>
        <p>
          Dilarang memakai alamat BaraMail untuk penipuan, phishing, pelanggaran hukum, pendaftaran massal yang melanggar
          aturan situs lain, atau kegiatan yang melanggar syarat layanan pihak ketiga.
        </p>
      </template>

      <template v-else>
        <h1>Kebijakan abuse</h1>
        <p>
          Semua alamat BaraMail dibuat oleh admin dan tidak terbuka untuk pendaftaran publik. Kami menindak penyalahgunaan
          domain dan alamat secepat mungkin.
        </p>

        <h2>Cara melapor</h2>
        <p>
          Kirim laporan ke <code>{{ abuseEmail }}</code>. Sertakan alamat yang dilaporkan, waktu kejadian, dan bukti singkat.
        </p>

        <h2>Tindakan yang kami ambil</h2>
        <ul>
          <li>Menonaktifkan mailbox yang dilaporkan.</li>
          <li>Menonaktifkan semua mailbox milik pelanggan yang melanggar.</li>
          <li>Menghentikan domain bila diperlukan.</li>
        </ul>
      </template>
    </article>
  </PublicLayout>
</template>

<script setup lang="ts">
import PublicLayout from "../layouts/PublicLayout.vue";

defineProps<{
  pageType: "terms" | "abuse";
}>();

const abuseEmail = import.meta.env.VITE_ABUSE_EMAIL || "abuse@example.com";
</script>
