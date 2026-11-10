package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func RunMigrations(ctx context.Context, db *sql.DB, migrationsDir string) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(190) NOT NULL PRIMARY KEY,
			applied_at DATETIME(3) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		var exists int
		err := db.QueryRowContext(ctx, "SELECT COUNT(1) FROM schema_migrations WHERE version = ?", name).Scan(&exists)
		if err != nil {
			return err
		}
		if exists > 0 {
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			return err
		}

		statements := splitSQLStatements(string(content))
		for _, stmt := range statements {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("migration %s failed: %w", name, err)
			}
		}

		_, err = db.ExecContext(ctx, "INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", name, time.Now().UTC())
		if err != nil {
			return err
		}
	}

	return nil
}

func SeedDomains(ctx context.Context, db *sql.DB, domains []string) error {
	now := time.Now().UTC()
	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		_, err := db.ExecContext(ctx, `
			INSERT INTO domains (name, status, created_at, updated_at)
			VALUES (?, 'active', ?, ?)
			ON DUPLICATE KEY UPDATE id = id
		`, d, now, now)
		if err != nil {
			return err
		}
	}
	return nil
}

func splitSQLStatements(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ";") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

