package telegram

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type HandleTextFunc func(ctx context.Context, text string) (string, error)

type Bot struct {
	api           *tgbotapi.BotAPI
	allowedUserID string
	handleText    HandleTextFunc
	logger        *slog.Logger
}

func New(token string, allowedUserID string, handleText HandleTextFunc, logger *slog.Logger) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &Bot{
		api:           api,
		allowedUserID: strings.TrimSpace(allowedUserID),
		handleText:    handleText,
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
	if message.From == nil {
		b.logger.Warn("ignoring telegram message without sender")
		return
	}

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

	if b.handleText == nil {
		b.reply(message.Chat.ID, "Assistant is not configured.")
		return
	}

	text := strings.TrimSpace(message.Text)

	requestCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	answer, err := b.handleText(requestCtx, text)
	if err != nil {
		b.logger.Error("assistant handle text failed", "error", err)
		b.reply(message.Chat.ID, "LLM error: "+err.Error())
		return
	}

	b.reply(message.Chat.ID, answer)
}

func (b *Bot) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)

	if _, err := b.api.Send(msg); err != nil {
		b.logger.Error("failed to send telegram message", "error", err)
	}
}
