package repositories

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"baramail/backend/internal/models"
)

var ErrDuplicateMessage = errors.New("duplicate message")

type MessageRepository struct {
	db *sql.DB
}

func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// StoreMessageWithQuota inserts a message and trims the oldest messages beyond maxPerMailbox in a single transaction.
func (r *MessageRepository) StoreMessageWithQuota(ctx context.Context, msg *models.Message, maxPerMailbox int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO messages (mailbox_id, dedupe_key, message_id, sender, recipient, subject, body_text, body_source, size_bytes, received_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.MailboxID, msg.DedupeKey, msg.MessageID, msg.Sender, msg.Recipient, msg.Subject, msg.BodyText, msg.BodySource, msg.SizeBytes, msg.ReceivedAt, now)
	if err != nil {
		if isMySQLDuplicate(err) {
			return ErrDuplicateMessage
		}
		return err
	}

	insertID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	msg.ID = uint64(insertID)
	msg.CreatedAt = now

	if maxPerMailbox > 0 {
		// Find the cutoff ID so we keep only the newest maxPerMailbox messages.
		var cutoffID uint64
		err := tx.QueryRowContext(ctx, `
			SELECT id FROM messages
			WHERE mailbox_id = ?
			ORDER BY id DESC
			LIMIT 1 OFFSET ?
		`, msg.MailboxID, maxPerMailbox).Scan(&cutoffID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && cutoffID > 0 {
			if _, err := tx.ExecContext(ctx, `
				DELETE FROM messages WHERE mailbox_id = ? AND id <= ?
			`, msg.MailboxID, cutoffID); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (r *MessageRepository) ListMessages(ctx context.Context, mailboxID uint64, limit int, beforeID, afterID uint64) ([]models.Message, error) {
	where := "mailbox_id = ?"
	args := []any{mailboxID}

	if afterID > 0 {
		where += " AND id > ?"
		args = append(args, afterID)
	} else if beforeID > 0 {
		where += " AND id < ?"
		args = append(args, beforeID)
	}
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, mailbox_id, message_id, sender, recipient, subject, LEFT(body_text, 240) AS snippet_raw, body_source, size_bytes, received_at, created_at
		FROM messages
		WHERE `+where+`
		ORDER BY id DESC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Message
	for rows.Next() {
		var m models.Message
		var msgID sql.NullString
		var snippetRaw string
		if err := rows.Scan(
			&m.ID, &m.MailboxID, &msgID, &m.Sender, &m.Recipient, &m.Subject, &snippetRaw, &m.BodySource, &m.SizeBytes, &m.ReceivedAt, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		if msgID.Valid {
			m.MessageID = &msgID.String
		}
		m.Snippet = buildSnippet(snippetRaw, 140)
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *MessageRepository) GetMessage(ctx context.Context, mailboxID, messageID uint64) (*models.Message, error) {
	var m models.Message
	var msgID sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT id, mailbox_id, message_id, sender, recipient, subject, body_text, body_source, size_bytes, received_at, created_at
		FROM messages
		WHERE mailbox_id = ? AND id = ?
	`, mailboxID, messageID).Scan(
		&m.ID, &m.MailboxID, &msgID, &m.Sender, &m.Recipient, &m.Subject, &m.BodyText, &m.BodySource, &m.SizeBytes, &m.ReceivedAt, &m.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if msgID.Valid {
		m.MessageID = &msgID.String
	}
	return &m, nil
}

func (r *MessageRepository) DeleteExpiredMessagesByTTL(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM messages WHERE received_at < ? LIMIT 5000`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *MessageRepository) DeleteMessagesOfExpiredMailboxes(ctx context.Context, expiredBefore time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		DELETE msg FROM messages msg
		INNER JOIN mailboxes mb ON mb.id = msg.mailbox_id
		WHERE mb.expires_at < ?
	`, expiredBefore)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *MessageRepository) IncrementWebhookMetric(ctx context.Context, key string) {
	now := time.Now().UTC()
	_, _ = r.db.ExecContext(ctx, `
		INSERT INTO webhook_metrics (metric_key, count, updated_at)
		VALUES (?, 1, ?)
		ON DUPLICATE KEY UPDATE count = count + 1, updated_at = VALUES(updated_at)
	`, key, now)
}

func buildSnippet(s string, maxRunes int) string {
	cleaned := strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(cleaned) <= maxRunes {
		return cleaned
	}
	runes := []rune(cleaned)
	return string(runes[:maxRunes]) + "..."
}

