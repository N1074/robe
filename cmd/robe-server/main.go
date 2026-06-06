package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	calendaradapter "github.com/N1074/robe/internal/adapters/calendar"
	"github.com/N1074/robe/internal/adapters/llm"
	"github.com/N1074/robe/internal/adapters/telegram"
	"github.com/N1074/robe/internal/config"
	"github.com/N1074/robe/internal/core"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"env":    cfg.Env,
		})
	})

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: mux,
	}

	llmClient := llm.NewOllamaClient(cfg.LLMBaseURL, cfg.LLMModel, cfg.LLMNumPredict, cfg.LLMTemperature)
	var calendarClient core.Calendar
	if cfg.CalendarProvider == "google" {
		client, err := calendaradapter.NewGoogleCalendar(ctx, calendaradapter.GoogleConfig{
			CredentialsFile: cfg.CalendarCredentialsFile,
			TokenFile:       cfg.CalendarTokenFile,
			CalendarID:      cfg.CalendarID,
		})
		if err != nil {
			logger.Error("failed to configure google calendar", "error", err)
		} else {
			calendarClient = client
			logger.Info("google calendar configured", "calendar_id", cfg.CalendarID)
		}
	} else if cfg.CalendarProvider != "" {
		logger.Warn("unsupported calendar provider", "provider", cfg.CalendarProvider)
	}

	assistant := core.NewAssistant(llmClient, core.Status{
		Env:              cfg.Env,
		LLMProvider:      cfg.LLMProvider,
		LLMModel:         cfg.LLMModel,
		AccessRestricted: cfg.TelegramAllowedUserID != "",
		CalendarEnabled:  calendarClient != nil,
		Timezone:         cfg.CalendarTimezone,
	}, core.WithCalendar(calendarClient))

	if cfg.TelegramBotToken != "" {
		bot, err := telegram.New(cfg.TelegramBotToken, cfg.TelegramAllowedUserID, assistant.HandleText, logger)
		if err != nil {
			logger.Error("failed to create telegram bot", "error", err)
			os.Exit(1)
		}

		go func() {
			if err := bot.Start(ctx); err != nil && err != context.Canceled {
				logger.Error("telegram bot stopped", "error", err)
			}
		}()
	} else {
		logger.Warn("TELEGRAM_BOT_TOKEN is empty; telegram bot disabled")
	}

	go func() {
		logger.Info("starting robe-server", "addr", cfg.HTTPAddr, "env", cfg.Env)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	logger.Info("shutdown requested")

	if err := server.Shutdown(context.Background()); err != nil {
		logger.Error("server shutdown failed", "error", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
