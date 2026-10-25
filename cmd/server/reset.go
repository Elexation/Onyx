package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Elexation/onyx/internal/adapter/database"
	"github.com/Elexation/onyx/internal/service"
)

// runResetPassword handles `onyx reset-password`; safe while the server runs (WAL, busy_timeout)
func runResetPassword() int {
	// Keep logs off stdout so the password line is the only output there
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	configDir := env("ONYX_CONFIG", "config")
	dbPath := filepath.Join(configDir, "onyx.db")
	if _, err := os.Stat(dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "no database at %s (set ONYX_CONFIG or run from the server's working directory): %v\n", dbPath, err)
		return 1
	}
	db, err := database.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database: %v\n", err)
		return 1
	}
	defer db.Close()

	settings := service.NewSettingsService(database.NewSettingsRepo(db))
	auth := service.NewAuthService(database.NewUserRepo(db), database.NewSessionRepo(db), settings)
	password, err := auth.ResetPassword()
	if err != nil {
		fmt.Fprintf(os.Stderr, "reset password: %v\n", err)
		return 1
	}
	fmt.Printf("New admin password: %s\n", password)
	fmt.Fprintln(os.Stderr, "All sessions were signed out; access tokens and share links still work. Log in and change this password in Settings.")
	return 0
}
