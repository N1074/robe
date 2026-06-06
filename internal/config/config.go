package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Env                   string
	HTTPAddr              string
	TelegramBotToken      string
	TelegramAllowedUserID string

	LLMProvider    string
	LLMBaseURL     string
	LLMModel       string
	LLMNumPredict  int
	LLMTemperature float64
}

func Load() Config {
	loadDotEnv(".env")

	return Config{
		Env:                   getenv("ROBE_ENV", "dev"),
		HTTPAddr:              getenv("ROBE_HTTP_ADDR", ":8080"),
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramAllowedUserID: os.Getenv("TELEGRAM_ALLOWED_USER_ID"),

		LLMProvider:    getenv("LLM_PROVIDER", "ollama"),
		LLMBaseURL:     getenv("LLM_BASE_URL", "http://localhost:11434"),
		LLMModel:       getenv("LLM_MODEL", "qwen3:14b"),
		LLMNumPredict:  getenvInt("LLM_NUM_PREDICT", 512),
		LLMTemperature: getenvFloat("LLM_TEMPERATURE", 0.2),
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getenvFloat(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}

	return parsed
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
