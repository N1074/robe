package telegram

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitTelegramMessageKeepsShortMessage(t *testing.T) {
	got := splitTelegramMessage("hello")
	if len(got) != 1 || got[0] != "hello" {
		t.Fatalf("unexpected chunks: %#v", got)
	}
}

func TestSplitTelegramMessageSplitsLongMessage(t *testing.T) {
	got := splitTelegramMessage(strings.Repeat("a", telegramMaxMessageRunes+10))
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(got))
	}

	for _, chunk := range got {
		if utf8.RuneCountInString(chunk) > telegramMaxMessageRunes {
			t.Fatalf("chunk too long: %d", utf8.RuneCountInString(chunk))
		}
	}
}

func TestSplitTelegramMessageDoesNotBreakUTF8(t *testing.T) {
	got := splitTelegramMessage(strings.Repeat("ñ", telegramMaxMessageRunes+10))
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(got))
	}

	for _, chunk := range got {
		if !utf8.ValidString(chunk) {
			t.Fatalf("invalid utf8 chunk")
		}
	}
}
