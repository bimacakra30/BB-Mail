import type { ApiResponse } from "../types";

const API_BASE = import.meta.env.VITE_API_BASE_URL || "/api/v1";

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

export async function apiRequest<T>(
  path: string,
  options: RequestInit & { csrfToken?: string; bearerToken?: string } = {},
): Promise<T> {
  const headers = new Headers(options.headers || {});
  if (options.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (options.csrfToken) {
    headers.set("X-CSRF-Token", options.csrfToken);
  }
  if (options.bearerToken) {
    headers.set("Authorization", `Bearer ${options.bearerToken}`);
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
    credentials: "same-origin",
  });

  const payload = (await response.json().catch(() => null)) as ApiResponse<T> | null;
  if (!response.ok || !payload || !payload.success) {
    const code = payload?.error?.code || `HTTP_${response.status}`;
    const msg = payload?.error?.message || "Permintaan gagal diproses.";
    throw new ApiError(response.status, code, msg);
  }

  return payload.data;
}

export function getExportUrl(params: Record<string, string | number | undefined>): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== "") {
      q.set(k, String(v));
    }
  }
  const qs = q.toString();
  return `${API_BASE}/admin/mailboxes/export${qs ? `?${qs}` : ""}`;
}

