package config

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	Env                   string
	HTTPAddr              string
	TelegramBotToken      string
	TelegramAllowedUserID string
	OpenAIAPIKey          string
	OpenAIModel           string
}

func Load() Config {
	loadDotEnv(".env")

	return Config{
		Env:                   getenv("ROBE_ENV", "dev"),
		HTTPAddr:              getenv("ROBE_HTTP_ADDR", ":8080"),
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramAllowedUserID: os.Getenv("TELEGRAM_ALLOWED_USER_ID"),
		OpenAIAPIKey:          os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:           getenv("OPENAI_MODEL", "gpt-4.1-mini"),
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if key == "" {
			continue
		}

		_ = os.Setenv(key, value)
	}
}
