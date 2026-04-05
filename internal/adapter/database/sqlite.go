package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/ncruces/go-sqlite3/driver"
)

func Open(dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	dsn := buildDSN(dbPath)
	slog.Info("opening database", "path", dbPath)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	slog.Info("database ready")
	return db, nil
}

func buildDSN(dbPath string) string {
	pragmas := []string{
		"busy_timeout(10000)",
		"journal_mode(WAL)",
		"synchronous(normal)",
		"temp_store(memory)",
		"mmap_size(30000000000)",
		"cache_size(-64000)",
		"foreign_keys(ON)",
	}

	var b strings.Builder
	b.WriteString("file:")
	b.WriteString(filepath.ToSlash(dbPath))
	for i, p := range pragmas {
		if i == 0 {
			b.WriteByte('?')
		} else {
			b.WriteByte('&')
		}
		b.WriteString("_pragma=")
		b.WriteString(p)
	}
	return b.String()
}
