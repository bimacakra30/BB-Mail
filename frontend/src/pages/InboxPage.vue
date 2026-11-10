<template>
  <PublicLayout>
    <PublicWarningBanner style="margin-bottom: 1rem;" />

    <div class="card" style="margin-bottom: 1rem;">
      <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 0.75rem;">
        <div>
          <div style="display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap;">
            <strong style="font-size: 1.1rem;">{{ inboxStore.currentEmail }}</strong>
            <button type="button" class="btn btn-outline btn-sm" @click="copyText(inboxStore.currentEmail, 'Alamat')">
              {{ copiedLabel === 'Alamat' ? 'Tersalin!' : 'Salin Alamat' }}
            </button>
          </div>
          <div style="font-size: 0.8rem; color: var(--text-muted); margin-top: 0.25rem;">
            Aktif hingga: {{ formatDateTime(inboxStore.expiresAt) }}
            <span v-if="inboxStore.lastUpdatedAt">
              &middot; Diperbarui: {{ formatTimeOnly(inboxStore.lastUpdatedAt) }}
            </span>
          </div>
        </div>

        <div style="display: flex; gap: 0.5rem;">
          <button type="button" class="btn btn-outline btn-sm" :disabled="inboxStore.loading" @click="manualRefresh">
            {{ inboxStore.loading ? 'Memuat...' : 'Refresh' }}
          </button>
          <button type="button" class="btn btn-outline btn-sm" @click="switchAddress">
            Ganti Alamat
          </button>
        </div>
      </div>
    </div>

    <div v-if="inboxStore.error" class="error-alert" style="margin-bottom: 1rem;">
      {{ inboxStore.error }}
    </div>

    <div v-if="inboxStore.selectedMessage" class="card">
      <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 0.5rem; margin-bottom: 1rem;">
        <button type="button" class="btn btn-outline btn-sm" @click="inboxStore.selectedMessage = null">
          &larr; Kembali ke Daftar
        </button>
        <button type="button" class="btn btn-outline btn-sm" @click="copyText(inboxStore.selectedMessage.body_text, 'Isi Email')">
          {{ copiedLabel === 'Isi Email' ? 'Tersalin!' : 'Salin Seluruh Teks' }}
        </button>
      </div>

      <h2 style="margin: 0 0 0.5rem; font-size: 1.2rem;">
        {{ inboxStore.selectedMessage.subject || '(Tanpa Subjek)' }}
      </h2>

      <div style="font-size: 0.85rem; color: var(--text-muted); border-bottom: 1px solid var(--border); padding-bottom: 0.75rem; margin-bottom: 1rem;">
        <div><strong>Dari:</strong> {{ inboxStore.selectedMessage.sender }}</div>
        <div><strong>Kepada:</strong> {{ inboxStore.selectedMessage.recipient }}</div>
        <div>
          <strong>Waktu Diterima:</strong> {{ formatDateTime(inboxStore.selectedMessage.received_at) }}
          <span class="badge badge-disabled" style="margin-left: 0.4rem;">{{ inboxStore.selectedMessage.body_source }}</span>
        </div>
      </div>

      <div v-if="detectedOtpCodes.length > 0" style="margin-bottom: 1rem; padding: 0.75rem; background: #eff6ff; border: 1px solid #bfdbfe; border-radius: 6px;">
        <span style="font-size: 0.8rem; font-weight: 600; color: #1e40af; margin-right: 0.5rem;">
          Kode Angka Terdeteksi (Klik untuk menyalin):
        </span>
        <button
          v-for="code in detectedOtpCodes"
          :key="code"
          type="button"
          class="btn btn-sm"
          style="margin-right: 0.4rem; font-family: monospace; font-size: 0.95rem;"
          @click="copyText(code, code)"
        >
          {{ copiedLabel === code ? 'Tersalin!' : code }}
        </button>
      </div>

      <pre
        style="white-space: pre-wrap; word-break: break-word; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 0.9rem; background: #f8fafc; padding: 1rem; border-radius: 6px; border: 1px solid var(--border); margin: 0;"
      ><template v-for="(seg, idx) in bodySegments" :key="idx"><a
        v-if="seg.isLink"
        :href="seg.text"
        target="_blank"
        rel="noopener noreferrer nofollow"
      >{{ seg.text }}</a><template v-else>{{ seg.text }}</template></template></pre>
    </div>

    <div v-else class="card" style="padding: 0;">
      <div v-if="inboxStore.messages.length === 0 && !inboxStore.loading" style="padding: 2.5rem 1rem; text-align: center; color: var(--text-muted);">
        Belum ada email masuk untuk alamat ini. Halaman akan diperbarui otomatis setiap {{ inboxStore.pollIntervalSeconds }} detik.
      </div>

      <div v-else>
        <div
          v-for="msg in inboxStore.messages"
          :key="msg.id"
          style="padding: 0.9rem 1.25rem; border-bottom: 1px solid var(--border); cursor: pointer;"
          @click="inboxStore.fetchMessageDetail(msg.id)"
        >
          <div style="display: flex; justify-content: space-between; align-items: baseline; gap: 0.5rem;">
            <strong style="font-size: 0.95rem;">{{ msg.subject || '(Tanpa Subjek)' }}</strong>
            <span style="font-size: 0.78rem; color: var(--text-muted); white-space: nowrap;">
              {{ formatDateTime(msg.received_at) }}
            </span>
          </div>
          <div style="font-size: 0.82rem; color: var(--text-muted); margin-top: 0.15rem;">
            Dari: {{ msg.sender }}
          </div>
          <div style="font-size: 0.85rem; color: #334155; margin-top: 0.3rem;">
            {{ msg.snippet || '(Pesan kosong)' }}
          </div>
        </div>

        <div v-if="inboxStore.nextBeforeId > 0" style="padding: 0.85rem; text-align: center;">
          <button type="button" class="btn btn-outline btn-sm" :disabled="inboxStore.loading" @click="inboxStore.loadOlderMessages">
            Muat Pesan Lebih Lama
          </button>
        </div>
      </div>
    </div>
  </PublicLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";
