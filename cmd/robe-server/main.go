package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/N1074/robe/internal/config"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"env":    cfg.Env,
		})
	})

	logger.Info("starting robe-server", "addr", cfg.HTTPAddr, "env", cfg.Env)

	if err := http.ListenAndServe(cfg.HTTPAddr, mux); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
