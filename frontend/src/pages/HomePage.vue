<template>
  <PublicLayout>
    <section class="hero">
      <div class="hero-copy">
        <h1>Ambil kode OTP tanpa membuka email utama.</h1>
        <p>
          Masukkan alamat BaraMail yang sudah dibuat admin. Pesan masuk tampil dalam hitungan detik, dan kode verifikasi bisa langsung disalin.
        </p>
        <div class="row">
          <span class="chip">Hanya menerima email</span>
          <span class="chip">Tanpa login</span>
        </div>
      </div>

      <div class="stack">
        <form class="panel stack" @submit.prevent="handleOpenInbox">
          <div class="field">
            <label for="email-input">Alamat email</label>
            <input
              id="email-input"
              v-model="emailInput"
              type="email"
              class="input"
              placeholder="nama@domain.tld"
              required
              autocomplete="off"
              autocapitalize="off"
              spellcheck="false"
              inputmode="email"
            />
          </div>

          <div v-if="errorMsg" class="error" role="alert">{{ errorMsg }}</div>

          <button type="submit" class="btn primary lg" :disabled="inboxStore.loading">
            {{ inboxStore.loading ? "Membuka inbox..." : "Buka inbox" }}
          </button>
        </form>

        <PublicWarningBanner />
      </div>
    </section>
  </PublicLayout>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { ApiError } from "../api/client";
import PublicWarningBanner from "../components/PublicWarningBanner.vue";
import PublicLayout from "../layouts/PublicLayout.vue";
import { useInboxStore } from "../stores/inbox";

const inboxStore = useInboxStore();
const router = useRouter();
const emailInput = ref<string>(inboxStore.currentEmail || "");
const errorMsg = ref<string>("");

async function handleOpenInbox(): Promise<void> {
  errorMsg.value = "";
  const trimmed = emailInput.value.trim().toLowerCase();
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmed)) {
    errorMsg.value = "Format alamat belum benar. Tulis lengkap seperti nama@domain.tld.";
    return;
  }
  try {
    await inboxStore.openInbox(trimmed);
    await router.push("/inbox");
  } catch (err) {
    if (err instanceof ApiError && err.code === "RATE_LIMITED") {
      errorMsg.value = "Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi.";
      return;
    }
    errorMsg.value = "Mailbox tidak tersedia. Periksa ejaan alamat atau tanyakan ke admin apakah alamat masih aktif.";
  }
}
</script>
