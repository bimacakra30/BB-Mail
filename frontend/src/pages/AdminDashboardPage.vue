<template>
  <AdminLayout>
    <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 0.75rem; margin-bottom: 1.25rem;">
      <h1 style="margin: 0; font-size: 1.4rem;">Dashboard &amp; Mailboxes</h1>
      <div style="display: flex; gap: 0.5rem; flex-wrap: wrap;">
        <button type="button" class="btn" @click="showCreateModal = true">+ Create Mailbox</button>
        <button type="button" class="btn btn-outline" @click="showBulkModal = true">Bulk Create</button>
        <a :href="exportCSVHref" class="btn btn-outline" style="text-decoration: none;">Export CSV</a>
      </div>
    </div>

    <div
      v-if="stats"
      style="display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 0.75rem; margin-bottom: 1.25rem;"
    >
      <div class="card" style="padding: 0.9rem;">
        <div style="font-size: 0.78rem; color: var(--text-muted);">Total Mailbox</div>
        <div style="font-size: 1.35rem; font-weight: 700;">{{ stats.total_mailboxes }}</div>
      </div>
      <div class="card" style="padding: 0.9rem;">
        <div style="font-size: 0.78rem; color: var(--text-muted);">Aktif</div>
        <div style="font-size: 1.35rem; font-weight: 700; color: var(--success);">{{ stats.active_mailboxes }}</div>
      </div>
      <div class="card" style="padding: 0.9rem;">
        <div style="font-size: 0.78rem; color: var(--text-muted);">Disabled</div>
        <div style="font-size: 1.35rem; font-weight: 700;">{{ stats.disabled_mailboxes }}</div>
      </div>
      <div class="card" style="padding: 0.9rem;">
        <div style="font-size: 0.78rem; color: var(--text-muted);">Expired</div>
        <div style="font-size: 1.35rem; font-weight: 700; color: var(--danger);">{{ stats.expired_mailboxes }}</div>
      </div>
      <div class="card" style="padding: 0.9rem;">
        <div style="font-size: 0.78rem; color: var(--text-muted);">Total Email / Hari Ini</div>
        <div style="font-size: 1.35rem; font-weight: 700;">{{ stats.total_messages }} / {{ stats.messages_today }}</div>
      </div>
      <div class="card" style="padding: 0.9rem;">
        <div style="font-size: 0.78rem; color: var(--text-muted);">Webhook Ditolak (404)</div>
        <div style="font-size: 1.35rem; font-weight: 700;">{{ stats.webhook_metrics?.rejected_mailbox_not_available || 0 }}</div>
      </div>
    </div>

    <div class="card" style="margin-bottom: 1rem;">
      <form
        style="display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: 0.65rem; align-items: end;"
        @submit.prevent="applyFilter"
      >
        <div>
          <label style="display: block; font-size: 0.78rem; font-weight: 600; margin-bottom: 0.25rem;">Cari Alamat</label>
          <input v-model="filterSearch" class="input" placeholder="budi@..." />
        </div>
        <div>
          <label style="display: block; font-size: 0.78rem; font-weight: 600; margin-bottom: 0.25rem;">Status</label>
          <select v-model="filterStatus" class="select">
            <option value="">Semua Status</option>
            <option value="active">Active</option>
            <option value="disabled">Disabled</option>
            <option value="expired">Expired</option>
          </select>
        </div>
        <div>
          <label style="display: block; font-size: 0.78rem; font-weight: 600; margin-bottom: 0.25rem;">Domain</label>
          <select v-model.number="filterDomainId" class="select">
            <option :value="0">Semua Domain</option>
            <option v-for="d in domains" :key="d.id" :value="d.id">{{ d.name }} ({{ d.status }})</option>
          </select>
        </div>
        <div>
          <label style="display: block; font-size: 0.78rem; font-weight: 600; margin-bottom: 0.25rem;">Owner Ref</label>
          <input v-model="filterOwnerRef" class="input" placeholder="cust-0012" />
        </div>
        <div style="display: flex; gap: 0.4rem;">
          <button type="submit" class="btn">Filter</button>
          <button type="button" class="btn btn-outline" @click="resetFilter">Reset</button>
        </div>
      </form>
    </div>

    <div
      v-if="selectedIds.length > 0 || filterOwnerRef || filterDomainId > 0"
      class="card"
      style="margin-bottom: 1rem; padding: 0.8rem 1rem; display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 0.5rem; background: #f8fafc;"
    >
      <span style="font-size: 0.85rem; font-weight: 600;">
        Aksi Massal ({{ selectedIds.length > 0 ? `${selectedIds.length} dipilih` : 'Berdasarkan filter aktif' }}):
      </span>
      <div style="display: flex; gap: 0.4rem; flex-wrap: wrap;">
        <button type="button" class="btn btn-outline btn-sm" @click="runBulkAction('disable')">Disable</button>
        <button type="button" class="btn btn-outline btn-sm" @click="runBulkAction('enable')">Enable</button>
        <button type="button" class="btn btn-outline btn-sm" @click="runBulkAction('extend', 7)">+7 Hari</button>
        <button type="button" class="btn btn-danger btn-sm" @click="runBulkAction('delete')">Hapus</button>
      </div>
    </div>

    <div v-if="errorMsg" class="error-alert" style="margin-bottom: 1rem;">{{ errorMsg }}</div>

    <div class="card" style="padding: 0;">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th style="width: 36px;">
                <input type="checkbox" :checked="allSelected" @change="toggleSelectAll" />
              </th>
              <th>Alamat Email</th>
              <th>Status</th>
              <th>Kedaluwarsa</th>
              <th>Owner Ref</th>
              <th>Pesan</th>
              <th>Aksi</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="mailboxes.length === 0">
              <td colspan="7" style="text-align: center; padding: 2rem; color: var(--text-muted);">
                Tidak ada mailbox yang ditemukan.
              </td>
            </tr>
            <tr v-for="m in mailboxes" :key="m.id">
              <td>
                <input v-model="selectedIds" type="checkbox" :value="m.id" />
              </td>
              <td>
                <div style="font-weight: 600;">{{ m.email }}</div>
                <div v-if="m.note" style="font-size: 0.75rem; color: var(--text-muted);">{{ m.note }}</div>
              </td>
              <td>
                <span
                  class="badge"
                  :class="{
                    'badge-active': m.effective_status === 'active' && m.domain_status !== 'retired',
                    'badge-disabled': m.effective_status === 'disabled',
                    'badge-expired': m.effective_status === 'expired' || m.domain_status === 'retired',
                  }"
                >
                  {{ m.domain_status === 'retired' ? 'domain retired' : m.effective_status }}
                </span>
              </td>
              <td style="font-size: 0.82rem;">{{ formatDate(m.expires_at) }}</td>
              <td style="font-size: 0.82rem;">{{ m.owner_ref || '-' }}</td>
              <td>{{ m.message_count }}</td>
              <td>
                <div style="display: flex; gap: 0.35rem; flex-wrap: wrap;">
                  <button type="button" class="btn btn-outline btn-sm" @click="openMessagesModal(m)">Pesan</button>
                  <button type="button" class="btn btn-outline btn-sm" @click="openEditModal(m)">Edit</button>
                  <button
                    v-if="m.status === 'active'"
                    type="button"
                    class="btn btn-outline btn-sm"
                    @click="quickToggleStatus(m, 'disabled')"
                  >
                    Disable
                  </button>
                  <button
                    v-else
                    type="button"
                    class="btn btn-outline btn-sm"
                    @click="quickToggleStatus(m, 'active')"
                  >
                    Enable
                  </button>
                  <button type="button" class="btn btn-danger btn-sm" @click="confirmDelete(m)">Hapus</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div
        v-if="total > limit"
        style="padding: 0.85rem 1rem; display: flex; justify-content: space-between; align-items: center; font-size: 0.85rem;"
      >
        <span>Total {{ total }} mailbox (Halaman {{ page }})</span>
        <div style="display: flex; gap: 0.4rem;">
          <button type="button" class="btn btn-outline btn-sm" :disabled="page <= 1" @click="changePage(page - 1)">Prev</button>
          <button type="button" class="btn btn-outline btn-sm" :disabled="page * limit >= total" @click="changePage(page + 1)">Next</button>
        </div>
      </div>
    </div>

    <CreateMailboxModal
      v-if="showCreateModal"
      :domains="domains"
      @close="showCreateModal = false"
      @created="onMailboxCreated"
    />

    <BulkCreateModal
      v-if="showBulkModal"
      :domains="domains"
      @close="showBulkModal = false"
      @completed="onBulkCompleted"
    />

    <div v-if="editingMailbox" class="modal-backdrop" @click.self="editingMailbox = null">
      <div class="modal-panel" style="max-width: 460px;">
        <h3 style="margin-top: 0;">Edit Mailbox: {{ editingMailbox.email }}</h3>
        <form @submit.prevent="saveEditMailbox">
          <div style="margin-bottom: 0.85rem;">
            <label style="display: block; font-size: 0.85rem; font-weight: 600; margin-bottom: 0.3rem;">Perpanjang Masa Aktif (+ Hari)</label>
            <input v-model.number="editExtendDays" type="number" min="0" max="365" class="input" placeholder="0 = tidak diubah" />
          </div>
          <div style="margin-bottom: 0.85rem;">
            <label style="display: block; font-size: 0.85rem; font-weight: 600; margin-bottom: 0.3rem;">Owner Ref</label>
            <input v-model="editOwnerRef" class="input" />
          </div>
          <div style="margin-bottom: 1rem;">
            <label style="display: block; font-size: 0.85rem; font-weight: 600; margin-bottom: 0.3rem;">Catatan</label>
            <input v-model="editNote" class="input" />
          </div>
          <div style="display: flex; justify-content: flex-end; gap: 0.5rem;">
            <button type="button" class="btn btn-outline" @click="editingMailbox = null">Batal</button>
            <button type="submit" class="btn">Simpan</button>
          </div>
        </form>
      </div>
    </div>

    <div v-if="inspectingMailbox" class="modal-backdrop" @click.self="inspectingMailbox = null">
      <div class="modal-panel" style="max-width: 720px;">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
          <h3 style="margin: 0;">Pesan: {{ inspectingMailbox.email }}</h3>
          <button type="button" class="btn btn-outline btn-sm" @click="inspectingMailbox = null">Tutup</button>
        </div>

        <div v-if="inspectedMessageDetail">
          <button type="button" class="btn btn-outline btn-sm" style="margin-bottom: 0.75rem;" @click="inspectedMessageDetail = null">
            &larr; Kembali ke Daftar
          </button>
          <h4>{{ inspectedMessageDetail.subject || '(Tanpa Subjek)' }}</h4>
          <p style="font-size: 0.82rem; color: var(--text-muted);">
            Dari: {{ inspectedMessageDetail.sender }} &middot; {{ formatDate(inspectedMessageDetail.received_at) }}
          </p>
          <pre
            style="white-space: pre-wrap; word-break: break-word; background: #f8fafc; padding: 0.9rem; border: 1px solid var(--border); border-radius: 6px; font-size: 0.85rem;"
          >{{ inspectedMessageDetail.body_text }}</pre>
        </div>

        <div v-else>
          <div v-if="mailboxMessages.length === 0" style="padding: 1.5rem; text-align: center; color: var(--text-muted);">
            Tidak ada pesan di mailbox ini.
          </div>
          <div v-else class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Subjek</th>
                  <th>Pengirim</th>
                  <th>Waktu</th>
                  <th>Aksi</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="msg in mailboxMessages" :key="msg.id">
                  <td>{{ msg.subject || '(Tanpa Subjek)' }}</td>
                  <td>{{ msg.sender }}</td>
                  <td>{{ formatDate(msg.received_at) }}</td>
                  <td>
                    <button type="button" class="btn btn-outline btn-sm" @click="loadAdminMessageDetail(msg.id)">Baca</button>
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
import type { Domain, Mailbox, MessageDetail, MessageSummary, StatsSummary } from "../types";

