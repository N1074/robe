package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	calendaradapter "github.com/N1074/robe/internal/adapters/calendar"
	gmailadapter "github.com/N1074/robe/internal/adapters/gmail"
	"github.com/N1074/robe/internal/adapters/llm"
	"github.com/N1074/robe/internal/adapters/storage"
	"github.com/N1074/robe/internal/adapters/stt"
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

	llmClient := llm.NewOllamaClient(cfg.LLMBaseURL, cfg.LLMModel, cfg.LLMNumPredict, cfg.LLMTemperature, cfg.PromptsDir)
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

	var emailClient core.Email
	if cfg.EmailProvider == "gmail" {
		client, err := gmailadapter.NewGoogleGmail(ctx, gmailadapter.GoogleConfig{
			CredentialsFile: cfg.GmailCredentialsFile,
			TokenFile:       cfg.GmailTokenFile,
			UserID:          cfg.GmailUserID,
		})
		if err != nil {
			logger.Error("failed to configure gmail", "error", err)
		} else {
			emailClient = client
			logger.Info("gmail configured", "user_id", cfg.GmailUserID)
		}
	} else if cfg.EmailProvider != "" {
		logger.Warn("unsupported email provider", "provider", cfg.EmailProvider)
	}

	var transcribe telegram.TranscribeFunc
	if cfg.STTProvider == "command" {
		transcriber := stt.NewCommandTranscriber(cfg.STTCommand, cfg.STTArgs, time.Duration(cfg.STTTimeoutSeconds)*time.Second)
		transcribe = transcriber.Transcribe
		logger.Info("stt command configured")
	} else if cfg.STTProvider != "" {
		logger.Warn("unsupported stt provider", "provider", cfg.STTProvider)
	}

	var memoryStore core.MemoryStore
	var auditLogger core.AuditLogger
	var contactDirectory core.ContactDirectory
	var emailAccountStore core.EmailAccountStore
	if cfg.MemoryProvider == "postgres" {
		store, err := storage.NewPostgresMemoryStoreWithOptions(ctx, cfg.DatabaseURL, storage.Options{
			ContactEncryptionKey:          cfg.ContactEncryptionKey,
			PreviousContactEncryptionKeys: cfg.ContactPreviousEncryptionKeys,
		})
		if err != nil {
			logger.Error("failed to configure memory store", "error", err)
		} else {
			defer store.Close()
			memoryStore = store
			auditLogger = store
			contactDirectory = store
			emailAccountStore = store
			if len(cfg.ContactPreviousEncryptionKeys) > 0 && cfg.ContactEncryptionKey != "" {
				if err := store.RotateContactEncryption(ctx); err != nil {
					logger.Error("failed to rotate contact encryption", "error", err)
				} else {
					logger.Info("contact encryption rotation completed")
				}
			}
			logger.Info("postgres memory configured")
		}
	} else if cfg.MemoryProvider != "" {
		logger.Warn("unsupported memory provider", "provider", cfg.MemoryProvider)
	}

	if emailAccountStore != nil && cfg.EmailProvider == "gmail" {
		account, err := emailAccountStore.UpsertEmailAccount(ctx, core.EmailAccount{
			Provider:          core.EmailAccountProviderGmail,
			UserID:            cfg.GmailUserID,
			CredentialsFile:   cfg.GmailCredentialsFile,
			TokenFile:         cfg.GmailTokenFile,
			Status:            core.EmailAccountStatusActive,
			AutoReviewEnabled: cfg.EmailReviewEnabled,
		})
		if err != nil {
			logger.Error("failed to upsert gmail email account", "error", err)
		} else {
			logger.Info("gmail email account registered", "account_key", account.AccountKey, "autoreview_enabled", account.AutoReviewEnabled)
		}
	}

	var embedder core.Embedder
	if cfg.EmbeddingProvider == "ollama" {
		embedder = llm.NewOllamaEmbedder(cfg.EmbeddingBaseURL, cfg.EmbeddingModel)
		logger.Info("ollama embeddings configured", "model", cfg.EmbeddingModel)
	} else if cfg.EmbeddingProvider != "" {
		logger.Warn("unsupported embedding provider", "provider", cfg.EmbeddingProvider)
	}

	assistant := core.NewAssistant(llmClient, core.Status{
		Env:               cfg.Env,
		LLMProvider:       cfg.LLMProvider,
		LLMModel:          cfg.LLMModel,
		AccessRestricted:  cfg.TelegramAllowedUserID != "",
		CalendarEnabled:   calendarClient != nil,
		EmailEnabled:      emailClient != nil,
		VoiceEnabled:      transcribe != nil,
		MemoryEnabled:     memoryStore != nil,
		EmbeddingsEnabled: embedder != nil,
		Timezone:          cfg.CalendarTimezone,
	}, core.WithCalendar(calendarClient), core.WithEmail(emailClient), core.WithMemory(memoryStore), core.WithEmbedder(embedder), core.WithProjectAliases(cfg.ProjectAliases), core.WithAuditLogger(auditLogger), core.WithContactDirectory(contactDirectory))

	if cfg.TelegramBotToken != "" {
		bot, err := telegram.New(cfg.TelegramBotToken, cfg.TelegramAllowedUserID, assistant.HandleText, transcribe, logger)
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

	if cfg.EmailReviewEnabled && emailAccountStore != nil {
		startEmailReviewScheduler(ctx, logger, emailAccountStore, llmClient, auditLogger, contactDirectory, cfg.EmailReviewInterval, cfg.EmailReviewDryRun)
	} else if cfg.EmailReviewEnabled {
		logger.Warn("email review scheduler requested but email account store is not configured")
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

func startEmailReviewScheduler(ctx context.Context, logger *slog.Logger, accounts core.EmailAccountStore, llmClient *llm.OllamaClient, auditLogger core.AuditLogger, contactDirectory core.ContactDirectory, interval time.Duration, dryRun bool) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	run := func() {
		if err := runEmailReviewOnce(ctx, logger, accounts, llmClient, auditLogger, contactDirectory, dryRun); err != nil {
			logger.Error("email review run failed", "error", err)
		}
	}

	go func() {
		logger.Info("email review scheduler started", "interval", interval.String(), "dry_run", dryRun)
		run()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				logger.Info("email review scheduler stopped")
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

func runEmailReviewOnce(ctx context.Context, logger *slog.Logger, accounts core.EmailAccountStore, llmClient *llm.OllamaClient, auditLogger core.AuditLogger, contactDirectory core.ContactDirectory, dryRun bool) error {
	configuredAccounts, err := accounts.ListEmailAccounts(ctx, true)
	if err != nil {
		return err
	}
	for _, account := range configuredAccounts {
		if account.Provider != core.EmailAccountProviderGmail {
			logger.Warn("unsupported email account provider", "account_key", account.AccountKey, "provider", account.Provider)
			continue
		}
		if strings.TrimSpace(account.CredentialsFile) == "" || strings.TrimSpace(account.TokenFile) == "" {
			logger.Warn("email account missing oauth files", "account_key", account.AccountKey)
			continue
		}
		client, err := gmailadapter.NewGoogleGmail(ctx, gmailadapter.GoogleConfig{
			CredentialsFile: account.CredentialsFile,
			TokenFile:       account.TokenFile,
			UserID:          account.UserID,
		})
		if err != nil {
			logger.Error("failed to configure scheduled gmail account", "account_key", account.AccountKey, "error", err)
			continue
		}
		assistant := core.NewAssistant(
			llmClient,
			core.Status{},
			core.WithEmail(client),
			core.WithAuditLogger(auditLogger),
			core.WithContactDirectory(contactDirectory),
		)
		results, err := assistant.ReviewUnreadEmails(ctx, core.EmailReviewOptions{DryRun: dryRun, Limit: 10})
		if err != nil {
			logger.Error("scheduled email review failed", "account_key", account.AccountKey, "error", err)
			continue
		}
		logger.Info("scheduled email review completed", "account_key", account.AccountKey, "messages", len(results), "dry_run", dryRun)
	}
	return nil
}