import PublicWarningBanner from "../components/PublicWarningBanner.vue";
import PublicLayout from "../layouts/PublicLayout.vue";
import { useInboxStore } from "../stores/inbox";

const inboxStore = useInboxStore();
const router = useRouter();
const copiedLabel = ref<string>("");
let pollTimer: number | null = null;

// Extract 4-8 digit numeric candidates as convenience chips while always displaying full original plaintext.
const detectedOtpCodes = computed<string[]>(() => {
  const text = inboxStore.selectedMessage?.body_text || "";
  const matches = text.match(/\b\d{4,8}\b/g) || [];
  return Array.from(new Set(matches)).slice(0, 5);
});

interface TextSegment {
  text: string;
  isLink: boolean;
}

// Segment plaintext safely so only http:// and https:// URLs become links without rendering HTML.
const bodySegments = computed<TextSegment[]>(() => {
  const raw = inboxStore.selectedMessage?.body_text || "(Tidak ada isi teks)";
  const urlRegex = /(https?:\/\/[^\s<>()"']+)/gi;
  const result: TextSegment[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = urlRegex.exec(raw)) !== null) {
    if (match.index > lastIndex) {
      result.push({ text: raw.slice(lastIndex, match.index), isLink: false });
    }
    result.push({ text: match[0], isLink: true });
    lastIndex = urlRegex.lastIndex;
  }
  if (lastIndex < raw.length) {
    result.push({ text: raw.slice(lastIndex), isLink: false });
  }
  return result;
});

function scheduleNextPoll(): void {
  stopPolling();
  const baseIntervalMs = (inboxStore.pollIntervalSeconds || 5) * 1000;
  const intervalMs = document.hidden ? Math.max(30_000, baseIntervalMs) : baseIntervalMs;
  pollTimer = window.setTimeout(async () => {
    if (!inboxStore.currentEmail) return;
    await inboxStore.pollNewMessages();
    if (inboxStore.currentEmail) {
      scheduleNextPoll();
    }
  }, intervalMs);
}

function stopPolling(): void {
  if (pollTimer !== null) {
    clearTimeout(pollTimer);
    pollTimer = null;
  }
}

function handleVisibilityChange(): void {
  if (!inboxStore.currentEmail) return;
  if (!document.hidden) {
    void inboxStore.pollNewMessages();
  }
  scheduleNextPoll();
}

async function manualRefresh(): Promise<void> {
  await inboxStore.pollNewMessages();
  scheduleNextPoll();
}

async function switchAddress(): Promise<void> {
  stopPolling();
  inboxStore.clearSession();
  await router.push("/");
}

async function copyText(text: string, label: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(text);
    copiedLabel.value = label;
    setTimeout(() => {
      if (copiedLabel.value === label) copiedLabel.value = "";
    }, 1800);
  } catch {
    // Clipboard API may fail on insecure contexts.
  }
}

function formatDateTime(iso: string): string {
  if (!iso) return "-";
  const d = new Date(iso);
  return isNaN(d.getTime()) ? iso : d.toLocaleString("id-ID");
}

function formatTimeOnly(d: Date): string {
  return d.toLocaleTimeString("id-ID");
}

onMounted(async () => {
  if (!inboxStore.currentEmail) {
    await router.replace("/");
    return;
  }
  await inboxStore.fetchInitialMessages();
  document.addEventListener("visibilitychange", handleVisibilityChange);
  scheduleNextPoll();
});

onUnmounted(() => {
  stopPolling();
  document.removeEventListener("visibilitychange", handleVisibilityChange);
});
</script>

