<template>
  <AdminLayout>
    <div class="page-head">
      <h1>Mailbox</h1>
      <div class="row">
        <button type="button" class="btn primary" @click="showCreateModal = true">+ Buat mailbox</button>
        <button type="button" class="btn" @click="showBulkModal = true">Buat massal</button>
        <a :href="exportCSVHref" class="btn">Ekspor CSV</a>
      </div>
    </div>

    <div v-if="stats" class="stats">
      <div class="stat is-brand">
        <div class="stat-value">{{ stats.total_mailboxes }}</div>
        <div class="stat-label">Total mailbox</div>
      </div>
      <div class="stat is-ok">
        <div class="stat-value">{{ stats.active_mailboxes }}</div>
        <div class="stat-label">Aktif</div>
      </div>
      <div class="stat is-muted">
        <div class="stat-value">{{ stats.disabled_mailboxes }}</div>
        <div class="stat-label">Nonaktif</div>
      </div>
      <div class="stat is-danger">
        <div class="stat-value">{{ stats.expired_mailboxes }}</div>
        <div class="stat-label">Kedaluwarsa</div>
      </div>
      <div class="stat">
        <div class="stat-value">{{ stats.messages_today }}</div>
        <div class="stat-label">Email hari ini</div>
      </div>
      <div class="stat">
        <div class="stat-value">{{ stats.webhook_metrics?.rejected_mailbox_not_available || 0 }}</div>
        <div class="stat-label">Webhook ditolak (404)</div>
      </div>
    </div>

    <form class="filters" @submit.prevent="applyFilter">
      <div class="field">
        <label for="f-search">Cari alamat</label>
        <input id="f-search" v-model="filterSearch" class="input" placeholder="budi@..." />
      </div>
      <div class="field">
        <label for="f-status">Status</label>
        <select id="f-status" v-model="filterStatus" class="select">
          <option value="">Semua status</option>
          <option value="active">Aktif</option>
          <option value="disabled">Nonaktif</option>
          <option value="expired">Kedaluwarsa</option>
        </select>
      </div>
      <div class="field">
        <label for="f-domain">Domain</label>
        <select id="f-domain" v-model.number="filterDomainId" class="select">
          <option :value="0">Semua domain</option>
          <option v-for="d in domains" :key="d.id" :value="d.id">{{ d.name }}</option>
        </select>
      </div>
      <div class="field">
        <label for="f-owner">Owner ref</label>
        <input id="f-owner" v-model="filterOwnerRef" class="input" placeholder="cust-0012" />
      </div>
      <div class="row">
        <button type="submit" class="btn primary">Terapkan</button>
        <button type="button" class="btn" @click="resetFilter">Reset</button>
      </div>
    </form>

    <div v-if="selectedIds.length > 0 || filterOwnerRef || filterDomainId > 0" class="bulk-bar">
      <span>
        Aksi massal untuk
        <strong>{{ selectedIds.length > 0 ? `${selectedIds.length} mailbox terpilih` : "semua hasil filter" }}</strong>
      </span>
      <div class="row">
        <button type="button" class="btn sm" @click="runBulkAction('disable')">Nonaktifkan</button>
        <button type="button" class="btn sm" @click="runBulkAction('enable')">Aktifkan</button>
        <button type="button" class="btn sm" @click="runBulkAction('extend', 7)">Perpanjang 7 hari</button>
        <button type="button" class="btn sm danger" @click="runBulkAction('delete')">Hapus</button>
      </div>
    </div>

    <div v-if="errorMsg" class="error" role="alert" style="margin-bottom: 16px;">{{ errorMsg }}</div>

    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th style="width: 40px;">
              <input type="checkbox" aria-label="Pilih semua mailbox di halaman ini" :checked="allSelected" @change="toggleSelectAll" />
            </th>
            <th>Alamat email</th>
            <th>Status</th>
            <th>Kedaluwarsa</th>
            <th>Owner ref</th>
            <th>Pesan</th>
            <th class="actions">Aksi</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="mailboxes.length === 0">
            <td colspan="7" class="muted" style="text-align: center; padding: 32px;">
              Belum ada mailbox yang cocok. Ubah filter atau buat mailbox baru.
            </td>
          </tr>
          <tr v-for="m in mailboxes" :key="m.id">
            <td>
              <input v-model="selectedIds" type="checkbox" :value="m.id" :aria-label="`Pilih ${m.email}`" />
            </td>
            <td>
              <div class="cell-main">{{ m.email }}</div>
              <div v-if="m.note" class="meta">{{ m.note }}</div>
            </td>
            <td>
              <span class="badge" :class="statusOf(m).cls">{{ statusOf(m).label }}</span>
            </td>
            <td class="meta">{{ formatDateTime(m.expires_at) }}</td>
            <td class="meta">{{ m.owner_ref || "-" }}</td>
            <td>{{ m.message_count }}</td>
            <td class="actions">
              <div class="row">
                <button type="button" class="btn sm" @click="openMessagesModal(m)">Pesan</button>
                <button type="button" class="btn sm" @click="openEditModal(m)">Edit</button>
                <button v-if="m.status === 'active'" type="button" class="btn sm" @click="quickToggleStatus(m, 'disabled')">
                  Nonaktifkan
                </button>
                <button v-else type="button" class="btn sm" @click="quickToggleStatus(m, 'active')">Aktifkan</button>
                <button type="button" class="btn sm danger" @click="confirmDelete(m)">Hapus</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="total > limit" class="pager">
      <span>{{ total }} mailbox · halaman {{ page }}</span>
      <div class="row">
        <button type="button" class="btn sm" :disabled="page <= 1" @click="changePage(page - 1)">Sebelumnya</button>
        <button type="button" class="btn sm" :disabled="page * limit >= total" @click="changePage(page + 1)">Berikutnya</button>
      </div>
    </div>

    <CreateMailboxModal v-if="showCreateModal" :domains="domains" @close="showCreateModal = false" @created="onMailboxCreated" />

    <BulkCreateModal v-if="showBulkModal" :domains="domains" @close="showBulkModal = false" @completed="onBulkCompleted" />

    <div v-if="editingMailbox" class="modal-backdrop" @click.self="editingMailbox = null">
      <form class="modal" role="dialog" aria-modal="true" aria-labelledby="edit-title" @submit.prevent="saveEditMailbox">
        <div class="modal-head">
          <h2 id="edit-title">Edit mailbox</h2>
          <button type="button" class="btn sm" @click="editingMailbox = null">Tutup</button>
        </div>
        <p class="cell-main">{{ editingMailbox.email }}</p>
        <div class="field">
          <label for="e-extend">Perpanjang masa aktif (hari)</label>
          <input id="e-extend" v-model.number="editExtendDays" type="number" min="0" max="365" class="input" />
          <span class="hint">Isi 0 jika masa aktif tidak diubah.</span>
        </div>
        <div class="field">
          <label for="e-owner">Owner ref</label>
          <input id="e-owner" v-model="editOwnerRef" class="input" />
        </div>
        <div class="field">
          <label for="e-note">Catatan</label>
          <input id="e-note" v-model="editNote" class="input" />
        </div>
        <div class="modal-actions">
          <button type="button" class="btn" @click="editingMailbox = null">Batal</button>
          <button type="submit" class="btn primary">Simpan perubahan</button>
        </div>
      </form>
    </div>

    <div v-if="inspectingMailbox" class="modal-backdrop" @click.self="inspectingMailbox = null">
      <div class="modal wide" role="dialog" aria-modal="true" aria-labelledby="msg-title">
        <div class="modal-head">
          <h2 id="msg-title">Pesan</h2>
          <button type="button" class="btn sm" @click="inspectingMailbox = null">Tutup</button>
        </div>
        <p class="cell-main">{{ inspectingMailbox.email }}</p>

        <div v-if="inspectedMessageDetail" class="stack">
          <div class="row">
            <button type="button" class="btn sm" @click="inspectedMessageDetail = null">&larr; Kembali ke daftar</button>
          </div>
          <div>
            <h3 class="message-title">{{ inspectedMessageDetail.subject || "(Tanpa subjek)" }}</h3>
            <p class="meta">Dari {{ inspectedMessageDetail.sender }} · {{ formatDateTime(inspectedMessageDetail.received_at) }}</p>
          </div>
          <pre class="plain-body">{{ inspectedMessageDetail.body_text }}</pre>
        </div>

        <div v-else>
          <div v-if="mailboxMessages.length === 0" class="empty">Mailbox ini belum menerima pesan.</div>
          <div v-else class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Subjek</th>
                  <th>Pengirim</th>
                  <th>Waktu</th>
                  <th class="actions">Aksi</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="msg in mailboxMessages" :key="msg.id">
                  <td class="cell-main">{{ msg.subject || "(Tanpa subjek)" }}</td>
                  <td class="meta">{{ msg.sender }}</td>
                  <td class="meta">{{ formatDateTime(msg.received_at) }}</td>
                  <td class="actions">
                    <button type="button" class="btn sm" @click="loadAdminMessageDetail(msg.id)">Baca</button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </AdminLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ApiError, apiRequest, getExportUrl } from "../api/client";
