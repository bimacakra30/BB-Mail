import { defineStore } from "pinia";
import { ref } from "vue";
import { apiRequest } from "../api/client";

export interface AdminUser {
  id: number;
  username: string;
  email?: string;
}

export interface ServerConfigDefaults {
  random_localpart_length: number;
  default_mailbox_expiry_days: number;
  max_mailbox_expiry_days: number;
  max_bulk_create: number;
}

export const useAdminStore = defineStore("admin", () => {
  const user = ref<AdminUser | null>(null);
  const csrfToken = ref<string>("");
  const checkedAuth = ref<boolean>(false);
  const serverDefaults = ref<ServerConfigDefaults>({
    random_localpart_length: 12,
    default_mailbox_expiry_days: 7,
    max_mailbox_expiry_days: 365,
    max_bulk_create: 200,
  });

  async function login(username: string, password: string): Promise<void> {
    const data = await apiRequest<{
      user: AdminUser;
      csrf_token: string;
    }>("/admin/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    });
    user.value = data.user;
    csrfToken.value = data.csrf_token;
    checkedAuth.value = true;
  }

  async function fetchMe(): Promise<boolean> {
    try {
      const data = await apiRequest<{
        user: AdminUser;
        csrf_token: string;
        config?: ServerConfigDefaults;
      }>("/admin/me");
      user.value = data.user;
      csrfToken.value = data.csrf_token;
      if (data.config) {
        serverDefaults.value = data.config;
      }
      checkedAuth.value = true;
      return true;
    } catch {
      user.value = null;
      csrfToken.value = "";
      checkedAuth.value = true;
      return false;
    }
  }

  async function logout(): Promise<void> {
    try {
      await apiRequest("/admin/logout", {
        method: "POST",
        csrfToken: csrfToken.value,
      });
    } finally {
      user.value = null;
      csrfToken.value = "";
    }
  }

  return {
    user,
    csrfToken,
    checkedAuth,
    serverDefaults,
    login,
    fetchMe,
    logout,
  };
});

