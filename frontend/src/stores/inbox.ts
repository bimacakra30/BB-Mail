import { defineStore } from "pinia";
import { ref } from "vue";
import { ApiError, apiRequest } from "../api/client";
import type { InboxAccessData, MessageDetail, MessageSummary } from "../types";

export const useInboxStore = defineStore("inbox", () => {
  // Address and short-lived token stay in memory only per FR-008.
  const currentEmail = ref<string>("");
  const expiresAt = ref<string>("");
  const accessToken = ref<string>("");
  const tokenExpiresAt = ref<string>("");
  const pollIntervalSeconds = ref<number>(5);

  const messages = ref<MessageSummary[]>([]);
  const nextBeforeId = ref<number>(0);
  const selectedMessage = ref<MessageDetail | null>(null);
  const lastUpdatedAt = ref<Date | null>(null);
  const loading = ref<boolean>(false);
  const error = ref<string>("");

  async function openInbox(email: string): Promise<void> {
    loading.value = true;
    error.value = "";
    try {
      const data = await apiRequest<InboxAccessData>("/inbox/access", {
        method: "POST",
        body: JSON.stringify({ email: email.trim().toLowerCase() }),
      });
      currentEmail.value = data.email;
      expiresAt.value = data.expires_at;
      accessToken.value = data.access_token;
      tokenExpiresAt.value = data.token_expires_at;
      pollIntervalSeconds.value = data.poll_interval_seconds || 5;
      messages.value = [];
      selectedMessage.value = null;
      nextBeforeId.value = 0;
    } catch (err) {
      clearSession();
      if (err instanceof ApiError) {
        error.value = err.message;
      } else {
        error.value = "Mailbox tidak tersedia.";
      }
      throw err;
    } finally {
      loading.value = false;
    }
  }

  async function ensureValidToken(): Promise<void> {
    if (!currentEmail.value) {
      throw new ApiError(401, "NO_EMAIL", "Silakan masukkan alamat email.");
    }
    const expMs = Date.parse(tokenExpiresAt.value);
    // Refresh token proactively 20 seconds before expiry.
    if (!accessToken.value || isNaN(expMs) || Date.now() + 20_000 >= expMs) {
      const data = await apiRequest<InboxAccessData>("/inbox/access", {
        method: "POST",
        body: JSON.stringify({ email: currentEmail.value }),
      });
      accessToken.value = data.access_token;
      tokenExpiresAt.value = data.token_expires_at;
      expiresAt.value = data.expires_at;
    }
  }

  async function fetchInitialMessages(): Promise<void> {
    loading.value = true;
    error.value = "";
    try {
      await ensureValidToken();
      const res = await apiRequest<{
        mailbox: { email: string; expires_at: string };
        items: MessageSummary[] | null;
        next_before_id: number;
      }>("/inbox/messages?limit=20", {
        bearerToken: accessToken.value,
      });
      expiresAt.value = res.mailbox.expires_at;
      messages.value = res.items || [];
      nextBeforeId.value = res.next_before_id || 0;
      lastUpdatedAt.value = new Date();
    } catch (err) {
      await handleTokenOrAvailabilityError(err);
    } finally {
      loading.value = false;
    }
  }

  async function pollNewMessages(): Promise<void> {
    if (!currentEmail.value) return;
    try {
      await ensureValidToken();
      const highestId = messages.value.length > 0 ? messages.value[0].id : 0;
      const qs = highestId > 0 ? `?after_id=${highestId}&limit=20` : `?limit=20`;
      const res = await apiRequest<{
        mailbox: { email: string; expires_at: string };
        items: MessageSummary[] | null;
        next_before_id: number;
      }>(`/inbox/messages${qs}`, {
        bearerToken: accessToken.value,
      });
      expiresAt.value = res.mailbox.expires_at;
      const incoming = res.items || [];
      if (highestId === 0) {
        messages.value = incoming;
        nextBeforeId.value = res.next_before_id || 0;
      } else if (incoming.length > 0) {
        const existingIds = new Set(messages.value.map((m) => m.id));
        const fresh = incoming.filter((m) => !existingIds.has(m.id));
        if (fresh.length > 0) {
          messages.value = [...fresh, ...messages.value];
        }
      }
      lastUpdatedAt.value = new Date();
    } catch (err) {
      await handleTokenOrAvailabilityError(err);
    }
  }

  async function loadOlderMessages(): Promise<void> {
    if (!nextBeforeId.value || loading.value) return;
    loading.value = true;
    try {
      await ensureValidToken();
      const res = await apiRequest<{
        items: MessageSummary[] | null;
        next_before_id: number;
      }>(`/inbox/messages?before_id=${nextBeforeId.value}&limit=20`, {
        bearerToken: accessToken.value,
      });
      const older = res.items || [];
      messages.value = [...messages.value, ...older];
      nextBeforeId.value = res.next_before_id || 0;
    } catch (err) {
      await handleTokenOrAvailabilityError(err);
    } finally {
      loading.value = false;
    }
  }

  async function fetchMessageDetail(id: number): Promise<void> {
    loading.value = true;
    error.value = "";
    try {
      await ensureValidToken();
      const detail = await apiRequest<MessageDetail>(`/inbox/messages/${id}`, {
        bearerToken: accessToken.value,
      });
      selectedMessage.value = detail;
    } catch (err) {
      await handleTokenOrAvailabilityError(err);
    } finally {
      loading.value = false;
    }
  }

  async function handleTokenOrAvailabilityError(err: unknown): Promise<void> {
    if (err instanceof ApiError) {
      if (err.status === 401 && currentEmail.value) {
        accessToken.value = "";
        try {
          await ensureValidToken();
          return;
        } catch (refreshErr) {
          clearSession();
          error.value = refreshErr instanceof ApiError ? refreshErr.message : "Mailbox tidak tersedia.";
          return;
        }
      }
      if (err.code === "MAILBOX_NOT_AVAILABLE") {
        clearSession();
      }
      error.value = err.message;
    } else {
      error.value = "Gagal memuat data inbox.";
    }
  }

  function clearSession(): void {
    currentEmail.value = "";
    expiresAt.value = "";
    accessToken.value = "";
    tokenExpiresAt.value = "";
    messages.value = [];
    selectedMessage.value = null;
    nextBeforeId.value = 0;
  }

  return {
    currentEmail,
    expiresAt,
    accessToken,
    pollIntervalSeconds,
    messages,
    nextBeforeId,
    selectedMessage,
    lastUpdatedAt,
    loading,
    error,
    openInbox,
    fetchInitialMessages,
    pollNewMessages,
    loadOlderMessages,
    fetchMessageDetail,
    clearSession,
  };
});

