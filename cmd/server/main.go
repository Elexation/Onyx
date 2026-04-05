package main

import (
	"log/slog"
	"net/http"
	"os"

	server "github.com/Elexation/onyx/internal/port/http"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	port := env("ONYX_PORT", "8080")

	router := server.NewRouter(logger)

	slog.Info("starting server", "port", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
