package telegram

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api           *tgbotapi.BotAPI
	allowedUserID string
	logger        *slog.Logger
}

func New(token string, allowedUserID string, logger *slog.Logger) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &Bot{
		api:           api,
		allowedUserID: strings.TrimSpace(allowedUserID),
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

			userID := strconv.FormatInt(update.Message.From.ID, 10)

			if b.allowedUserID == "" {
				b.logger.Warn(
					"telegram allowed user id is empty; set TELEGRAM_ALLOWED_USER_ID",
					"detected_user_id", userID,
					"username", update.Message.From.UserName,
				)
			} else if userID != b.allowedUserID {
				b.logger.Warn(
					"ignoring telegram message from unauthorized user",
					"detected_user_id", userID,
					"username", update.Message.From.UserName,
				)
				continue
			}

			text := strings.TrimSpace(update.Message.Text)

			switch text {
			case "/ping":
				b.reply(update.Message.Chat.ID, "pong")
			case "/start":
				b.reply(update.Message.Chat.ID, "Robe v0.1 online. Try /ping.")
			default:
				b.reply(update.Message.Chat.ID, "Robe v0.1 received: "+text)
			}
		}
	}
}

func (b *Bot) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)

	if _, err := b.api.Send(msg); err != nil {
		b.logger.Error("failed to send telegram message", "error", err)
	}
}
