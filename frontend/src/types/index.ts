export interface ApiResponse<T> {
  success: boolean;
  data: T;
  message?: string;
  error?: {
    code: string;
    message: string;
  };
}

export interface Domain {
  id: number;
  name: string;
  status: "active" | "retired";
  mailbox_count: number;
  created_at: string;
  updated_at: string;
}

export interface Mailbox {
  id: number;
  domain_id: number;
  domain_name?: string;
  domain_status?: "active" | "retired";
  local_part: string;
  email: string;
  status: "active" | "disabled";
  effective_status: "active" | "disabled" | "expired";
  expires_at: string;
  owner_ref: string | null;
  note: string | null;
  created_via: "admin" | "bulk" | "self_service";
  message_count: number;
  created_at: string;
  updated_at: string;
}

export interface MessageSummary {
  id: number;
  mailbox_id: number;
  message_id?: string;
  sender: string;
  recipient: string;
  subject: string;
  snippet: string;
  body_source: "text" | "html_converted" | "empty";
  size_bytes: number;
  received_at: string;
  created_at: string;
}

export interface MessageDetail extends MessageSummary {
  body_text: string;
}

export interface InboxAccessData {
  email: string;
  expires_at: string;
  access_token: string;
  token_expires_at: string;
  poll_interval_seconds: number;
}

export interface StatsSummary {
  total_mailboxes: number;
  active_mailboxes: number;
  disabled_mailboxes: number;
  expired_mailboxes: number;
  total_messages: number;
  messages_today: number;
  mailboxes_by_domain: Record<string, number>;
  webhook_metrics: Record<string, number>;
}

export interface AuditLog {
  id: number;
  admin_user_id: number | null;
  admin_username?: string;
  action: string;
  target_type?: string;
  target_id?: number;
  meta?: Record<string, unknown>;
  ip: string;
  created_at: string;
}

export interface BulkItemResult {
  input?: string;
  email?: string;
  status: "created" | "skipped_duplicate" | "invalid";
  reason?: string;
  mailbox_id?: number;
}

export interface BulkCreateReport {
  total_requested: number;
  created_count: number;
  skipped_count: number;
  invalid_count: number;
  items: BulkItemResult[];
}

