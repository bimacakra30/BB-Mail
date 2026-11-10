CREATE TABLE IF NOT EXISTS admin_users (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  username VARCHAR(100) NOT NULL,
  email VARCHAR(255) NULL,
  password_hash VARCHAR(255) NOT NULL,
  failed_login_count INT UNSIGNED NOT NULL DEFAULT 0,
  locked_until DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_admin_users_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS admin_sessions (
  id VARCHAR(64) NOT NULL,
  admin_user_id BIGINT UNSIGNED NOT NULL,
  csrf_token VARCHAR(64) NOT NULL,
  ip VARCHAR(45) NOT NULL,
  user_agent VARCHAR(255) NOT NULL DEFAULT '',
  expires_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_admin_sessions_user (admin_user_id),
  KEY idx_admin_sessions_expires (expires_at),
  CONSTRAINT fk_admin_sessions_user FOREIGN KEY (admin_user_id)
    REFERENCES admin_users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS domains (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(190) NOT NULL,
  status ENUM('active','retired') NOT NULL DEFAULT 'active',
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_domains_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS mailboxes (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  domain_id BIGINT UNSIGNED NOT NULL,
  local_part VARCHAR(64) NOT NULL,
  email VARCHAR(255) NOT NULL,
  status ENUM('active','disabled') NOT NULL DEFAULT 'active',
  expires_at DATETIME(3) NOT NULL,
  owner_ref VARCHAR(100) NULL,
  note VARCHAR(255) NULL,
  created_via ENUM('admin','bulk','self_service') NOT NULL DEFAULT 'admin',
  access_code_hash VARCHAR(255) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_mailboxes_email (email),
  KEY idx_mailboxes_domain (domain_id),
  KEY idx_mailboxes_owner (owner_ref),
  KEY idx_mailboxes_status_expires (status, expires_at),
  CONSTRAINT fk_mailboxes_domain FOREIGN KEY (domain_id) REFERENCES domains (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS messages (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  mailbox_id BIGINT UNSIGNED NOT NULL,
  dedupe_key BINARY(32) NOT NULL,
  message_id VARCHAR(998) NULL,
  sender VARCHAR(320) NOT NULL,
  recipient VARCHAR(320) NOT NULL,
  subject VARCHAR(998) NOT NULL DEFAULT '',
  body_text MEDIUMTEXT NOT NULL,
  body_source ENUM('text','html_converted','empty') NOT NULL,
  size_bytes INT UNSIGNED NOT NULL,
  received_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_messages_dedupe (dedupe_key),
  KEY idx_messages_mailbox_id (mailbox_id, id),
  KEY idx_messages_received_at (received_at),
  CONSTRAINT fk_messages_mailbox FOREIGN KEY (mailbox_id)
    REFERENCES mailboxes (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS admin_audit_logs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  admin_user_id BIGINT UNSIGNED NULL,
  action VARCHAR(60) NOT NULL,
  target_type VARCHAR(60) NULL,
  target_id BIGINT UNSIGNED NULL,
  meta JSON NULL,
  ip VARCHAR(45) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_audit_logs_created_at (created_at),
  KEY idx_audit_logs_action (action),
  KEY idx_audit_logs_admin_user (admin_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS webhook_metrics (
  metric_key VARCHAR(64) NOT NULL,
  count BIGINT UNSIGNED NOT NULL DEFAULT 0,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (metric_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