const adminStore = useAdminStore();

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

const allSelected = computed(() => {
  return mailboxes.value.length > 0 && selectedIds.value.length === mailboxes.value.length;
});

const exportCSVHref = computed(() => {
  return getExportUrl({
    search: filterSearch.value.trim() || undefined,
    status: filterStatus.value || undefined,
    domain_id: filterDomainId.value > 0 ? filterDomainId.value : undefined,
    owner_ref: filterOwnerRef.value.trim() || undefined,
  });
});

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
    errorMsg.value = err instanceof ApiError ? err.message : "Gagal memuat data dashboard.";
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
    await loadAll();
  } catch (err) {
    errorMsg.value = err instanceof ApiError ? err.message : "Gagal mengubah status mailbox.";
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
    await loadAll();
  } catch (err) {
    errorMsg.value = err instanceof ApiError ? err.message : "Gagal memperbarui mailbox.";
  }
}

async function confirmDelete(m: Mailbox): Promise<void> {
  if (!window.confirm(`Hapus mailbox ${m.email} beserta seluruh pesannya?`)) return;
  errorMsg.value = "";
  try {
    await apiRequest(`/admin/mailboxes/${m.id}`, {
      method: "DELETE",
      csrfToken: adminStore.csrfToken,
    });
    await loadAll();
  } catch (err) {
    errorMsg.value = err instanceof ApiError ? err.message : "Gagal menghapus mailbox.";
  }
}

async function runBulkAction(action: string, extendDays = 0): Promise<void> {
  if (action === "delete" && !window.confirm("Yakin ingin menghapus mailbox yang dipilih/difilter beserta semua pesannya?")) {
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
    await loadAll();
  } catch (err) {
    errorMsg.value = err instanceof ApiError ? err.message : "Gagal menjalankan aksi massal.";
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
    errorMsg.value = err instanceof ApiError ? err.message : "Gagal memuat pesan mailbox.";
  }
}

async function loadAdminMessageDetail(messageId: number): Promise<void> {
  if (!inspectingMailbox.value) return;
  try {
    inspectedMessageDetail.value = await apiRequest<MessageDetail>(
      `/admin/mailboxes/${inspectingMailbox.value.id}/messages?message_id=${messageId}`,
    );
  } catch (err) {
    errorMsg.value = err instanceof ApiError ? err.message : "Gagal memuat isi pesan.";
  }
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  return isNaN(d.getTime()) ? iso : d.toLocaleString("id-ID");
}

onMounted(() => {
  void loadAll();
});
</script>

