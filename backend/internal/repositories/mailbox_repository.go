package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"baramail/backend/internal/models"
)

type MailboxFilter struct {
	Search   string
	Status   string
	DomainID uint64
	OwnerRef string
	Limit    int
	Offset   int
}

type MailboxRepository struct {
	db *sql.DB
}

func NewMailboxRepository(db *sql.DB) *MailboxRepository {
	return &MailboxRepository{db: db}
}

func (r *MailboxRepository) CreateMailbox(ctx context.Context, m *models.Mailbox) (*models.Mailbox, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO mailboxes (domain_id, local_part, email, status, expires_at, owner_ref, note, created_via, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, m.DomainID, m.LocalPart, m.Email, m.Status, m.ExpiresAt, m.OwnerRef, m.Note, m.CreatedVia, now, now)
	if err != nil {
		if isMySQLDuplicate(err) {
			return nil, ErrDuplicateEntry
		}
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	m.ID = uint64(id)
	m.CreatedAt = now
	m.UpdatedAt = now
	m.EffectiveStatus = m.ComputeEffectiveStatus(now)
	return m, nil
}

func (r *MailboxRepository) GetByID(ctx context.Context, id uint64) (*models.Mailbox, error) {
	var m models.Mailbox
	var ownerRef, note sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT m.id, m.domain_id, d.name, d.status, m.local_part, m.email, m.status, m.expires_at,
		       m.owner_ref, m.note, m.created_via,
		       (SELECT COUNT(1) FROM messages msg WHERE msg.mailbox_id = m.id) AS message_count,
		       m.created_at, m.updated_at
		FROM mailboxes m
		INNER JOIN domains d ON d.id = m.domain_id
		WHERE m.id = ?
	`, id).Scan(
		&m.ID, &m.DomainID, &m.DomainName, &m.DomainStatus, &m.LocalPart, &m.Email, &m.Status, &m.ExpiresAt,
		&ownerRef, &note, &m.CreatedVia, &m.MessageCount, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if ownerRef.Valid {
		m.OwnerRef = &ownerRef.String
	}
	if note.Valid {
		m.Note = &note.String
	}
	m.EffectiveStatus = m.ComputeEffectiveStatus(time.Now().UTC())
	return &m, nil
}

func (r *MailboxRepository) GetByEmail(ctx context.Context, email string) (*models.Mailbox, error) {
	var m models.Mailbox
	var ownerRef, note sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT m.id, m.domain_id, d.name, d.status, m.local_part, m.email, m.status, m.expires_at,
		       m.owner_ref, m.note, m.created_via, m.created_at, m.updated_at
		FROM mailboxes m
		INNER JOIN domains d ON d.id = m.domain_id
		WHERE m.email = ?
	`, strings.ToLower(strings.TrimSpace(email))).Scan(
		&m.ID, &m.DomainID, &m.DomainName, &m.DomainStatus, &m.LocalPart, &m.Email, &m.Status, &m.ExpiresAt,
		&ownerRef, &note, &m.CreatedVia, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if ownerRef.Valid {
		m.OwnerRef = &ownerRef.String
	}
	if note.Valid {
		m.Note = &note.String
	}
	m.EffectiveStatus = m.ComputeEffectiveStatus(time.Now().UTC())
	return &m, nil
}

func (r *MailboxRepository) CountByOwnerRef(ctx context.Context, ownerRef string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM mailboxes WHERE owner_ref = ?`, ownerRef).Scan(&count)
	return count, err
}

func (r *MailboxRepository) ListMailboxes(ctx context.Context, f MailboxFilter) ([]models.Mailbox, int, error) {
	now := time.Now().UTC()
	where, args := buildMailboxWhere(f, now)

	var total int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM mailboxes m
		INNER JOIN domains d ON d.id = m.domain_id
		WHERE `+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT m.id, m.domain_id, d.name, d.status, m.local_part, m.email, m.status, m.expires_at,
		       m.owner_ref, m.note, m.created_via,
		       (SELECT COUNT(1) FROM messages msg WHERE msg.mailbox_id = m.id) AS message_count,
		       m.created_at, m.updated_at
		FROM mailboxes m
		INNER JOIN domains d ON d.id = m.domain_id
		WHERE ` + where + `
		ORDER BY m.id DESC
		LIMIT ? OFFSET ?
	`
	queryArgs := append(append([]any{}, args...), f.Limit, f.Offset)
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []models.Mailbox
	for rows.Next() {
		var m models.Mailbox
		var ownerRef, note sql.NullString
		if err := rows.Scan(
			&m.ID, &m.DomainID, &m.DomainName, &m.DomainStatus, &m.LocalPart, &m.Email, &m.Status, &m.ExpiresAt,
			&ownerRef, &note, &m.CreatedVia, &m.MessageCount, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		if ownerRef.Valid {
			m.OwnerRef = &ownerRef.String
		}
		if note.Valid {
			m.Note = &note.String
		}
		m.EffectiveStatus = m.ComputeEffectiveStatus(now)
		list = append(list, m)
	}
	return list, total, rows.Err()
}

func (r *MailboxRepository) ExportMailboxes(ctx context.Context, f MailboxFilter) ([]models.Mailbox, error) {
	now := time.Now().UTC()
	where, args := buildMailboxWhere(f, now)

	query := `
		SELECT m.id, m.domain_id, d.name, d.status, m.local_part, m.email, m.status, m.expires_at,
		       m.owner_ref, m.note, m.created_via, m.created_at, m.updated_at
		FROM mailboxes m
		INNER JOIN domains d ON d.id = m.domain_id
		WHERE ` + where + `
		ORDER BY m.id DESC
		LIMIT 10000
	`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Mailbox
	for rows.Next() {
		var m models.Mailbox
		var ownerRef, note sql.NullString
		if err := rows.Scan(
			&m.ID, &m.DomainID, &m.DomainName, &m.DomainStatus, &m.LocalPart, &m.Email, &m.Status, &m.ExpiresAt,
			&ownerRef, &note, &m.CreatedVia, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if ownerRef.Valid {
			m.OwnerRef = &ownerRef.String
		}
		if note.Valid {
			m.Note = &note.String
		}
		m.EffectiveStatus = m.ComputeEffectiveStatus(now)
		list = append(list, m)
	}
	return list, rows.Err()
}

func buildMailboxWhere(f MailboxFilter, now time.Time) (string, []any) {
	var clauses []string
	var args []any
	clauses = append(clauses, "1=1")

	if f.Search != "" {
		clauses = append(clauses, "m.email LIKE ?")
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(f.Search))+"%")
	}
	if f.DomainID > 0 {
		clauses = append(clauses, "m.domain_id = ?")
		args = append(args, f.DomainID)
	}
	if f.OwnerRef != "" {
		clauses = append(clauses, "m.owner_ref = ?")
		args = append(args, strings.TrimSpace(f.OwnerRef))
	}
	switch f.Status {
	case "active":
		clauses = append(clauses, "m.status = 'active' AND m.expires_at > ?")
		args = append(args, now)
	case "disabled":
		clauses = append(clauses, "m.status = 'disabled'")
	case "expired":
		clauses = append(clauses, "m.status = 'active' AND m.expires_at <= ?")
		args = append(args, now)
	}
	return strings.Join(clauses, " AND "), args
}

func (r *MailboxRepository) UpdateMailbox(ctx context.Context, id uint64, status models.MailboxStatus, expiresAt time.Time, ownerRef, note *string) (*models.Mailbox, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE mailboxes
		SET status = ?, expires_at = ?, owner_ref = ?, note = ?, updated_at = ?
		WHERE id = ?
	`, status, expiresAt, ownerRef, note, now, id)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, ErrNotFound
	}
	return r.GetByID(ctx, id)
}

