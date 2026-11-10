<template>
  <AdminLayout>
    <h1 style="margin: 0 0 1.25rem; font-size: 1.4rem;">Manajemen Domain</h1>

    <div class="card" style="margin-bottom: 1.25rem;">
      <form style="display: flex; gap: 0.65rem; flex-wrap: wrap; align-items: end;" @submit.prevent="addDomain">
        <div style="flex: 1; min-width: 240px;">
          <label style="display: block; font-size: 0.85rem; font-weight: 600; margin-bottom: 0.3rem;">Tambah Domain Baru</label>
          <input v-model="newDomainName" class="input" placeholder="mis. domain-baru.tld" required />
        </div>
        <button type="submit" class="btn" :disabled="submitting">Tambah Domain</button>
      </form>
      <p style="margin: 0.6rem 0 0; font-size: 0.8rem; color: var(--text-muted);">
        Pastikan domain telah dikonfigurasi pada Cloudflare DNS dan Email Routing sebelum digunakan.
      </p>
    </div>

    <div v-if="errorMsg" class="error-alert" style="margin-bottom: 1rem;">{{ errorMsg }}</div>

    <div class="card" style="padding: 0;">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Nama Domain</th>
              <th>Status</th>
              <th>Jumlah Mailbox</th>
              <th>Dibuat</th>
              <th>Aksi</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="d in domains" :key="d.id">
              <td><strong>{{ d.name }}</strong></td>
              <td>
                <span class="badge" :class="d.status === 'active' ? 'badge-active' : 'badge-retired'">
                  {{ d.status }}
                </span>
              </td>
              <td>{{ d.mailbox_count }}</td>
              <td>{{ new Date(d.created_at).toLocaleString('id-ID') }}</td>
              <td>
                <button
                  v-if="d.status === 'active'"
                  type="button"
                  class="btn btn-danger btn-sm"
                  @click="toggleDomainStatus(d, 'retired')"
                >
                  Retire Domain
                </button>
                <button
                  v-else
                  type="button"
                  class="btn btn-outline btn-sm"
                  @click="toggleDomainStatus(d, 'active')"
                >
                  Aktifkan Kembali
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </AdminLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ApiError, apiRequest } from "../api/client";
import AdminLayout from "../layouts/AdminLayout.vue";
import { useAdminStore } from "../stores/admin";
import type { Domain } from "../types";

const adminStore = useAdminStore();
const domains = ref<Domain[]>([]);
const newDomainName = ref<string>("");
const submitting = ref<boolean>(false);
const errorMsg = ref<string>("");

async function loadDomains(): Promise<void> {
  try {
    domains.value = (await apiRequest<Domain[]>("/admin/domains")) || [];
  } catch (err) {
    errorMsg.value = err instanceof ApiError ? err.message : "Gagal memuat daftar domain.";
  }
}

async function addDomain(): Promise<void> {
  submitting.value = true;
  errorMsg.value = "";
  try {
    await apiRequest("/admin/domains", {
      method: "POST",
      csrfToken: adminStore.csrfToken,
      body: JSON.stringify({ name: newDomainName.value.trim().toLowerCase() }),
    });
    newDomainName.value = "";
    await loadDomains();
  } catch (err) {
    errorMsg.value = err instanceof ApiError ? err.message : "Gagal menambahkan domain.";
  } finally {
    submitting.value = false;
  }
}

async function toggleDomainStatus(d: Domain, nextStatus: "active" | "retired"): Promise<void> {
  const msg =
    nextStatus === "retired"
      ? `Retire domain ${d.name}? Seluruh mailbox pada domain ini tidak akan bisa diakses publik dan tidak menerima email baru.`
      : `Aktifkan kembali domain ${d.name}?`;
  if (!window.confirm(msg)) return;

  errorMsg.value = "";
  try {
    await apiRequest(`/admin/domains/${d.id}`, {
      method: "PATCH",
      csrfToken: adminStore.csrfToken,
      body: JSON.stringify({ status: nextStatus }),
    });
    await loadDomains();
  } catch (err) {
    errorMsg.value = err instanceof ApiError ? err.message : "Gagal mengubah status domain.";
  }
}

onMounted(() => {
  void loadDomains();
});
</script>