import BulkCreateModal from "../components/BulkCreateModal.vue";
import CreateMailboxModal from "../components/CreateMailboxModal.vue";
import AdminLayout from "../layouts/AdminLayout.vue";
import { useAdminStore } from "../stores/admin";
import { useToast } from "../stores/toast";
import type { Domain, Mailbox, MessageDetail, MessageSummary, StatsSummary } from "../types";
import { formatDateTime } from "../utils/format";

const adminStore = useAdminStore();
const toast = useToast();

const stats = ref<StatsSummary | null>(null);
const domains = ref<Domain[]>([]);
const mailboxes = ref<Mailbox[]>([]);
const total = ref<number>(0);
const page = ref<number>(1);
const limit = ref<number>(20);

const filterSearch = ref<string>("");
const filterStatus = ref<string>("");
const filterDomainId = ref<number>(0);
const filterOwnerRef = ref<string>("");
const selectedIds = ref<number[]>([]);
const errorMsg = ref<string>("");

const showCreateModal = ref<boolean>(false);
const showBulkModal = ref<boolean>(false);

const editingMailbox = ref<Mailbox | null>(null);
const editExtendDays = ref<number>(0);
const editOwnerRef = ref<string>("");
const editNote = ref<string>("");

const inspectingMailbox = ref<Mailbox | null>(null);
const mailboxMessages = ref<MessageSummary[]>([]);
const inspectedMessageDetail = ref<MessageDetail | null>(null);

