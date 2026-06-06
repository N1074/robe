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
