import { createRouter, createWebHistory } from "vue-router";
import AdminAuditPage from "../pages/AdminAuditPage.vue";
import AdminDashboardPage from "../pages/AdminDashboardPage.vue";
import AdminDomainsPage from "../pages/AdminDomainsPage.vue";
import AdminLoginPage from "../pages/AdminLoginPage.vue";
import HomePage from "../pages/HomePage.vue";
import InboxPage from "../pages/InboxPage.vue";
import LegalPage from "../pages/LegalPage.vue";
import { useAdminStore } from "../stores/admin";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/",
      name: "home",
      component: HomePage,
    },
    {
      path: "/inbox",
      name: "inbox",
      component: InboxPage,
    },
    {
      path: "/terms",
      name: "terms",
      component: LegalPage,
      props: { pageType: "terms" },
    },
    {
      path: "/abuse",
      name: "abuse",
      component: LegalPage,
      props: { pageType: "abuse" },
    },
    {
      path: "/admin/login",
      name: "admin-login",
      component: AdminLoginPage,
    },
    {
      path: "/admin",
      name: "admin-dashboard",
      component: AdminDashboardPage,
      meta: { requiresAdmin: true },
    },
    {
      path: "/admin/domains",
      name: "admin-domains",
      component: AdminDomainsPage,
      meta: { requiresAdmin: true },
    },
    {
      path: "/admin/audit-logs",
      name: "admin-audit",
      component: AdminAuditPage,
      meta: { requiresAdmin: true },
    },
  ],
});

router.beforeEach(async (to) => {
  if (to.meta.requiresAdmin) {
    const adminStore = useAdminStore();
    if (!adminStore.user) {
      const ok = await adminStore.fetchMe();
      if (!ok) {
        return { name: "admin-login" };
      }
    }
  }
  return true;
});

export default router;