func (r *MailboxRepository) DeleteMailbox(ctx context.Context, id uint64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM mailboxes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MailboxRepository) BulkAction(ctx context.Context, action string, ids []uint64, ownerRef string, domainID uint64, extendDays int) (int64, error) {
	now := time.Now().UTC()
	var clauses []string
	var whereArgs []any

	if len(ids) > 0 {
		placeholders := make([]string, len(ids))
		for i, id := range ids {
			placeholders[i] = "?"
			whereArgs = append(whereArgs, id)
		}
		clauses = append(clauses, fmt.Sprintf("id IN (%s)", strings.Join(placeholders, ",")))
	}
	if ownerRef != "" {
		clauses = append(clauses, "owner_ref = ?")
		whereArgs = append(whereArgs, ownerRef)
	}
	if domainID > 0 {
		clauses = append(clauses, "domain_id = ?")
		whereArgs = append(whereArgs, domainID)
	}
	if len(clauses) == 0 {
		return 0, errors.New("filter atau daftar ID wajib diisi untuk aksi massal")
	}

	where := strings.Join(clauses, " AND ")
	var query string
	var args []any

	switch action {
	case "disable":
		query = "UPDATE mailboxes SET status = 'disabled', updated_at = ? WHERE " + where
		args = append([]any{now}, whereArgs...)
	case "enable":
		// FR-006: only re-enable mailboxes that have not yet expired.
		query = "UPDATE mailboxes SET status = 'active', updated_at = ? WHERE expires_at > ? AND " + where
		args = append([]any{now, now}, whereArgs...)
	case "extend":
		query = "UPDATE mailboxes SET expires_at = DATE_ADD(GREATEST(expires_at, ?), INTERVAL ? DAY), updated_at = ? WHERE " + where
		args = append([]any{now, extendDays, now}, whereArgs...)
	case "delete":
		query = "DELETE FROM mailboxes WHERE " + where
		args = whereArgs
	default:
		return 0, errors.New("aksi massal tidak dikenal")
	}

	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *MailboxRepository) GetStats(ctx context.Context, now time.Time) (*models.StatsSummary, error) {
	summary := &models.StatsSummary{
		MailboxesByDomain: make(map[string]int),
		WebhookMetrics:    make(map[string]uint64),
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(1) AS total,
			COALESCE(SUM(CASE WHEN status = 'active' AND expires_at > ? THEN 1 ELSE 0 END), 0) AS active_count,
			COALESCE(SUM(CASE WHEN status = 'disabled' THEN 1 ELSE 0 END), 0) AS disabled_count,
			COALESCE(SUM(CASE WHEN status = 'active' AND expires_at <= ? THEN 1 ELSE 0 END), 0) AS expired_count
		FROM mailboxes
	`, now, now).Scan(&summary.TotalMailboxes, &summary.ActiveMailboxes, &summary.DisabledMailboxes, &summary.ExpiredMailboxes)
	if err != nil {
		return nil, err
	}

	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	err = r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(1) AS total_messages,
			COALESCE(SUM(CASE WHEN received_at >= ? THEN 1 ELSE 0 END), 0) AS messages_today
		FROM messages
	`, startOfDay).Scan(&summary.TotalMessages, &summary.MessagesToday)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT d.name, COUNT(m.id)
		FROM domains d
		LEFT JOIN mailboxes m ON m.domain_id = d.id
		GROUP BY d.id, d.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var cnt int
		if err := rows.Scan(&name, &cnt); err != nil {
			return nil, err
		}
		summary.MailboxesByDomain[name] = cnt
	}

	mRows, err := r.db.QueryContext(ctx, `SELECT metric_key, count FROM webhook_metrics`)
	if err == nil {
		defer mRows.Close()
		for mRows.Next() {
			var k string
			var c uint64
			if err := mRows.Scan(&k, &c); err == nil {
				summary.WebhookMetrics[k] = c
			}
		}
	}

	return summary, nil
}

