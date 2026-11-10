package services

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"baramail/backend/internal/config"
	"baramail/backend/internal/mailparse"
	"baramail/backend/internal/models"
	"baramail/backend/internal/repositories"
	"baramail/backend/internal/validators"
)

var (
	ErrWebhookUnauthorized = errors.New("webhook secret tidak valid")
	ErrPayloadTooLarge     = errors.New("ukuran email melebihi batas")
	ErrInvalidMIME         = errors.New("header atau format MIME tidak valid")
)

type IngestInput struct {
	Secret        string
	EnvelopeTo    string
	EnvelopeFrom  string
	ReceivedAtRaw string
	RawMIME       []byte
}

type IngestResult struct {
	Status    string `json:"status"`
	MessageID uint64 `json:"message_id,omitempty"`
}

type IngestionService struct {
	cfg         *config.Config
	mailboxRepo *repositories.MailboxRepository
	msgRepo     *repositories.MessageRepository
}

func NewIngestionService(cfg *config.Config, mailboxRepo *repositories.MailboxRepository, msgRepo *repositories.MessageRepository) *IngestionService {
	return &IngestionService{
		cfg:         cfg,
		mailboxRepo: mailboxRepo,
		msgRepo:     msgRepo,
	}
}

func (s *IngestionService) VerifySecret(provided string) bool {
	if provided == "" || s.cfg.WebhookSecret == "" {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(provided), []byte(s.cfg.WebhookSecret)) == 1 {
		return true
	}
	if s.cfg.WebhookSecretPrev != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(s.cfg.WebhookSecretPrev)) == 1 {
		return true
	}
	return false
}

func (s *IngestionService) IngestEmail(ctx context.Context, in IngestInput) (*IngestResult, error) {
	s.msgRepo.IncrementWebhookMetric(ctx, "received_total")

	if !s.VerifySecret(in.Secret) {
		s.msgRepo.IncrementWebhookMetric(ctx, "rejected_unauthorized")
		return nil, ErrWebhookUnauthorized
	}

	if int64(len(in.RawMIME)) > s.cfg.MaxEmailSizeBytes {
		s.msgRepo.IncrementWebhookMetric(ctx, "rejected_payload_too_large")
		return nil, ErrPayloadTooLarge
	}

	if len(in.RawMIME) == 0 || strings.TrimSpace(in.EnvelopeTo) == "" {
		s.msgRepo.IncrementWebhookMetric(ctx, "rejected_invalid_message")
		return nil, ErrInvalidMIME
	}

	_, _, recipient, err := validators.NormalizeEmail(in.EnvelopeTo)
	if err != nil {
		s.msgRepo.IncrementWebhookMetric(ctx, "rejected_mailbox_not_available")
		return nil, ErrMailboxNotAvailable
	}

	mb, err := s.mailboxRepo.GetByEmail(ctx, recipient)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			s.msgRepo.IncrementWebhookMetric(ctx, "rejected_mailbox_not_available")
			return nil, ErrMailboxNotAvailable
		}
		s.msgRepo.IncrementWebhookMetric(ctx, "error_internal")
		return nil, err
	}

	now := time.Now().UTC()
	if !mb.IsAccessible(now) {
		s.msgRepo.IncrementWebhookMetric(ctx, "rejected_mailbox_not_available")
		return nil, ErrMailboxNotAvailable
	}

	fallbackSender := strings.TrimSpace(in.EnvelopeFrom)
	if fallbackSender == "" {
		fallbackSender = "unknown"
	}

	parsed, err := mailparse.ParseRawMIME(in.RawMIME, fallbackSender)
	if err != nil {
		s.msgRepo.IncrementWebhookMetric(ctx, "rejected_invalid_message")
		return nil, ErrInvalidMIME
	}

	receivedAt := now
	if in.ReceivedAtRaw != "" {
		if t, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(in.ReceivedAtRaw)); parseErr == nil {
			receivedAt = t.UTC()
		}
	}

	sender := parsed.Sender
	if len(sender) > 320 {
		sender = sender[:320]
	}
	subject := parsed.Subject
	if len(subject) > 998 {
		subject = subject[:998]
	}

	msg := &models.Message{
		MailboxID:  mb.ID,
		DedupeKey:  mailparse.ComputeDedupeKey(in.RawMIME, recipient),
		MessageID:  parsed.MessageID,
		Sender:     sender,
		Recipient:  recipient,
		Subject:    subject,
		BodyText:   parsed.BodyText,
		BodySource: parsed.BodySource,
		SizeBytes:  uint32(len(in.RawMIME)),
		ReceivedAt: receivedAt,
	}

	err = s.msgRepo.StoreMessageWithQuota(ctx, msg, s.cfg.MaxMessagesPerMailbox)
	if err != nil {
		if errors.Is(err, repositories.ErrDuplicateMessage) {
			s.msgRepo.IncrementWebhookMetric(ctx, "duplicate_total")
			return &IngestResult{Status: "duplicate"}, nil
		}
		s.msgRepo.IncrementWebhookMetric(ctx, "error_internal")
		return nil, err
	}

	s.msgRepo.IncrementWebhookMetric(ctx, "stored_total")
	return &IngestResult{
		Status:    "stored",
		MessageID: msg.ID,
	}, nil
}
