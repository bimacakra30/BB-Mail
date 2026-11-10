<template>
  <AdminLayout>
    <div class="page-head">
      <h1>Log audit</h1>
      <div class="field" style="min-width: 220px;">
        <label for="f-action" class="sr-only">Filter aksi</label>
        <select id="f-action" v-model="actionFilter" class="select" @change="applyFilter">
          <option value="">Semua aksi</option>
          <option v-for="(label, key) in actionLabels" :key="key" :value="key">{{ label }}</option>
        </select>
      </div>
    </div>

    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Waktu</th>
            <th>Admin</th>
            <th>Aksi</th>
            <th>Target</th>
            <th>IP</th>
            <th>Detail</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="logs.length === 0">
            <td colspan="6" class="muted" style="text-align: center; padding: 32px;">Belum ada catatan untuk filter ini.</td>
          </tr>
          <tr v-for="l in logs" :key="l.id">
            <td class="meta" style="white-space: nowrap;">{{ formatDateTime(l.created_at) }}</td>
            <td>{{ l.admin_username || "-" }}</td>
            <td>{{ actionLabels[l.action] || l.action }}</td>
            <td class="meta">{{ l.target_type ? `${l.target_type} #${l.target_id ?? ""}` : "-" }}</td>
            <td><code>{{ l.ip }}</code></td>
            <td class="meta"><code>{{ l.meta ? JSON.stringify(l.meta) : "-" }}</code></td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="total > limit" class="pager">
      <span>{{ total }} catatan · halaman {{ page }}</span>
      <div class="row">
        <button type="button" class="btn sm" :disabled="page <= 1" @click="changePage(page - 1)">Sebelumnya</button>
        <button type="button" class="btn sm" :disabled="page * limit >= total" @click="changePage(page + 1)">Berikutnya</button>
      </div>
    </div>
  </AdminLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { apiRequest } from "../api/client";
import AdminLayout from "../layouts/AdminLayout.vue";
import type { AuditLog } from "../types";
import { formatDateTime } from "../utils/format";

const actionLabels: Record<string, string> = {
  "admin.login.success": "Masuk berhasil",
  "admin.login.failed": "Masuk gagal",
  "admin.login.locked": "Akun dikunci",
  "admin.logout": "Keluar",
  "mailbox.create": "Buat mailbox",
  "mailbox.bulk_create": "Buat massal",
  "mailbox.update": "Ubah mailbox",
  "mailbox.delete": "Hapus mailbox",
  "mailbox.bulk_action": "Aksi massal",
  "mailbox.export": "Ekspor CSV",
  "domain.create": "Tambah domain",
  "domain.retire": "Hentikan domain",
  "domain.activate": "Aktifkan domain",
};

const logs = ref<AuditLog[]>([]);
const total = ref<number>(0);
const page = ref<number>(1);
const limit = ref<number>(30);
const actionFilter = ref<string>("");

async function loadLogs(): Promise<void> {
  const q = new URLSearchParams({
    page: String(page.value),
    limit: String(limit.value),
  });
  if (actionFilter.value) {
    q.set("action", actionFilter.value);
  }
  const res = await apiRequest<{ items: AuditLog[] | null; total: number }>(`/admin/audit-logs?${q.toString()}`);
  logs.value = res.items || [];
  total.value = res.total || 0;
}

function applyFilter(): void {
  page.value = 1;
  void loadLogs();
}

function changePage(nextPage: number): void {
  page.value = nextPage;
  void loadLogs();
}

onMounted(() => {
  void loadLogs();
});
</script>
