<template>
  <AdminLayout>
    <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 0.75rem; margin-bottom: 1.25rem;">
      <h1 style="margin: 0; font-size: 1.4rem;">Audit Logs</h1>
      <div style="display: flex; gap: 0.5rem;">
        <select v-model="actionFilter" class="select" style="width: auto;" @change="applyFilter">
          <option value="">Semua Aksi</option>
          <option value="admin.login.success">admin.login.success</option>
          <option value="admin.login.failed">admin.login.failed</option>
          <option value="mailbox.create">mailbox.create</option>
          <option value="mailbox.bulk_create">mailbox.bulk_create</option>
          <option value="mailbox.update">mailbox.update</option>
          <option value="mailbox.delete">mailbox.delete</option>
          <option value="mailbox.bulk_action">mailbox.bulk_action</option>
          <option value="mailbox.export">mailbox.export</option>
          <option value="domain.create">domain.create</option>
          <option value="domain.retire">domain.retire</option>
          <option value="domain.activate">domain.activate</option>
        </select>
      </div>
    </div>

    <div class="card" style="padding: 0;">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Waktu</th>
              <th>Admin</th>
              <th>Aksi</th>
              <th>Target</th>
              <th>IP</th>
              <th>Detail (Meta)</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="logs.length === 0">
              <td colspan="6" style="text-align: center; padding: 2rem; color: var(--text-muted);">
                Belum ada catatan audit log.
              </td>
            </tr>
            <tr v-for="l in logs" :key="l.id">
              <td style="white-space: nowrap; font-size: 0.8rem;">{{ new Date(l.created_at).toLocaleString('id-ID') }}</td>
              <td>{{ l.admin_username || '-' }}</td>
              <td><code>{{ l.action }}</code></td>
              <td>{{ l.target_type ? `${l.target_type}#${l.target_id || ''}` : '-' }}</td>
              <td><code>{{ l.ip }}</code></td>
              <td style="font-size: 0.78rem; font-family: monospace;">{{ l.meta ? JSON.stringify(l.meta) : '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div
        v-if="total > limit"
        style="padding: 0.85rem 1rem; display: flex; justify-content: space-between; align-items: center; font-size: 0.85rem;"
      >
        <span>Total {{ total }} log (Halaman {{ page }})</span>
        <div style="display: flex; gap: 0.4rem;">
          <button type="button" class="btn btn-outline btn-sm" :disabled="page <= 1" @click="changePage(page - 1)">Prev</button>
          <button type="button" class="btn btn-outline btn-sm" :disabled="page * limit >= total" @click="changePage(page + 1)">Next</button>
        </div>
      </div>
    </div>
  </AdminLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { apiRequest } from "../api/client";
import AdminLayout from "../layouts/AdminLayout.vue";
import type { AuditLog } from "../types";

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

