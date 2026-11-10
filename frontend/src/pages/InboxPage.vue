<template>
  <PublicLayout>
    <div class="stack">
      <PublicWarningBanner />

      <div class="inbox-head">
        <div>
          <h1 class="inbox-address">{{ inboxStore.currentEmail }}</h1>
          <p class="meta">
            Aktif sampai {{ formatDateTime(inboxStore.expiresAt) }}
            <template v-if="inboxStore.lastUpdatedAt"> · diperbarui {{ formatDateTime(inboxStore.lastUpdatedAt) }}</template>
          </p>
        </div>
        <div class="row">
          <button type="button" class="btn" @click="copyAddress">
            {{ addressCopied ? "Alamat tersalin" : "Salin alamat" }}
          </button>
          <button type="button" class="btn" :disabled="inboxStore.loading" @click="manualRefresh">
            {{ inboxStore.loading ? "Memuat..." : "Refresh" }}
          </button>
          <button type="button" class="btn" @click="switchAddress">Ganti alamat</button>
        </div>
      </div>

      <div v-if="inboxStore.error" class="error" role="alert">{{ inboxStore.error }}</div>

      <article v-if="inboxStore.selectedMessage" class="panel stack">
        <div class="row">
          <button type="button" class="btn sm" @click="inboxStore.selectedMessage = null">&larr; Kembali ke daftar</button>
          <span class="spacer" />
          <button type="button" class="btn sm" @click="copyBody">
            {{ bodyCopied ? "Teks tersalin" : "Salin seluruh teks" }}
          </button>
        </div>

        <div class="message-card" style="cursor: default; padding: 0; border: none;">
          <div>
            <h2 class="message-title">{{ inboxStore.selectedMessage.subject || "(Tanpa subjek)" }}</h2>
            <p class="meta">Dari {{ inboxStore.selectedMessage.sender }}</p>
            <p class="meta">Kepada {{ inboxStore.selectedMessage.recipient }}</p>
          </div>
          <div class="message-side">
            <span class="meta">{{ formatDateTime(inboxStore.selectedMessage.received_at) }}</span>
            <OtpBox v-if="detailOtp" :code="detailOtp" />
          </div>
        </div>

        <pre class="plain-body"><template v-for="(seg, idx) in bodySegments" :key="idx"><a
          v-if="seg.isLink"
          :href="seg.text"
          target="_blank"
          rel="noopener noreferrer nofollow"
        >{{ seg.text }}</a><template v-else>{{ seg.text }}</template></template></pre>
      </article>

      <template v-else>
        <div v-if="inboxStore.messages.length === 0 && !inboxStore.loading" class="empty">
          Belum ada pesan. Kirim email ke alamat ini lalu tekan Refresh. Inbox juga diperbarui otomatis setiap
          {{ inboxStore.pollIntervalSeconds }} detik.
        </div>

        <div v-else class="stack" style="gap: 12px;">
          <article
            v-for="msg in inboxStore.messages"
            :key="msg.id"
            class="message-card"
            @click="inboxStore.fetchMessageDetail(msg.id)"
          >
            <div>
              <h3>
                <a href="#" class="title-link" @click.prevent.stop="inboxStore.fetchMessageDetail(msg.id)">
                  {{ msg.subject || "(Tanpa subjek)" }}
                </a>
              </h3>
              <p class="meta">Dari {{ msg.sender }}</p>
              <p class="message-body">{{ msg.snippet || "(Pesan kosong)" }}</p>
            </div>
            <div class="message-side">
              <span class="meta">{{ formatDateTime(msg.received_at) }}</span>
              <OtpBox v-if="detectOtp(msg.snippet)" :code="detectOtp(msg.snippet) as string" />
            </div>
          </article>

          <div v-if="inboxStore.nextBeforeId > 0" class="row" style="justify-content: center;">
            <button type="button" class="btn" :disabled="inboxStore.loading" @click="inboxStore.loadOlderMessages">
              Muat pesan lebih lama
            </button>
          </div>
        </div>
      </template>
    </div>
  </PublicLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";
import OtpBox from "../components/OtpBox.vue";
import PublicWarningBanner from "../components/PublicWarningBanner.vue";
import PublicLayout from "../layouts/PublicLayout.vue";
import { useInboxStore } from "../stores/inbox";
import { detectOtp, formatDateTime } from "../utils/format";

const inboxStore = useInboxStore();
const router = useRouter();
const addressCopied = ref<boolean>(false);
const bodyCopied = ref<boolean>(false);
let pollTimer: number | null = null;

const detailOtp = computed<string | null>(() => detectOtp(inboxStore.selectedMessage?.body_text));

interface TextSegment {
  text: string;
  isLink: boolean;
}

// Only http(s) URLs become links; the body is never rendered as HTML.
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

async function copyToClipboard(text: string, flag: typeof addressCopied): Promise<void> {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    return;
  }
  flag.value = true;
  setTimeout(() => {
    flag.value = false;
  }, 1400);
}

function copyAddress(): void {
  void copyToClipboard(inboxStore.currentEmail, addressCopied);
}

function copyBody(): void {
  void copyToClipboard(inboxStore.selectedMessage?.body_text || "", bodyCopied);
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
