<template>
  <div style="min-height: 100vh; display: flex; flex-direction: column;">
    <header style="background: #0f172a; color: #ffffff; padding: 0.85rem 0;">
      <div class="container" style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 0.75rem;">
        <div style="display: flex; align-items: center; gap: 1.5rem;">
          <RouterLink to="/admin" style="font-size: 1.15rem; font-weight: 700; color: #ffffff; text-decoration: none;">
            BaraMail Admin
          </RouterLink>
          <nav style="display: flex; gap: 1rem; font-size: 0.875rem;">
            <RouterLink to="/admin" style="color: #cbd5e1;">Mailboxes</RouterLink>
            <RouterLink to="/admin/domains" style="color: #cbd5e1;">Domains</RouterLink>
            <RouterLink to="/admin/audit-logs" style="color: #cbd5e1;">Audit Logs</RouterLink>
          </nav>
        </div>
        <div style="display: flex; align-items: center; gap: 0.75rem; font-size: 0.85rem;">
          <span v-if="adminStore.user" style="color: #94a3b8;">{{ adminStore.user.username }}</span>
          <button type="button" class="btn btn-outline btn-sm" @click="handleLogout">Logout</button>
        </div>
      </div>
    </header>

    <main class="container" style="flex: 1; padding-top: 1.5rem; padding-bottom: 2.5rem; width: 100%;">
      <slot />
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

