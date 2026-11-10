<template>
  <PublicLayout>
    <div style="max-width: 560px; margin: 1.5rem auto;">
      <PublicWarningBanner style="margin-bottom: 1.25rem;" />

      <div class="card">
        <h1 style="margin: 0 0 0.4rem; font-size: 1.5rem;">Buka Inbox OTP</h1>
        <p style="margin: 0 0 1.25rem; color: var(--text-muted); font-size: 0.9rem;">
          Masukkan alamat email yang telah diaktifkan administrator untuk melihat pesan masuk dan kode verifikasi.
          Layanan ini <strong>hanya menerima email (inbound-only)</strong> dan tidak mendukung pengiriman atau balasan email.
        </p>

        <form @submit.prevent="handleOpenInbox">
          <div style="margin-bottom: 1rem;">
            <label for="email-input" style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">
              Alamat Email
            </label>
            <input
              id="email-input"
              v-model="emailInput"
              type="email"
              class="input"
              placeholder="nama@domain.tld"
              required
              autocomplete="off"
            />
          </div>

          <div v-if="errorMsg" class="error-alert" style="margin-bottom: 1rem;">
            {{ errorMsg }}
          </div>

          <button type="submit" class="btn" style="width: 100%; padding: 0.65rem;" :disabled="inboxStore.loading">
            {{ inboxStore.loading ? 'Memeriksa Mailbox...' : 'Open Inbox' }}
          </button>
        </form>
      </div>
    </div>
  </PublicLayout>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
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
    errorMsg.value = "Format alamat email tidak valid.";
    return;
  }
  try {
    await inboxStore.openInbox(trimmed);
    await router.push("/inbox");
  } catch {
    errorMsg.value = inboxStore.error || "Mailbox tidak tersedia.";
  }
}
</script>

