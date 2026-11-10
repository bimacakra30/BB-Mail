<template>
  <div class="modal-backdrop" @click.self="$emit('close')">
    <div class="modal wide" role="dialog" aria-modal="true" aria-labelledby="bulk-title">
      <div class="modal-head">
        <h2 id="bulk-title">Buat massal</h2>
        <button type="button" class="btn sm" @click="$emit('close')">Tutup</button>
      </div>

      <form v-if="!report" class="stack" @submit.prevent="submitBulk">
        <div class="grid-2">
          <div class="field">
            <label for="b-domain">Domain</label>
            <select id="b-domain" v-model.number="domainId" class="select" required>
              <option v-for="d in activeDomains" :key="d.id" :value="d.id">@{{ d.name }}</option>
            </select>
          </div>
          <div class="field">
            <label for="b-expiry">Masa aktif (hari)</label>
            <input id="b-expiry" v-model.number="expiresInDays" type="number" min="1" :max="maxExpiryDays" class="input" required />
          </div>
        </div>

        <div class="field">
          <span class="label">Cara membuat</span>
          <div class="row" role="radiogroup" aria-label="Cara membuat">
            <button type="button" role="radio" :aria-checked="mode === 'random'" class="btn" :class="{ 'is-on': mode === 'random' }" @click="mode = 'random'">
              Alamat acak
            </button>
            <button type="button" role="radio" :aria-checked="mode === 'sequence'" class="btn" :class="{ 'is-on': mode === 'sequence' }" @click="mode = 'sequence'">
              Awalan + nomor urut
            </button>
            <button type="button" role="radio" :aria-checked="mode === 'list'" class="btn" :class="{ 'is-on': mode === 'list' }" @click="mode = 'list'">
              Tempel daftar
            </button>
          </div>
        </div>

        <div v-if="mode === 'random'" class="grid-2">
          <div class="field">
            <label for="b-count">Jumlah (maks {{ maxBulk }})</label>
            <input id="b-count" v-model.number="count" type="number" min="1" :max="maxBulk" class="input" required />
          </div>
          <div class="field">
            <label for="b-length">Panjang karakter</label>
            <input id="b-length" v-model.number="length" type="number" min="6" max="64" class="input" required />
          </div>
        </div>

        <div v-else-if="mode === 'sequence'" class="grid-3">
          <div class="field">
            <label for="b-prefix">Awalan</label>
            <input id="b-prefix" v-model="prefix" class="input" placeholder="user-" required autocapitalize="off" />
          </div>
          <div class="field">
            <label for="b-start">Mulai dari nomor</label>
            <input id="b-start" v-model.number="start" type="number" min="1" class="input" required />
          </div>
          <div class="field">
            <label for="b-count2">Jumlah (maks {{ maxBulk }})</label>
            <input id="b-count2" v-model.number="count" type="number" min="1" :max="maxBulk" class="input" required />
          </div>
        </div>

        <div v-else class="field">
          <label for="b-list">Daftar nama alamat (satu per baris, maks {{ maxBulk }})</label>
          <textarea id="b-list" v-model="rawAddressList" rows="6" class="textarea" placeholder="budi01&#10;budi02&#10;toko-a" required></textarea>
        </div>

        <div class="grid-2">
          <div class="field">
            <label for="b-owner">Owner ref (opsional)</label>
            <input id="b-owner" v-model="ownerRef" class="input" placeholder="cust-0012" />
          </div>
          <div class="field">
            <label for="b-note">Catatan (opsional)</label>
            <input id="b-note" v-model="note" class="input" placeholder="Batch November" />
          </div>
        </div>

        <div v-if="errorMsg" class="error" role="alert">{{ errorMsg }}</div>

        <div class="modal-actions">
          <button type="button" class="btn" @click="$emit('close')">Batal</button>
          <button type="submit" class="btn primary" :disabled="submitting || activeDomains.length === 0">
            {{ submitting ? "Memproses..." : "Buat mailbox" }}
          </button>
        </div>
      </form>

      <div v-else class="stack">
        <div class="row">
          <span class="badge ok">Dibuat {{ report.created_count }}</span>
          <span class="badge off">Dilewati (sudah ada) {{ report.skipped_count }}</span>
          <span class="badge bad">Tidak valid {{ report.invalid_count }}</span>
        </div>

        <div class="table-wrap" style="max-height: 320px; overflow-y: auto;">
          <table>
            <thead>
              <tr>
                <th>Alamat</th>
                <th>Hasil</th>
                <th>Keterangan</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(item, idx) in report.items" :key="idx">
                <td class="cell-main">{{ item.email || item.input }}</td>
                <td>
                  <span class="badge" :class="resultBadge[item.status].cls">{{ resultBadge[item.status].label }}</span>
                </td>
                <td class="meta">{{ item.reason || "-" }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="modal-actions">
          <button type="button" class="btn" @click="downloadReportCSV">Unduh hasil CSV</button>
          <button type="button" class="btn primary" @click="$emit('completed')">Selesai</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ApiError, apiRequest } from "../api/client";
import { useAdminStore } from "../stores/admin";
import type { BulkCreateReport, BulkItemResult, Domain } from "../types";

const props = defineProps<{
  domains: Domain[];
}>();

defineEmits<{
  (e: "close"): void;
  (e: "completed"): void;
}>();

const resultBadge: Record<BulkItemResult["status"], { label: string; cls: string }> = {
  created: { label: "Dibuat", cls: "ok" },
  skipped_duplicate: { label: "Sudah ada", cls: "off" },
  invalid: { label: "Tidak valid", cls: "bad" },
};

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
    errorMsg.value = err instanceof ApiError ? err.message : "Pembuatan massal gagal. Periksa isian lalu coba lagi.";
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
  a.download = `baramail-hasil-buat-massal-${Date.now()}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}
</script>
