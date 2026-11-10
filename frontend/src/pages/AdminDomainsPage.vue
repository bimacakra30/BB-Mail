<template>
  <AdminLayout>
    <div class="page-head">
      <h1>Domain</h1>
    </div>

    <form class="panel stack" style="margin-bottom: 20px;" @submit.prevent="addDomain">
      <div class="row" style="align-items: flex-end;">
        <div class="field" style="flex: 1; min-width: 240px;">
          <label for="new-domain">Tambah domain</label>
          <input id="new-domain" v-model="newDomainName" class="input" placeholder="domain-baru.tld" required />
        </div>
        <button type="submit" class="btn primary" :disabled="submitting">Tambah domain</button>
      </div>
      <p class="hint">Atur Email Routing di Cloudflare untuk domain ini sebelum membuat mailbox di dalamnya.</p>
    </form>

    <div v-if="errorMsg" class="error" role="alert" style="margin-bottom: 16px;">{{ errorMsg }}</div>

    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Nama domain</th>
            <th>Status</th>
            <th>Jumlah mailbox</th>
            <th>Dibuat</th>
            <th class="actions">Aksi</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="domains.length === 0">
            <td colspan="5" class="muted" style="text-align: center; padding: 32px;">
              Belum ada domain. Tambahkan domain pertama di atas.
            </td>
          </tr>
          <tr v-for="d in domains" :key="d.id">
            <td class="cell-main">{{ d.name }}</td>
            <td>
              <span class="badge" :class="d.status === 'active' ? 'ok' : 'bad'">
                {{ d.status === "active" ? "Aktif" : "Dihentikan" }}
              </span>
            </td>
            <td>{{ d.mailbox_count }}</td>
            <td class="meta">{{ formatDateTime(d.created_at) }}</td>
            <td class="actions">
              <button v-if="d.status === 'active'" type="button" class="btn sm danger" @click="toggleDomainStatus(d, 'retired')">
                Hentikan domain
              </button>
              <button v-else type="button" class="btn sm" @click="toggleDomainStatus(d, 'active')">Aktifkan kembali</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </AdminLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ApiError, apiRequest } from "../api/client";
import AdminLayout from "../layouts/AdminLayout.vue";
import { useAdminStore } from "../stores/admin";
import { useToast } from "../stores/toast";
import type { Domain } from "../types";
import { formatDateTime } from "../utils/format";

const adminStore = useAdminStore();
const toast = useToast();
const domains = ref<Domain[]>([]);
const newDomainName = ref<string>("");
const submitting = ref<boolean>(false);
const errorMsg = ref<string>("");

async function loadDomains(): Promise<void> {
  try {
    domains.value = (await apiRequest<Domain[]>("/admin/domains")) || [];
  } catch (err) {
    errorMsg.value = err instanceof ApiError ? err.message : "Daftar domain gagal dimuat. Muat ulang halaman.";
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
    toast.show("Domain ditambahkan");
    await loadDomains();
  } catch (err) {
    errorMsg.value = err instanceof ApiError ? err.message : "Domain gagal ditambahkan. Periksa penulisan lalu coba lagi.";
  } finally {
    submitting.value = false;
  }
}

async function toggleDomainStatus(d: Domain, nextStatus: "active" | "retired"): Promise<void> {
  const msg =
    nextStatus === "retired"
      ? `Hentikan ${d.name}? Semua mailbox di domain ini langsung tidak bisa dibuka dan tidak menerima email baru.`
      : `Aktifkan kembali ${d.name}?`;
  if (!window.confirm(msg)) return;

  errorMsg.value = "";
  try {
    await apiRequest(`/admin/domains/${d.id}`, {
      method: "PATCH",
      csrfToken: adminStore.csrfToken,
      body: JSON.stringify({ status: nextStatus }),
    });
    toast.show(nextStatus === "retired" ? "Domain dihentikan" : "Domain diaktifkan kembali");
    await loadDomains();
  } catch (err) {
    errorMsg.value = err instanceof ApiError ? err.message : "Status domain gagal diubah. Coba lagi.";
  }
}

onMounted(() => {
  void loadDomains();
});
</script>