const allSelected = computed(() => mailboxes.value.length > 0 && selectedIds.value.length === mailboxes.value.length);

const exportCSVHref = computed(() =>
  getExportUrl({
    search: filterSearch.value.trim() || undefined,
    status: filterStatus.value || undefined,
    domain_id: filterDomainId.value > 0 ? filterDomainId.value : undefined,
    owner_ref: filterOwnerRef.value.trim() || undefined,
  }),
);

function statusOf(m: Mailbox): { label: string; cls: string } {
  if (m.domain_status === "retired") return { label: "Domain dihentikan", cls: "bad" };
  if (m.effective_status === "expired") return { label: "Kedaluwarsa", cls: "bad" };
  if (m.effective_status === "disabled") return { label: "Nonaktif", cls: "off" };
  return { label: "Aktif", cls: "ok" };
}

function apiMessage(err: unknown, fallback: string): string {
  return err instanceof ApiError ? err.message : fallback;
}

async function loadAll(): Promise<void> {
  errorMsg.value = "";
  try {
    const [statsRes, domainsRes] = await Promise.all([
      apiRequest<StatsSummary>("/admin/stats"),
      apiRequest<Domain[]>("/admin/domains"),
    ]);
    stats.value = statsRes;
    domains.value = domainsRes || [];
    await loadMailboxes();
  } catch (err) {
    errorMsg.value = apiMessage(err, "Data dashboard gagal dimuat. Muat ulang halaman untuk mencoba lagi.");
  }
}

async function loadMailboxes(): Promise<void> {
  const q = new URLSearchParams();
  q.set("page", String(page.value));
  q.set("limit", String(limit.value));
  if (filterSearch.value.trim()) q.set("search", filterSearch.value.trim());
  if (filterStatus.value) q.set("status", filterStatus.value);
  if (filterDomainId.value > 0) q.set("domain_id", String(filterDomainId.value));
  if (filterOwnerRef.value.trim()) q.set("owner_ref", filterOwnerRef.value.trim());

  const res = await apiRequest<{ items: Mailbox[] | null; total: number }>(`/admin/mailboxes?${q.toString()}`);
  mailboxes.value = res.items || [];
  total.value = res.total || 0;
  selectedIds.value = [];
}

function applyFilter(): void {
  page.value = 1;
  void loadMailboxes();
}

function resetFilter(): void {
  filterSearch.value = "";
  filterStatus.value = "";
  filterDomainId.value = 0;
  filterOwnerRef.value = "";
  page.value = 1;
  void loadMailboxes();
}

function changePage(newPage: number): void {
  page.value = newPage;
  void loadMailboxes();
}

function toggleSelectAll(e: Event): void {
  const checked = (e.target as HTMLInputElement).checked;
  selectedIds.value = checked ? mailboxes.value.map((m) => m.id) : [];
}

function onMailboxCreated(): void {
  showCreateModal.value = false;
  toast.show("Mailbox dibuat");
  void loadAll();
}

function onBulkCompleted(): void {
  showBulkModal.value = false;
  void loadAll();
}

