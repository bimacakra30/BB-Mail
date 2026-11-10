<template>
  <div class="page">
    <header class="admin-bar">
      <div class="container">
        <RouterLink to="/admin" class="logo">BaraMail</RouterLink>
        <nav aria-label="Navigasi admin">
          <RouterLink to="/admin">Mailbox</RouterLink>
          <RouterLink to="/admin/domains">Domain</RouterLink>
          <RouterLink to="/admin/audit-logs">Log audit</RouterLink>
        </nav>
        <div class="user">
          <span v-if="adminStore.user">{{ adminStore.user.username }}</span>
          <button type="button" class="btn sm" @click="handleLogout">Keluar</button>
        </div>
      </div>
    </header>

    <main class="site-main">
      <div class="container">
        <slot />
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from "vue-router";
import { useAdminStore } from "../stores/admin";

const adminStore = useAdminStore();
const router = useRouter();

async function handleLogout(): Promise<void> {
  await adminStore.logout();
  await router.push("/admin/login");
}
</script>
