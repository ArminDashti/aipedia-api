package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Open connects to SQLite. databaseURL is a file path or modernc DSN
// (e.g. file:./data/aipedia.db?_pragma=foreign_keys(1)).
func Open(databaseURL string) (*sql.DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	dsn := normalizeSQLiteDSN(databaseURL)
	if err := ensureSQLiteDir(dsn); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite handles concurrency poorly with multiple writers; keep the pool small.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign_keys: %w", err)
	}
	return db, nil
}

func normalizeSQLiteDSN(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "file:") || strings.Contains(raw, "mode=") {
		return raw
	}
	// Plain path → file DSN with sensible pragmas.
	return "file:" + filepath.ToSlash(raw) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
}

func ensureSQLiteDir(dsn string) error {
	path := dsn
	if i := strings.Index(path, "?"); i >= 0 {
		path = path[:i]
	}
	path = strings.TrimPrefix(path, "file:")
	path = strings.TrimPrefix(path, "//")
	if path == "" || path == ":memory:" || strings.HasPrefix(path, ":memory:") {
		return nil
	}
	dir := filepath.Dir(filepath.FromSlash(path))
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// Migrate runs SQL files in migrationsDir in lexical order.
func Migrate(db *sql.DB, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, name := range files {
		body, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			return err
		}
		for i, stmt := range splitSQLStatements(string(body)) {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("migration %s statement %d: %w", name, i+1, err)
			}
		}
	}
	return nil
}

func splitSQLStatements(script string) []string {
	parts := strings.Split(script, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		stmt := strings.TrimSpace(part)
		if stmt == "" {
			continue
		}
		// Skip comment-only chunks.
		lines := strings.Split(stmt, "\n")
		hasSQL := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "--") {
				continue
			}
			hasSQL = true
			break
		}
		if hasSQL {
			out = append(out, stmt)
		}
	}
	return out
}

// Ping checks database connectivity.
func Ping(ctx context.Context, db *sql.DB) error {
	return db.PingContext(ctx)
}
