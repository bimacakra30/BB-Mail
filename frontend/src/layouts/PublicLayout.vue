<template>
  <div style="min-height: 100vh; display: flex; flex-direction: column;">
    <header style="background: var(--surface); border-bottom: 1px solid var(--border); padding: 0.9rem 0;">
      <div class="container" style="display: flex; justify-content: space-between; align-items: center;">
        <RouterLink to="/" style="font-size: 1.25rem; font-weight: 700; color: var(--text); text-decoration: none;">
          BaraMail
        </RouterLink>
        <nav style="display: flex; gap: 1rem; font-size: 0.875rem;">
          <RouterLink to="/">Buka Inbox</RouterLink>
          <RouterLink to="/terms">Syarat Layanan</RouterLink>
          <RouterLink to="/abuse">Kebijakan Abuse</RouterLink>
        </nav>
      </div>
    </header>

    <main class="container" style="flex: 1; padding-top: 1.75rem; padding-bottom: 2.5rem; width: 100%;">
      <slot />
    </main>

    <footer style="border-top: 1px solid var(--border); padding: 1.25rem 0; font-size: 0.8rem; color: var(--text-muted);">
      <div class="container" style="display: flex; justify-content: space-between; flex-wrap: wrap; gap: 0.5rem;">
        <span>BaraMail — Inbound-Only OTP Inbox. Tidak untuk akun penting atau finansial.</span>
        <div style="display: flex; gap: 1rem;">
          <RouterLink to="/terms">Syarat Layanan</RouterLink>
          <RouterLink to="/abuse">Lapor Penyalahgunaan ({{ abuseEmail }})</RouterLink>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
const abuseEmail = import.meta.env.VITE_ABUSE_EMAIL || "abuse@example.com";
</script>

