package storage

import (
	"strings"
	"time"
)

func nonEmpty(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func defaultInt(value int, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func defaultFloat(value float64, fallback float64) float64 {
	if value == 0 {
		return fallback
	}
	return value
}

func nonZeroTime(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value
}
