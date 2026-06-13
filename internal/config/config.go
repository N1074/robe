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

	CalendarProvider        string
	CalendarID              string
	CalendarCredentialsFile string
	CalendarTokenFile       string
	CalendarTimezone        string

	EmailProvider        string
	GmailCredentialsFile string
	GmailTokenFile       string
	GmailUserID          string

	STTProvider       string
	STTCommand        string
	STTArgs           []string
	STTTimeoutSeconds int

	MemoryProvider                string
	DatabaseURL                   string
	ProjectAliases                map[string]string
	ContactEncryptionKey          string
	ContactPreviousEncryptionKeys []string

	EmbeddingProvider string
	EmbeddingBaseURL  string
	EmbeddingModel    string
	PromptsDir        string
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
		LLMNumPredict:  getenvInt("LLM_NUM_PREDICT", 1024),
		LLMTemperature: getenvFloat("LLM_TEMPERATURE", 0.2),

		CalendarProvider:        getenv("CALENDAR_PROVIDER", ""),
		CalendarID:              getenv("CALENDAR_ID", "primary"),
		CalendarCredentialsFile: os.Getenv("CALENDAR_CREDENTIALS_FILE"),
		CalendarTokenFile:       os.Getenv("CALENDAR_TOKEN_FILE"),
		CalendarTimezone:        getenv("CALENDAR_TIMEZONE", "Europe/Madrid"),

		EmailProvider:        getenv("EMAIL_PROVIDER", ""),
		GmailCredentialsFile: os.Getenv("GMAIL_CREDENTIALS_FILE"),
		GmailTokenFile:       os.Getenv("GMAIL_TOKEN_FILE"),
		GmailUserID:          getenv("GMAIL_USER_ID", "me"),

		STTProvider:       getenv("STT_PROVIDER", ""),
		STTCommand:        os.Getenv("STT_COMMAND"),
		STTArgs:           splitArgs(os.Getenv("STT_ARGS")),
		STTTimeoutSeconds: getenvInt("STT_TIMEOUT_SECONDS", 120),

		MemoryProvider:                getenv("MEMORY_PROVIDER", ""),
		DatabaseURL:                   os.Getenv("DATABASE_URL"),
		ProjectAliases:                parseProjectAliases(os.Getenv("MEMORY_PROJECT_ALIASES")),
		ContactEncryptionKey:          os.Getenv("CONTACT_ENCRYPTION_KEY"),
		ContactPreviousEncryptionKeys: splitCSV(os.Getenv("CONTACT_ENCRYPTION_PREVIOUS_KEYS")),

		EmbeddingProvider: getenv("EMBEDDING_PROVIDER", ""),
		EmbeddingBaseURL:  getenv("EMBEDDING_BASE_URL", getenv("LLM_BASE_URL", "http://localhost:11434")),
		EmbeddingModel:    getenv("EMBEDDING_MODEL", "nomic-embed-text"),
		PromptsDir:        os.Getenv("PROMPTS_DIR"),
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

func splitArgs(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	return strings.Fields(value)
}

func splitCSV(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseProjectAliases(value string) map[string]string {
	out := map[string]string{}
	value = strings.TrimSpace(value)
	if value == "" {
		return out
	}

	groups := strings.Split(value, ";")
	for _, group := range groups {
		project, aliases, ok := strings.Cut(group, "=")
		project = strings.TrimSpace(project)
		if !ok || project == "" {
			continue
		}

		out[project] = project
		for _, alias := range strings.Split(aliases, ",") {
			alias = strings.TrimSpace(alias)
			if alias != "" {
				out[alias] = project
			}
		}
	}
	return out
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
