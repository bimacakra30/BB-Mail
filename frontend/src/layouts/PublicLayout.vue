<template>
  <div class="page">
    <header class="site-header">
      <div class="container">
        <RouterLink to="/" class="logo">BaraMail</RouterLink>
        <nav class="site-nav" aria-label="Navigasi utama">
          <RouterLink to="/">Buka inbox</RouterLink>
          <RouterLink to="/terms">Syarat layanan</RouterLink>
          <RouterLink to="/abuse">Kebijakan abuse</RouterLink>
        </nav>
      </div>
    </header>

    <main class="site-main">
      <div class="container">
        <slot />
      </div>
    </main>

    <footer class="site-footer">
      <div class="container">
        <span>BaraMail hanya menerima email. Tidak untuk akun penting atau finansial.</span>
        <nav aria-label="Tautan footer">
          <RouterLink to="/terms">Syarat layanan</RouterLink>
          <RouterLink to="/abuse">Lapor penyalahgunaan</RouterLink>
        </nav>
      </div>
    </footer>
  </div>
</template>
