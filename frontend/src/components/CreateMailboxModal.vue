<template>
  <div class="modal-backdrop" @click.self="$emit('close')">
    <form class="modal" role="dialog" aria-modal="true" aria-labelledby="create-title" @submit.prevent="submitForm">
      <div class="modal-head">
        <h2 id="create-title">Buat mailbox</h2>
        <button type="button" class="btn sm" @click="$emit('close')">Tutup</button>
      </div>

      <div v-if="activeDomains.length === 0" class="warn">
        Belum ada domain aktif. Tambahkan atau aktifkan domain di halaman Domain terlebih dulu.
      </div>

      <div class="field">
        <label for="c-domain">Domain</label>
        <select id="c-domain" v-model.number="domainId" class="select" required>
          <option v-for="d in activeDomains" :key="d.id" :value="d.id">@{{ d.name }}</option>
        </select>
      </div>

      <div class="field">
        <span class="label">Jenis alamat</span>
        <div class="row" role="radiogroup" aria-label="Jenis alamat">
          <button type="button" role="radio" :aria-checked="mode === 'custom'" class="btn" :class="{ 'is-on': mode === 'custom' }" @click="mode = 'custom'">
            Nama sendiri
          </button>
          <button type="button" role="radio" :aria-checked="mode === 'random'" class="btn" :class="{ 'is-on': mode === 'random' }" @click="switchToRandom">
            Acak
          </button>
        </div>
      </div>

      <template v-if="mode === 'custom'">
        <div class="field">
          <label for="c-local">Nama alamat</label>
          <div class="suffix-input">
            <input id="c-local" v-model="localPart" class="input" placeholder="budi.01" required pattern="[a-zA-Z0-9._\-]+" autocapitalize="off" spellcheck="false" />
            <span class="suffix">@{{ selectedDomainName }}</span>
          </div>
        </div>
        <div class="warn">
          <strong>Alamat nama sendiri mudah ditebak.</strong>
          Siapa pun yang menebak <code>{{ (localPart || "nama") + "@" + selectedDomainName }}</code> bisa membaca OTP di dalamnya.
        </div>
      </template>

      <div v-else class="field">
        <label for="c-length">Panjang karakter</label>
        <input id="c-length" v-model.number="length" type="number" min="6" max="64" class="input" style="max-width: 140px;" @input="generatePreview" />
        <span class="hint">Contoh format: <code>{{ randomPreview }}@{{ selectedDomainName }}</code>. Alamat final dibuat acak oleh server.</span>
      </div>

      <div class="field">
        <label for="c-expiry">Masa aktif (hari)</label>
        <div class="row">
          <button
            v-for="preset in [1, 3, 7, 30]"
            :key="preset"
            type="button"
            class="btn sm"
            :class="{ 'is-on': expiresInDays === preset }"
            @click="expiresInDays = preset"
          >
            {{ preset }} hari
          </button>
        </div>
        <input id="c-expiry" v-model.number="expiresInDays" type="number" min="1" :max="maxExpiryDays" class="input" required />
      </div>

      <div class="grid-2">
        <div class="field">
          <label for="c-owner">Owner ref (opsional)</label>
          <input id="c-owner" v-model="ownerRef" class="input" placeholder="cust-0012" maxlength="100" />
        </div>
        <div class="field">
          <label for="c-note">Catatan (opsional)</label>
          <input id="c-note" v-model="note" class="input" placeholder="Paket 1 minggu" maxlength="255" />
        </div>
      </div>

      <div v-if="errorMsg" class="error" role="alert">{{ errorMsg }}</div>

      <div class="modal-actions">
        <button type="button" class="btn" @click="$emit('close')">Batal</button>
        <button type="submit" class="btn primary" :disabled="submitting || activeDomains.length === 0">
          {{ submitting ? "Menyimpan..." : "Buat mailbox" }}
        </button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ApiError, apiRequest } from "../api/client";
import { useAdminStore } from "../stores/admin";
import type { Domain, Mailbox } from "../types";

const props = defineProps<{
  domains: Domain[];
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "created", mailbox: Mailbox): void;
}>();

const adminStore = useAdminStore();

const activeDomains = computed(() => props.domains.filter((d) => d.status === "active"));
const domainId = ref<number>(0);
const mode = ref<"custom" | "random">("custom");
const localPart = ref<string>("");
const length = ref<number>(adminStore.serverDefaults.random_localpart_length || 12);
const expiresInDays = ref<number>(adminStore.serverDefaults.default_mailbox_expiry_days || 7);
const maxExpiryDays = computed(() => adminStore.serverDefaults.max_mailbox_expiry_days || 365);
const ownerRef = ref<string>("");
const note = ref<string>("");
const randomPreview = ref<string>("");
const submitting = ref<boolean>(false);
const errorMsg = ref<string>("");

const selectedDomainName = computed(() => {
  const found = activeDomains.value.find((d) => d.id === domainId.value);
  return found ? found.name : "domain.tld";
});

function generatePreview(): void {
  const alphabet = "abcdefghjkmnpqrstuvwxyz23456789";
  const len = Math.max(6, Math.min(64, length.value || 12));
  let out = "";
  for (let i = 0; i < len; i++) {
    out += alphabet[Math.floor(Math.random() * alphabet.length)];
  }
  randomPreview.value = out;
}

function switchToRandom(): void {
  mode.value = "random";
  generatePreview();
}

onMounted(() => {
  const firstDomain = activeDomains.value[0];
  if (firstDomain) {
    domainId.value = firstDomain.id;
  }
  generatePreview();
});

async function submitForm(): Promise<void> {
  submitting.value = true;
  errorMsg.value = "";
  try {
    const created = await apiRequest<Mailbox>("/admin/mailboxes", {
      method: "POST",
      csrfToken: adminStore.csrfToken,
      body: JSON.stringify({
        domain_id: domainId.value,
        mode: mode.value,
        local_part: mode.value === "custom" ? localPart.value.trim().toLowerCase() : undefined,
        length: mode.value === "random" ? length.value : undefined,
        expires_in_days: expiresInDays.value,
        owner_ref: ownerRef.value.trim() || null,
        note: note.value.trim() || null,
      }),
    });
    emit("created", created);
  } catch (err) {
    errorMsg.value = err instanceof ApiError ? err.message : "Mailbox gagal dibuat. Periksa isian lalu coba lagi.";
  } finally {
    submitting.value = false;
  }
}
</script>
