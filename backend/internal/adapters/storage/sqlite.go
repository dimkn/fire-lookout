// Package storage is the driven adapter that persists feeds and their status items in a
// local SQLite database, accessed through stdlib database/sql with the pure-Go
// modernc.org/sqlite driver. Nothing outside this package touches the database.
package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens (creating if needed) the SQLite database at path, along with its parent
// directory. The pragmas are pinned here rather than left to callers because the schema
// depends on them: foreign_keys makes ON DELETE CASCADE / SET DEFAULT actually fire, and
// WAL is what lets the poller write while the API reads.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create data directory %q: %w", dir, err)
		}
	}

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database %q: %w", path, err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to sqlite database %q: %w", path, err)
	}
	return db, nil
}

// Migrate brings the schema up to date with the embedded migrations. It is idempotent,
// so callers can run it on every startup.
func Migrate(ctx context.Context, db *sql.DB) error {
	dir, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("locate embedded migrations: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectSQLite3, db, dir,
		goose.WithLogger(goose.NopLogger()))
	if err != nil {
		return fmt.Errorf("prepare migrations: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
