<template>
  <div class="modal-backdrop" @click.self="$emit('close')">
    <div class="modal-panel">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
        <h3 style="margin: 0;">Buat Mailbox Baru</h3>
        <button class="btn btn-outline btn-sm" type="button" @click="$emit('close')">Tutup</button>
      </div>

      <form @submit.prevent="submitForm">
        <div style="margin-bottom: 1rem;">
          <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Domain Aktif</label>
          <select v-model.number="domainId" class="select" required>
            <option v-for="d in activeDomains" :key="d.id" :value="d.id">@{{ d.name }}</option>
          </select>
        </div>

        <div style="margin-bottom: 1rem;">
          <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Mode Alamat</label>
          <div style="display: flex; gap: 0.5rem;">
            <button
              type="button"
              class="btn"
              :class="mode === 'custom' ? '' : 'btn-outline'"
              @click="mode = 'custom'"
            >
              Custom (Default)
            </button>
            <button
              type="button"
              class="btn"
              :class="mode === 'random' ? '' : 'btn-outline'"
              @click="switchToRandom"
            >
              Acak (Capability Address)
            </button>
          </div>
        </div>

        <div v-if="mode === 'custom'" style="margin-bottom: 1rem;">
          <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Local-Part</label>
          <div style="display: flex; align-items: center; gap: 0.4rem;">
            <input
              v-model="localPart"
              class="input"
              placeholder="mis. budi.01"
              required
              pattern="[a-zA-Z0-9._-]+"
            />
            <span style="color: var(--text-muted); font-weight: 500;">@{{ selectedDomainName }}</span>
          </div>
          <div class="warning-banner" style="margin-top: 0.6rem; padding: 0.6rem 0.8rem; font-size: 0.8rem;">
            <strong>Perhatian:</strong> Alamat custom mudah ditebak. Siapa pun yang menebak
            <code>{{ (localPart || 'nama') + '@' + selectedDomainName }}</code> dapat membuka inbox dan membaca OTP di dalamnya.
          </div>
        </div>

        <div v-else style="margin-bottom: 1rem;">
          <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Panjang Karakter Acak</label>
          <div style="display: flex; align-items: center; gap: 0.5rem;">
            <input v-model.number="length" type="number" min="6" max="64" class="input" style="max-width: 110px;" @input="generatePreview" />
            <button type="button" class="btn btn-outline btn-sm" @click="generatePreview">Perbarui Contoh</button>
          </div>
          <p style="margin: 0.4rem 0 0; font-size: 0.8rem; color: var(--text-muted);">
            Pratinjau format (dibuat oleh CSPRNG server): <code>{{ randomPreview }}@{{ selectedDomainName }}</code>
          </p>
        </div>

        <div style="margin-bottom: 1rem;">
          <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Masa Aktif (Hari)</label>
          <div style="display: flex; gap: 0.4rem; flex-wrap: wrap; margin-bottom: 0.5rem;">
            <button
              v-for="preset in [1, 3, 7, 30]"
              :key="preset"
              type="button"
              class="btn btn-sm"
              :class="expiresInDays === preset ? '' : 'btn-outline'"
              @click="expiresInDays = preset"
            >
              {{ preset }} Hari
            </button>
          </div>
          <input v-model.number="expiresInDays" type="number" min="1" :max="maxExpiryDays" class="input" required />
        </div>

        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; margin-bottom: 1rem;">
          <div>
            <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Owner Ref (Opsional)</label>
            <input v-model="ownerRef" class="input" placeholder="mis. cust-0012" maxlength="100" />
          </div>
          <div>
            <label style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Catatan (Opsional)</label>
            <input v-model="note" class="input" placeholder="mis. paket 1 minggu" maxlength="255" />
          </div>
        </div>

        <div v-if="errorMsg" class="error-alert" style="margin-bottom: 1rem;">{{ errorMsg }}</div>

        <div style="display: flex; justify-content: flex-end; gap: 0.5rem;">
          <button type="button" class="btn btn-outline" @click="$emit('close')">Batal</button>
          <button type="submit" class="btn" :disabled="submitting || activeDomains.length === 0">
            {{ submitting ? 'Menyimpan...' : 'Buat Mailbox' }}
          </button>
        </div>
      </form>
    </div>
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
    errorMsg.value = err instanceof ApiError ? err.message : "Gagal membuat mailbox.";
  } finally {
    submitting.value = false;
  }
}
</script>

