<template>
  <div class="modal-backdrop" @click.self="$emit('close')">
    <div class="modal-panel" style="max-width: 680px;">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
        <h3 style="margin: 0;">Bulk Create Mailbox</h3>
        <button class="btn btn-outline btn-sm" type="button" @click="$emit('close')">Tutup</button>
      </div>

      <form v-if="!report" @submit.prevent="submitBulk">
        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; margin-bottom: 1rem;">
          <div>
            <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Domain Aktif</label>
            <select v-model.number="domainId" class="select" required>
              <option v-for="d in activeDomains" :key="d.id" :value="d.id">@{{ d.name }}</option>
            </select>
          </div>
          <div>
            <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Masa Aktif (Hari)</label>
            <input v-model.number="expiresInDays" type="number" min="1" :max="maxExpiryDays" class="input" required />
          </div>
        </div>

        <div style="margin-bottom: 1rem;">
          <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Mode Bulk</label>
          <div style="display: flex; gap: 0.5rem; flex-wrap: wrap;">
            <button type="button" class="btn btn-sm" :class="mode === 'random' ? '' : 'btn-outline'" @click="mode = 'random'">
              Jumlah Alamat Acak
            </button>
            <button type="button" class="btn btn-sm" :class="mode === 'sequence' ? '' : 'btn-outline'" @click="mode = 'sequence'">
              Prefix + Nomor Urut
            </button>
            <button type="button" class="btn btn-sm" :class="mode === 'list' ? '' : 'btn-outline'" @click="mode = 'list'">
              Tempel Daftar Custom
            </button>
          </div>
        </div>

        <div v-if="mode === 'random'" style="display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; margin-bottom: 1rem;">
          <div>
            <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Jumlah Mailbox (Maks {{ maxBulk }})</label>
            <input v-model.number="count" type="number" min="1" :max="maxBulk" class="input" required />
          </div>
          <div>
            <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Panjang Local-Part</label>
            <input v-model.number="length" type="number" min="6" max="64" class="input" required />
          </div>
        </div>

        <div v-else-if="mode === 'sequence'" style="display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 0.75rem; margin-bottom: 1rem;">
          <div>
            <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Prefix</label>
            <input v-model="prefix" class="input" placeholder="mis. user-" required />
          </div>
          <div>
            <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Mulai Nomor</label>
            <input v-model.number="start" type="number" min="1" class="input" required />
          </div>
          <div>
            <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Jumlah (Maks {{ maxBulk }})</label>
            <input v-model.number="count" type="number" min="1" :max="maxBulk" class="input" required />
          </div>
        </div>

        <div v-else style="margin-bottom: 1rem;">
          <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">
            Daftar Local-Part (Satu per baris, maks {{ maxBulk }})
          </label>
          <textarea
            v-model="rawAddressList"
            rows="6"
            class="textarea"
            placeholder="budi01&#10;budi02&#10;toko-a"
            required
          ></textarea>
        </div>

        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; margin-bottom: 1rem;">
          <div>
            <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Owner Ref (Opsional)</label>
            <input v-model="ownerRef" class="input" placeholder="mis. cust-0012" />
          </div>
          <div>
            <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Catatan (Opsional)</label>
            <input v-model="note" class="input" placeholder="mis. batch november" />
          </div>
        </div>

        <div v-if="errorMsg" class="error-alert" style="margin-bottom: 1rem;">{{ errorMsg }}</div>

        <div style="display: flex; justify-content: flex-end; gap: 0.5rem;">
          <button type="button" class="btn btn-outline" @click="$emit('close')">Batal</button>
          <button type="submit" class="btn" :disabled="submitting || activeDomains.length === 0">
            {{ submitting ? 'Memproses...' : 'Jalankan Bulk Create' }}
          </button>
        </div>
      </form>

      <div v-else>
        <div style="display: flex; gap: 1rem; margin-bottom: 1rem; flex-wrap: wrap;">
          <span class="badge badge-active">Dibuat: {{ report.created_count }}</span>
          <span class="badge badge-disabled">Dilewati (Duplikat): {{ report.skipped_count }}</span>
          <span class="badge badge-expired">Tidak Valid: {{ report.invalid_count }}</span>
        </div>

        <div class="table-wrap" style="max-height: 300px; overflow-y: auto; margin-bottom: 1rem; border: 1px solid var(--border);">
          <table>
            <thead>
              <tr>
                <th>Alamat / Input</th>
                <th>Status</th>
                <th>Keterangan</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(item, idx) in report.items" :key="idx">
                <td><code>{{ item.email || item.input }}</code></td>
                <td>
                  <span
                    class="badge"
                    :class="{
                      'badge-active': item.status === 'created',
                      'badge-disabled': item.status === 'skipped_duplicate',
                      'badge-expired': item.status === 'invalid',
                    }"
                  >
                    {{ item.status }}
                  </span>
                </td>
                <td>{{ item.reason || '-' }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div style="display: flex; justify-content: space-between; align-items: center;">
          <button type="button" class="btn btn-outline btn-sm" @click="downloadReportCSV">Unduh Hasil CSV</button>
          <button type="button" class="btn" @click="$emit('completed')">Selesai</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ApiError, apiRequest } from "../api/client";
