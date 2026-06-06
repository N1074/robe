package telegram

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const telegramMaxMessageRunes = 3900

type HandleTextFunc func(ctx context.Context, text string) (string, error)
type TranscribeFunc func(ctx context.Context, audioPath string) (string, error)

type Bot struct {
	api           *tgbotapi.BotAPI
	allowedUserID string
	handleText    HandleTextFunc
	transcribe    TranscribeFunc
	logger        *slog.Logger
}

func New(token string, allowedUserID string, handleText HandleTextFunc, transcribe TranscribeFunc, logger *slog.Logger) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &Bot{
		api:           api,
		allowedUserID: strings.TrimSpace(allowedUserID),
		handleText:    handleText,
		transcribe:    transcribe,
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
	audioFileID, audioExt := messageAudioFile(message)

	requestCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	if audioFileID != "" {
		b.handleAudio(requestCtx, message.Chat.ID, audioFileID, audioExt)
		return
	}

	answer, err := b.handleText(requestCtx, text)
	if err != nil {
		b.logger.Error("assistant handle text failed", "error", err)
		b.reply(message.Chat.ID, "LLM error: "+err.Error())
		return
	}

	b.reply(message.Chat.ID, answer)
}

func (b *Bot) handleAudio(ctx context.Context, chatID int64, fileID string, ext string) {
	if b.transcribe == nil {
		b.reply(chatID, "Voice input is not configured.")
		return
	}

	path, err := b.downloadTelegramFile(ctx, fileID, ext)
	if err != nil {
		b.logger.Error("failed to download telegram audio", "error", err)
		b.reply(chatID, "Voice error: failed to download audio.")
		return
	}
	defer os.Remove(path)

	transcript, err := b.transcribe(ctx, path)
	if err != nil {
		b.logger.Error("stt transcription failed", "error", err)
		b.reply(chatID, "Voice error: "+err.Error())
		return
	}

	transcript = strings.TrimSpace(transcript)
	if transcript == "" {
		b.reply(chatID, "Voice error: empty transcript.")
		return
	}

	answer, err := b.handleText(ctx, transcript)
	if err != nil {
		b.logger.Error("assistant handle transcript failed", "error", err)
		b.reply(chatID, "LLM error: "+err.Error())
		return
	}

	b.reply(chatID, "Heard: "+transcript+"\n\n"+answer)
}

func (b *Bot) downloadTelegramFile(ctx context.Context, fileID string, ext string) (string, error) {
	url, err := b.api.GetFileDirectURL(fileID)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("telegram file download returned status %d", resp.StatusCode)
	}

	file, err := os.CreateTemp("", "robe-telegram-audio-*"+ext)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		os.Remove(file.Name())
		return "", err
	}

	return file.Name(), nil
}

func messageAudioFile(message *tgbotapi.Message) (string, string) {
	if message.Voice != nil {
		return message.Voice.FileID, ".oga"
	}

	if message.Audio != nil {
		ext := filepath.Ext(message.Audio.FileName)
		if ext == "" {
			ext = ".audio"
		}
		return message.Audio.FileID, ext
	}

	return "", ""
}

func (b *Bot) reply(chatID int64, text string) {
	for _, chunk := range splitTelegramMessage(text) {
		msg := tgbotapi.NewMessage(chatID, chunk)

		if _, err := b.api.Send(msg); err != nil {
			b.logger.Error("failed to send telegram message", "error", err)
		}
	}
}

func splitTelegramMessage(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{""}
	}

	if utf8.RuneCountInString(text) <= telegramMaxMessageRunes {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for utf8.RuneCountInString(remaining) > telegramMaxMessageRunes {
		cut := cutTelegramChunk(remaining)
		chunks = append(chunks, strings.TrimSpace(remaining[:cut]))
		remaining = strings.TrimSpace(remaining[cut:])
	}

	if remaining != "" {
		chunks = append(chunks, remaining)
	}

	return chunks
}

func cutTelegramChunk(text string) int {
	cut := 0
	count := 0
	for idx := range text {
		if count == telegramMaxMessageRunes {
			break
		}
		cut = idx
		count++
	}

	if cut <= 0 {
		return len(text)
	}

	candidate := text[:cut]
	if newline := strings.LastIndex(candidate, "\n"); newline > telegramMaxMessageRunes/2 {
		return newline
	}
	if space := strings.LastIndex(candidate, " "); space > telegramMaxMessageRunes/2 {
		return space
	}

	return cut
}
