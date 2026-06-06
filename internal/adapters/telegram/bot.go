package telegram

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type AskFunc func(ctx context.Context, prompt string) (string, error)

type Bot struct {
	api           *tgbotapi.BotAPI
	allowedUserID string
	askFunc       AskFunc
	logger        *slog.Logger
}

func New(token string, allowedUserID string, askFunc AskFunc, logger *slog.Logger) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &Bot{
		api:           api,
		allowedUserID: strings.TrimSpace(allowedUserID),
		askFunc:       askFunc,
		logger:        logger,
	}, nil
}

func (b *Bot) Start(ctx context.Context) error {
	b.logger.Info("starting telegram bot", "username", b.api.Self.UserName)

	updates := b.api.GetUpdatesChan(tgbotapi.NewUpdate(0))

	for {
		select {
		case <-ctx.Done():
			b.logger.Info("stopping telegram bot")
			return ctx.Err()

		case update := <-updates:
			if update.Message == nil {
				continue
			}

			b.handleMessage(ctx, update.Message)
		}
	}
}

func (b *Bot) handleMessage(ctx context.Context, message *tgbotapi.Message) {
	userID := strconv.FormatInt(message.From.ID, 10)

	if b.allowedUserID == "" {
		b.logger.Warn(
			"telegram allowed user id is empty; set TELEGRAM_ALLOWED_USER_ID",
			"detected_user_id", userID,
			"username", message.From.UserName,
		)
	} else if userID != b.allowedUserID {
		b.logger.Warn(
			"ignoring telegram message from unauthorized user",
			"detected_user_id", userID,
			"username", message.From.UserName,
		)
		return
	}

	text := strings.TrimSpace(message.Text)

	switch {
	case text == "/ping":
		b.reply(message.Chat.ID, "pong")

	case text == "/start":
		b.reply(message.Chat.ID, "Robe v0.1 online. Try /ping or /ask <question>.")

	case text == "/help":
		b.reply(message.Chat.ID, "Commands:\n/ping\n/status\n/ask <question>")

	case text == "/status":
		b.reply(message.Chat.ID, "Robe v0.1 online.")

	case strings.HasPrefix(text, "/ask "):
		b.handleAsk(ctx, message.Chat.ID, strings.TrimSpace(strings.TrimPrefix(text, "/ask ")))

	default:
		b.reply(message.Chat.ID, "Unknown command. Try /help.")
	}
}

func (b *Bot) handleAsk(parentCtx context.Context, chatID int64, prompt string) {
	if b.askFunc == nil {
		b.reply(chatID, "LLM is not configured.")
		return
	}

	if prompt == "" {
		b.reply(chatID, "Usage: /ask <question>")
		return
	}

	b.reply(chatID, "Thinking...")

	ctx, cancel := context.WithTimeout(parentCtx, 90*time.Second)
	defer cancel()

	answer, err := b.askFunc(ctx, prompt)
	if err != nil {
		b.logger.Error("llm ask failed", "error", err)
		b.reply(chatID, "LLM error: "+err.Error())
		return
	}

	b.reply(chatID, answer)
}

func (b *Bot) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)

	if _, err := b.api.Send(msg); err != nil {
		b.logger.Error("failed to send telegram message", "error", err)
	}
}