import { useAdminStore } from "../stores/admin";
import type { BulkCreateReport, Domain } from "../types";

const props = defineProps<{
  domains: Domain[];
}>();

defineEmits<{
  (e: "close"): void;
  (e: "completed"): void;
}>();

const adminStore = useAdminStore();
const activeDomains = computed(() => props.domains.filter((d) => d.status === "active"));
const domainId = ref<number>(0);
const mode = ref<"random" | "sequence" | "list">("random");
const count = ref<number>(10);
const length = ref<number>(adminStore.serverDefaults.random_localpart_length || 12);
const prefix = ref<string>("user");
const start = ref<number>(1);
const rawAddressList = ref<string>("");
const expiresInDays = ref<number>(adminStore.serverDefaults.default_mailbox_expiry_days || 7);
const ownerRef = ref<string>("");
const note = ref<string>("");
const submitting = ref<boolean>(false);
const errorMsg = ref<string>("");
const report = ref<BulkCreateReport | null>(null);

const maxBulk = computed(() => adminStore.serverDefaults.max_bulk_create || 200);
const maxExpiryDays = computed(() => adminStore.serverDefaults.max_mailbox_expiry_days || 365);

onMounted(() => {
  const firstDomain = activeDomains.value[0];
  if (firstDomain) {
    domainId.value = firstDomain.id;
  }
});

async function submitBulk(): Promise<void> {
  submitting.value = true;
  errorMsg.value = "";
  try {
    const addresses =
      mode.value === "list"
        ? rawAddressList.value
            .split(/\r?\n/)
            .map((s) => s.trim())
            .filter(Boolean)
        : undefined;

    report.value = await apiRequest<BulkCreateReport>("/admin/mailboxes/bulk", {
      method: "POST",
      csrfToken: adminStore.csrfToken,
      body: JSON.stringify({
        domain_id: domainId.value,
        mode: mode.value,
        addresses,
        prefix: mode.value === "sequence" ? prefix.value.trim() : undefined,
        start: mode.value === "sequence" ? start.value : undefined,
        count: mode.value !== "list" ? count.value : undefined,
        length: mode.value === "random" ? length.value : undefined,
        expires_in_days: expiresInDays.value,
        owner_ref: ownerRef.value.trim() || null,
        note: note.value.trim() || null,
      }),
    });
  } catch (err) {
    errorMsg.value = err instanceof ApiError ? err.message : "Gagal memproses bulk create.";
  } finally {
    submitting.value = false;
  }
}

function downloadReportCSV(): void {
  if (!report.value) return;
  const rows = [["email", "input", "status", "reason"]];
  for (const item of report.value.items) {
    rows.push([item.email || "", item.input || "", item.status, item.reason || ""]);
  }
  const csvContent = rows.map((r) => r.map((v) => `"${v.replace(/"/g, '""')}"`).join(",")).join("\n");
  const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `baramail-bulk-report-${Date.now()}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}
</script>

