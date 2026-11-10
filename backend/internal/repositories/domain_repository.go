package repositories

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"baramail/backend/internal/models"

	"github.com/go-sql-driver/mysql"
)

var (
	ErrDuplicateEntry     = errors.New("duplicate entry")
	ErrDomainHasMailboxes = errors.New("domain masih memiliki mailbox")
)

type DomainRepository struct {
	db *sql.DB
}

func NewDomainRepository(db *sql.DB) *DomainRepository {
	return &DomainRepository{db: db}
}

func (r *DomainRepository) ListDomains(ctx context.Context) ([]models.Domain, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT d.id, d.name, d.status, COUNT(m.id) AS mailbox_count, d.created_at, d.updated_at
		FROM domains d
		LEFT JOIN mailboxes m ON m.domain_id = d.id
		GROUP BY d.id, d.name, d.status, d.created_at, d.updated_at
		ORDER BY d.name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Domain
	for rows.Next() {
		var d models.Domain
		if err := rows.Scan(&d.ID, &d.Name, &d.Status, &d.MailboxCount, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, rows.Err()
}

func (r *DomainRepository) GetByID(ctx context.Context, id uint64) (*models.Domain, error) {
	var d models.Domain
	err := r.db.QueryRowContext(ctx, `
		SELECT d.id, d.name, d.status, COUNT(m.id) AS mailbox_count, d.created_at, d.updated_at
		FROM domains d
		LEFT JOIN mailboxes m ON m.domain_id = d.id
		WHERE d.id = ?
		GROUP BY d.id, d.name, d.status, d.created_at, d.updated_at
	`, id).Scan(&d.ID, &d.Name, &d.Status, &d.MailboxCount, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DomainRepository) GetByName(ctx context.Context, name string) (*models.Domain, error) {
	var d models.Domain
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, status, created_at, updated_at
		FROM domains
		WHERE name = ?
	`, strings.ToLower(strings.TrimSpace(name))).Scan(&d.ID, &d.Name, &d.Status, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DomainRepository) CreateDomain(ctx context.Context, name string) (*models.Domain, error) {
	now := time.Now().UTC()
	normalized := strings.ToLower(strings.TrimSpace(name))
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO domains (name, status, created_at, updated_at)
		VALUES (?, 'active', ?, ?)
	`, normalized, now, now)
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
	return &models.Domain{
		ID:        uint64(id),
		Name:      normalized,
		Status:    models.DomainStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (r *DomainRepository) UpdateStatus(ctx context.Context, id uint64, status models.DomainStatus) (*models.Domain, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE domains SET status = ?, updated_at = ? WHERE id = ?
	`, status, now, id)
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

func isMySQLDuplicate(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
