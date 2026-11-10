package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"baramail/backend/internal/models"
)

var ErrNotFound = errors.New("record not found")

type AdminRepository struct {
	db *sql.DB
}

func NewAdminRepository(db *sql.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) CreateAdmin(ctx context.Context, username string, email *string, passwordHash string) (*models.AdminUser, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO admin_users (username, email, password_hash, failed_login_count, created_at, updated_at)
		VALUES (?, ?, ?, 0, ?, ?)
	`, username, email, passwordHash, now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &models.AdminUser{
		ID:        uint64(id),
		Username:  username,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (r *AdminRepository) UpdatePasswordByUsername(ctx context.Context, username, passwordHash string) error {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE admin_users
		SET password_hash = ?, failed_login_count = 0, locked_until = NULL, updated_at = ?
		WHERE username = ?
	`, passwordHash, now, username)
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

func (r *AdminRepository) GetByUsername(ctx context.Context, username string) (*models.AdminUser, error) {
	var u models.AdminUser
	var email sql.NullString
	var lockedUntil sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, email, password_hash, failed_login_count, locked_until, created_at, updated_at
		FROM admin_users
		WHERE username = ?
	`, username).Scan(&u.ID, &u.Username, &email, &u.PasswordHash, &u.FailedLoginCount, &lockedUntil, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if email.Valid {
		u.Email = &email.String
	}
	if lockedUntil.Valid {
		u.LockedUntil = &lockedUntil.Time
	}
	return &u, nil
}

func (r *AdminRepository) GetByID(ctx context.Context, id uint64) (*models.AdminUser, error) {
	var u models.AdminUser
	var email sql.NullString
	var lockedUntil sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, email, password_hash, failed_login_count, locked_until, created_at, updated_at
		FROM admin_users
		WHERE id = ?
	`, id).Scan(&u.ID, &u.Username, &email, &u.PasswordHash, &u.FailedLoginCount, &lockedUntil, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if email.Valid {
		u.Email = &email.String
	}
	if lockedUntil.Valid {
		u.LockedUntil = &lockedUntil.Time
	}
	return &u, nil
}

func (r *AdminRepository) RecordFailedLogin(ctx context.Context, userID uint64, maxAttempts int, lockoutDuration time.Duration) error {
	now := time.Now().UTC()
	lockUntil := now.Add(lockoutDuration)
	_, err := r.db.ExecContext(ctx, `
		UPDATE admin_users
		SET failed_login_count = failed_login_count + 1,
		    locked_until = CASE WHEN failed_login_count + 1 >= ? THEN ? ELSE locked_until END,
		    updated_at = ?
		WHERE id = ?
	`, maxAttempts, lockUntil, now, userID)
	return err
}

func (r *AdminRepository) ResetFailedLogin(ctx context.Context, userID uint64) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE admin_users
		SET failed_login_count = 0, locked_until = NULL, updated_at = ?
		WHERE id = ?
	`, now, userID)
	return err
}

func (r *AdminRepository) CreateSession(ctx context.Context, s *models.AdminSession) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO admin_sessions (id, admin_user_id, csrf_token, ip, user_agent, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, s.ID, s.AdminUserID, s.CSRFToken, s.IP, s.UserAgent, s.ExpiresAt, s.CreatedAt)
	return err
}

func (r *AdminRepository) GetSession(ctx context.Context, sessionID string) (*models.AdminSession, error) {
	var s models.AdminSession
	err := r.db.QueryRowContext(ctx, `
		SELECT id, admin_user_id, csrf_token, ip, user_agent, expires_at, created_at
		FROM admin_sessions
		WHERE id = ?
	`, sessionID).Scan(&s.ID, &s.AdminUserID, &s.CSRFToken, &s.IP, &s.UserAgent, &s.ExpiresAt, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *AdminRepository) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE id = ?`, sessionID)
	return err
}

func (r *AdminRepository) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE expires_at <= ?`, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *AdminRepository) CreateAuditLog(ctx context.Context, adminUserID *uint64, action string, targetType *string, targetID *uint64, meta any, ip string) error {
	var metaJSON []byte
	if meta != nil {
		b, err := json.Marshal(meta)
		if err == nil {
			metaJSON = b
		}
	}
	var metaArg any
	if len(metaJSON) > 0 {
		metaArg = string(metaJSON)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO admin_audit_logs (admin_user_id, action, target_type, target_id, meta, ip, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, adminUserID, action, targetType, targetID, metaArg, ip, time.Now().UTC())
	return err
}

func (r *AdminRepository) ListAuditLogs(ctx context.Context, actionFilter string, limit, offset int) ([]models.AuditLog, int, error) {
	where := "1=1"
	var args []any
	if actionFilter != "" {
		where += " AND l.action = ?"
		args = append(args, actionFilter)
	}

	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM admin_audit_logs l WHERE "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT l.id, l.admin_user_id, u.username, l.action, l.target_type, l.target_id, l.meta, l.ip, l.created_at
		FROM admin_audit_logs l
		LEFT JOIN admin_users u ON u.id = l.admin_user_id
		WHERE ` + where + `
		ORDER BY l.id DESC
		LIMIT ? OFFSET ?
	`
	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var item models.AuditLog
		var adminID sql.NullInt64
		var adminName sql.NullString
		var targetType sql.NullString
		var targetID sql.NullInt64
		var metaBytes []byte

		if err := rows.Scan(&item.ID, &adminID, &adminName, &item.Action, &targetType, &targetID, &metaBytes, &item.IP, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		if adminID.Valid {
			v := uint64(adminID.Int64)
			item.AdminUserID = &v
		}
		if adminName.Valid {
			item.AdminName = &adminName.String
		}
		if targetType.Valid {
			item.TargetType = &targetType.String
		}
		if targetID.Valid {
			v := uint64(targetID.Int64)
			item.TargetID = &v
		}
		if len(metaBytes) > 0 {
			item.Meta = json.RawMessage(metaBytes)
		}
		logs = append(logs, item)
	}
	return logs, total, rows.Err()
}

