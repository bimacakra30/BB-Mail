<template>
  <div class="login-wrap">
    <form class="panel stack login-panel" @submit.prevent="submitLogin">
      <span class="logo">BaraMail</span>
      <div>
        <h1 style="font-size: 28px; font-weight: 800;">Masuk</h1>
        <p class="muted">Kelola mailbox, domain, dan masa aktif.</p>
      </div>

      <div class="field">
        <label for="admin-user">Username</label>
        <input id="admin-user" v-model="username" class="input" required autocomplete="username" autocapitalize="off" />
      </div>

      <div class="field">
        <label for="admin-pass">Password</label>
        <input id="admin-pass" v-model="password" type="password" class="input" required autocomplete="current-password" />
      </div>

      <div v-if="errorMsg" class="error" role="alert">{{ errorMsg }}</div>

      <button type="submit" class="btn primary lg" :disabled="loading">
        {{ loading ? "Memeriksa..." : "Masuk" }}
      </button>
    </form>
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
    if (err instanceof ApiError && (err.code === "ACCOUNT_LOCKED" || err.code === "RATE_LIMITED")) {
      errorMsg.value = "Terlalu banyak percobaan masuk. Tunggu 15 menit lalu coba lagi.";
    } else {
      errorMsg.value = "Username atau password salah. Periksa lagi lalu coba masuk kembali.";
    }
  } finally {
    loading.value = false;
  }
}
</script>
