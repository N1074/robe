package config

import "testing"

func TestGetenvReturnsFallbackWhenMissing(t *testing.T) {
	t.Setenv("ROBE_TEST_MISSING", "")

	got := getenv("ROBE_TEST_MISSING", "fallback")
	if got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestGetenvReturnsValueWhenPresent(t *testing.T) {
	t.Setenv("ROBE_TEST_VALUE", "configured")

	got := getenv("ROBE_TEST_VALUE", "fallback")
	if got != "configured" {
		t.Fatalf("expected configured value, got %q", got)
	}
}

func TestGetenvIntReturnsFallbackForInvalidValue(t *testing.T) {
	t.Setenv("ROBE_TEST_INT", "invalid")

	got := getenvInt("ROBE_TEST_INT", 512)
	if got != 512 {
		t.Fatalf("expected fallback 512, got %d", got)
	}
}

func TestGetenvIntReturnsParsedValue(t *testing.T) {
	t.Setenv("ROBE_TEST_INT", "1024")

	got := getenvInt("ROBE_TEST_INT", 512)
	if got != 1024 {
		t.Fatalf("expected 1024, got %d", got)
	}
}

func TestGetenvFloatReturnsFallbackForInvalidValue(t *testing.T) {
	t.Setenv("ROBE_TEST_FLOAT", "invalid")

	got := getenvFloat("ROBE_TEST_FLOAT", 0.2)
	if got != 0.2 {
		t.Fatalf("expected fallback 0.2, got %f", got)
	}
}

func TestGetenvFloatReturnsParsedValue(t *testing.T) {
	t.Setenv("ROBE_TEST_FLOAT", "0.7")

	got := getenvFloat("ROBE_TEST_FLOAT", 0.2)
	if got != 0.7 {
		t.Fatalf("expected 0.7, got %f", got)
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV("old-one, old-two ,,")
	if len(got) != 2 || got[0] != "old-one" || got[1] != "old-two" {
		t.Fatalf("unexpected csv split: %#v", got)
	}
}

func TestLoadCalendarDefaults(t *testing.T) {
	t.Setenv("CALENDAR_PROVIDER", "")
	t.Setenv("CALENDAR_ID", "")
	t.Setenv("CALENDAR_CREDENTIALS_FILE", "")
	t.Setenv("CALENDAR_TOKEN_FILE", "")
	t.Setenv("CALENDAR_TIMEZONE", "")

	cfg := Load()

	if cfg.CalendarProvider != "" {
		t.Fatalf("expected empty calendar provider, got %q", cfg.CalendarProvider)
	}
	if cfg.CalendarID != "primary" {
		t.Fatalf("expected primary calendar id, got %q", cfg.CalendarID)
	}
	if cfg.CalendarTimezone != "Europe/Madrid" {
		t.Fatalf("expected Europe/Madrid timezone, got %q", cfg.CalendarTimezone)
	}
}

func TestLoadEmailDefaults(t *testing.T) {
	t.Setenv("EMAIL_PROVIDER", "")
	t.Setenv("GMAIL_CREDENTIALS_FILE", "")
	t.Setenv("GMAIL_TOKEN_FILE", "")
	t.Setenv("GMAIL_USER_ID", "")

	cfg := Load()

	if cfg.EmailProvider != "" {
		t.Fatalf("expected empty email provider, got %q", cfg.EmailProvider)
	}
	if cfg.GmailCredentialsFile != "" {
		t.Fatalf("expected empty gmail credentials file, got %q", cfg.GmailCredentialsFile)
	}
	if cfg.GmailTokenFile != "" {
		t.Fatalf("expected empty gmail token file, got %q", cfg.GmailTokenFile)
	}
	if cfg.GmailUserID != "me" {
		t.Fatalf("expected default gmail user id me, got %q", cfg.GmailUserID)
	}
}

func TestSplitArgs(t *testing.T) {
	got := splitArgs("--language es --file={audio}")
	want := []string{"--language", "es", "--file={audio}"}

	if len(got) != len(want) {
		t.Fatalf("expected %d args, got %d", len(want), len(got))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestParseProjectAliases(t *testing.T) {
	got := parseProjectAliases("demo=veg,orchard;writing=novel")

	if got["demo"] != "demo" || got["orchard"] != "demo" || got["veg"] != "demo" || got["novel"] != "writing" {
		t.Fatalf("unexpected aliases: %#v", got)
	}
}

func TestLoadSTTDefaults(t *testing.T) {
	t.Setenv("STT_PROVIDER", "")
	t.Setenv("STT_COMMAND", "")
	t.Setenv("STT_ARGS", "")
	t.Setenv("STT_TIMEOUT_SECONDS", "")

	cfg := Load()

	if cfg.STTProvider != "" {
		t.Fatalf("expected empty stt provider, got %q", cfg.STTProvider)
	}
	if cfg.STTCommand != "" {
		t.Fatalf("expected empty stt command, got %q", cfg.STTCommand)
	}
	if len(cfg.STTArgs) != 0 {
		t.Fatalf("expected no stt args, got %#v", cfg.STTArgs)
	}
	if cfg.STTTimeoutSeconds != 120 {
		t.Fatalf("expected stt timeout 120, got %d", cfg.STTTimeoutSeconds)
	}
}

func TestLoadMemoryDefaults(t *testing.T) {
	t.Setenv("MEMORY_PROVIDER", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("MEMORY_PROJECT_ALIASES", "")
	t.Setenv("CONTACT_ENCRYPTION_KEY", "")
	t.Setenv("CONTACT_ENCRYPTION_PREVIOUS_KEYS", "")
	t.Setenv("EMBEDDING_PROVIDER", "")
	t.Setenv("EMBEDDING_BASE_URL", "")
	t.Setenv("EMBEDDING_MODEL", "")

	cfg := Load()

	if cfg.MemoryProvider != "" {
		t.Fatalf("expected empty memory provider, got %q", cfg.MemoryProvider)
	}
	if cfg.DatabaseURL != "" {
		t.Fatalf("expected empty database url, got %q", cfg.DatabaseURL)
	}
	if len(cfg.ProjectAliases) != 0 {
		t.Fatalf("expected empty project aliases, got %#v", cfg.ProjectAliases)
	}
	if cfg.ContactEncryptionKey != "" {
		t.Fatalf("expected empty contact encryption key, got %q", cfg.ContactEncryptionKey)
	}
	if len(cfg.ContactPreviousEncryptionKeys) != 0 {
		t.Fatalf("expected empty previous contact keys, got %#v", cfg.ContactPreviousEncryptionKeys)
	}
	if cfg.EmbeddingProvider != "" {
		t.Fatalf("expected empty embedding provider, got %q", cfg.EmbeddingProvider)
	}
	if cfg.EmbeddingModel != "nomic-embed-text" {
		t.Fatalf("expected default embedding model, got %q", cfg.EmbeddingModel)
	}
	if cfg.EmbeddingBaseURL != "http://localhost:11434" {
		t.Fatalf("expected default embedding base url, got %q", cfg.EmbeddingBaseURL)
	}
}

func TestLoadPromptsDir(t *testing.T) {
	t.Setenv("PROMPTS_DIR", "/opt/ai/projects/robe/prompts")

	cfg := Load()

	if cfg.PromptsDir != "/opt/ai/projects/robe/prompts" {
		t.Fatalf("expected prompts dir, got %q", cfg.PromptsDir)
	}
}
