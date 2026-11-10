<template>
  <div style="min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 1rem;">
    <div class="card" style="max-width: 400px; width: 100%;">
      <h1 style="margin: 0 0 0.25rem; font-size: 1.35rem;">BaraMail Admin</h1>
      <p style="margin: 0 0 1.25rem; color: var(--text-muted); font-size: 0.875rem;">
        Masuk untuk mengelola domain, mailbox, dan masa aktif.
      </p>

      <form @submit.prevent="submitLogin">
        <div style="margin-bottom: 0.9rem;">
          <label for="admin-user" style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Username</label>
          <input id="admin-user" v-model="username" class="input" required autocomplete="username" />
        </div>

        <div style="margin-bottom: 1.1rem;">
          <label for="admin-pass" style="display: block; font-weight: 600; font-size: 0.85rem; margin-bottom: 0.35rem;">Password</label>
          <input id="admin-pass" v-model="password" type="password" class="input" required autocomplete="current-password" />
        </div>

        <div v-if="errorMsg" class="error-alert" style="margin-bottom: 1rem;">
          {{ errorMsg }}
        </div>

        <button type="submit" class="btn" style="width: 100%; padding: 0.6rem;" :disabled="loading">
          {{ loading ? 'Memverifikasi...' : 'Login' }}
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { ApiError } from "../api/client";
import { useAdminStore } from "../stores/admin";

const adminStore = useAdminStore();
const router = useRouter();
const username = ref<string>("");
const password = ref<string>("");
const loading = ref<boolean>(false);
const errorMsg = ref<string>("");

async function submitLogin(): Promise<void> {
  loading.value = true;
  errorMsg.value = "";
  try {
    await adminStore.login(username.value.trim(), password.value);
    await router.push("/admin");
  } catch (err) {
    errorMsg.value = err instanceof ApiError ? err.message : "Username atau password salah.";
  } finally {
    loading.value = false;
  }
}
</script>

