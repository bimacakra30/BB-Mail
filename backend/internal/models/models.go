package models

import (
	"encoding/json"
	"time"
)

type DomainStatus string

const (
	DomainStatusActive  DomainStatus = "active"
	DomainStatusRetired DomainStatus = "retired"
)

type MailboxStatus string

const (
	MailboxStatusActive   MailboxStatus = "active"
	MailboxStatusDisabled MailboxStatus = "disabled"
	MailboxStatusExpired  MailboxStatus = "expired"
)

type CreatedVia string

const (
	CreatedViaAdmin       CreatedVia = "admin"
	CreatedViaBulk        CreatedVia = "bulk"
	CreatedViaSelfService CreatedVia = "self_service"
)

type BodySource string

const (
	BodySourceText          BodySource = "text"
	BodySourceHTMLConverted BodySource = "html_converted"
	BodySourceEmpty         BodySource = "empty"
)

type AdminUser struct {
	ID               uint64     `json:"id"`
	Username         string     `json:"username"`
	Email            *string    `json:"email,omitempty"`
	PasswordHash     string     `json:"-"`
	FailedLoginCount uint       `json:"-"`
	LockedUntil      *time.Time `json:"-"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type AdminSession struct {
	ID          string    `json:"id"`
	AdminUserID uint64    `json:"admin_user_id"`
	CSRFToken   string    `json:"csrf_token"`
	IP          string    `json:"ip"`
	UserAgent   string    `json:"user_agent"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type Domain struct {
	ID           uint64       `json:"id"`
	Name         string       `json:"name"`
	Status       DomainStatus `json:"status"`
	MailboxCount int          `json:"mailbox_count"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type Mailbox struct {
	ID              uint64        `json:"id"`
	DomainID        uint64        `json:"domain_id"`
	DomainName      string        `json:"domain_name,omitempty"`
	DomainStatus    DomainStatus  `json:"domain_status,omitempty"`
	LocalPart       string        `json:"local_part"`
	Email           string        `json:"email"`
	Status          MailboxStatus `json:"status"`
	EffectiveStatus MailboxStatus `json:"effective_status"`
	ExpiresAt       time.Time     `json:"expires_at"`
	OwnerRef        *string       `json:"owner_ref"`
	Note            *string       `json:"note"`
	CreatedVia      CreatedVia    `json:"created_via"`
	AccessCodeHash  *string       `json:"-"`
	MessageCount    int           `json:"message_count"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

func (m *Mailbox) ComputeEffectiveStatus(now time.Time) MailboxStatus {
	if m.Status == MailboxStatusDisabled {
		return MailboxStatusDisabled
	}
	if !now.Before(m.ExpiresAt) {
		return MailboxStatusExpired
	}
	return MailboxStatusActive
}

func (m *Mailbox) IsAccessible(now time.Time) bool {
	if m.DomainStatus != "" && m.DomainStatus != DomainStatusActive {
		return false
	}
	return m.ComputeEffectiveStatus(now) == MailboxStatusActive
}

type Message struct {
	ID         uint64     `json:"id"`
	MailboxID  uint64     `json:"mailbox_id"`
	DedupeKey  []byte     `json:"-"`
	MessageID  *string    `json:"message_id,omitempty"`
	Sender     string     `json:"sender"`
	Recipient  string     `json:"recipient"`
	Subject    string     `json:"subject"`
	BodyText   string     `json:"body_text,omitempty"`
	Snippet    string     `json:"snippet,omitempty"`
	BodySource BodySource `json:"body_source"`
	SizeBytes  uint32     `json:"size_bytes"`
	ReceivedAt time.Time  `json:"received_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

type AuditLog struct {
	ID          uint64          `json:"id"`
	AdminUserID *uint64         `json:"admin_user_id"`
	AdminName   *string         `json:"admin_username,omitempty"`
	Action      string          `json:"action"`
	TargetType  *string         `json:"target_type,omitempty"`
	TargetID    *uint64         `json:"target_id,omitempty"`
	Meta        json.RawMessage `json:"meta,omitempty"`
	IP          string          `json:"ip"`
	CreatedAt   time.Time       `json:"created_at"`
}

type StatsSummary struct {
	TotalMailboxes    int               `json:"total_mailboxes"`
	ActiveMailboxes   int               `json:"active_mailboxes"`
	DisabledMailboxes int               `json:"disabled_mailboxes"`
	ExpiredMailboxes  int               `json:"expired_mailboxes"`
	TotalMessages     int               `json:"total_messages"`
	MessagesToday     int               `json:"messages_today"`
	MailboxesByDomain map[string]int    `json:"mailboxes_by_domain"`
	WebhookMetrics    map[string]uint64 `json:"webhook_metrics"`
}