async function quickToggleStatus(m: Mailbox, status: "active" | "disabled"): Promise<void> {
  errorMsg.value = "";
  try {
    await apiRequest(`/admin/mailboxes/${m.id}`, {
      method: "PATCH",
      csrfToken: adminStore.csrfToken,
      body: JSON.stringify({ status }),
    });
    toast.show(status === "disabled" ? "Mailbox dinonaktifkan" : "Mailbox diaktifkan");
    await loadAll();
  } catch (err) {
    errorMsg.value = apiMessage(err, "Status mailbox gagal diubah. Coba lagi.");
  }
}

function openEditModal(m: Mailbox): void {
  editingMailbox.value = m;
  editExtendDays.value = 0;
  editOwnerRef.value = m.owner_ref || "";
  editNote.value = m.note || "";
}

async function saveEditMailbox(): Promise<void> {
  if (!editingMailbox.value) return;
  errorMsg.value = "";
  try {
    await apiRequest(`/admin/mailboxes/${editingMailbox.value.id}`, {
      method: "PATCH",
      csrfToken: adminStore.csrfToken,
      body: JSON.stringify({
        extend_days: editExtendDays.value > 0 ? editExtendDays.value : undefined,
        owner_ref: editOwnerRef.value,
        note: editNote.value,
      }),
    });
    editingMailbox.value = null;
    toast.show("Perubahan disimpan");
    await loadAll();
  } catch (err) {
    errorMsg.value = apiMessage(err, "Perubahan gagal disimpan. Coba lagi.");
  }
}

async function confirmDelete(m: Mailbox): Promise<void> {
  if (!window.confirm(`Hapus ${m.email}? Semua pesan di mailbox ini ikut terhapus dan tidak bisa dikembalikan.`)) return;
  errorMsg.value = "";
  try {
    await apiRequest(`/admin/mailboxes/${m.id}`, {
      method: "DELETE",
      csrfToken: adminStore.csrfToken,
    });
    toast.show("Mailbox dihapus");
    await loadAll();
  } catch (err) {
    errorMsg.value = apiMessage(err, "Mailbox gagal dihapus. Coba lagi.");
  }
}

const bulkToastText: Record<string, string> = {
  disable: "Mailbox dinonaktifkan",
  enable: "Mailbox diaktifkan",
  extend: "Masa aktif diperpanjang 7 hari",
  delete: "Mailbox dihapus",
};

async function runBulkAction(action: string, extendDays = 0): Promise<void> {
  const target = selectedIds.value.length > 0 ? `${selectedIds.value.length} mailbox terpilih` : "semua mailbox hasil filter";
  if (action === "delete" && !window.confirm(`Hapus ${target}? Semua pesannya ikut terhapus dan tidak bisa dikembalikan.`)) {
    return;
  }
  errorMsg.value = "";
  try {
    await apiRequest("/admin/mailboxes/bulk-action", {
      method: "POST",
      csrfToken: adminStore.csrfToken,
      body: JSON.stringify({
        action,
        ids: selectedIds.value.length > 0 ? selectedIds.value : undefined,
        extend_days: extendDays > 0 ? extendDays : undefined,
        filter: {
          owner_ref: selectedIds.value.length === 0 ? filterOwnerRef.value.trim() : undefined,
          domain_id: selectedIds.value.length === 0 && filterDomainId.value > 0 ? filterDomainId.value : undefined,
        },
      }),
    });
    toast.show(bulkToastText[action] || "Aksi massal selesai");
    await loadAll();
  } catch (err) {
    errorMsg.value = apiMessage(err, "Aksi massal gagal dijalankan. Coba lagi.");
  }
}

async function openMessagesModal(m: Mailbox): Promise<void> {
  inspectingMailbox.value = m;
  inspectedMessageDetail.value = null;
  mailboxMessages.value = [];
  try {
    const res = await apiRequest<{ items: MessageSummary[] | null }>(`/admin/mailboxes/${m.id}/messages`);
    mailboxMessages.value = res.items || [];
  } catch (err) {
    errorMsg.value = apiMessage(err, "Pesan mailbox gagal dimuat. Coba lagi.");
  }
}

async function loadAdminMessageDetail(messageId: number): Promise<void> {
  if (!inspectingMailbox.value) return;
  try {
    inspectedMessageDetail.value = await apiRequest<MessageDetail>(
      `/admin/mailboxes/${inspectingMailbox.value.id}/messages?message_id=${messageId}`,
    );
  } catch (err) {
    errorMsg.value = apiMessage(err, "Isi pesan gagal dimuat. Coba lagi.");
  }
}

onMounted(() => {
  void loadAll();
});
</script>
