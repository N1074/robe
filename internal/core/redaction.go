package core

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	emailPattern      = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	ibanPattern       = regexp.MustCompile(`(?i)\b[A-Z]{2}\d{2}[A-Z0-9]{11,30}\b`)
	secretPattern     = regexp.MustCompile(`(?i)\b(api[_ -]?key|token|secret|password|passwd|contraseña)\s*[:=]\s*[^\s,;]+`)
	oauthCodePattern  = regexp.MustCompile(`(?i)\b(code|oauth_code)\s*[:=]\s*[A-Za-z0-9._\-]{12,}\b`)
	signedURLPattern  = regexp.MustCompile(`(?i)\bhttps?://[^\s]+(?:token|signature|sig|expires|X-Amz-Signature|X-Goog-Signature)=[^\s]+`)
	phonePattern      = regexp.MustCompile(`(?:\+\d{1,3}[\s.-]?)?(?:\(?\d{2,4}\)?[\s.-]?){2,4}\d{2,4}`)
	cardNumberPattern = regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`)
)

func redactForPrompt(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	text = signedURLPattern.ReplaceAllString(text, "[REDACTED_SIGNED_URL]")
	text = secretPattern.ReplaceAllString(text, "$1=[REDACTED_SECRET]")
	text = oauthCodePattern.ReplaceAllString(text, "$1=[REDACTED_OAUTH_CODE]")
	text = emailPattern.ReplaceAllString(text, "[REDACTED_EMAIL]")
	text = ibanPattern.ReplaceAllString(text, "[REDACTED_BANK_ACCOUNT]")
	text = cardNumberPattern.ReplaceAllStringFunc(text, redactCardNumber)
	text = phonePattern.ReplaceAllStringFunc(text, redactPhoneNumber)

	return text
}

func redactCardNumber(value string) string {
	digits := onlyDigits(value)
	if len(digits) < 13 || len(digits) > 19 || !passesLuhn(digits) {
		return value
	}
	return "[REDACTED_CARD]"
}

func redactPhoneNumber(value string) string {
	digits := onlyDigits(value)
	if len(digits) < 9 || len(digits) > 15 {
		return value
	}
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "+") {
		return "[REDACTED_PHONE]"
	}
	if len(digits) == 9 && strings.ContainsAny(value, " .-()") && strings.ContainsRune("6789", rune(digits[0])) {
		return "[REDACTED_PHONE]"
	}
	if len(digits) == 10 && strings.Contains(value, "(") && strings.Contains(value, ")") {
		return "[REDACTED_PHONE]"
	}
	if len(digits) == 10 && strings.Count(value, "-") >= 2 && !strings.HasPrefix(digits, "20") {
		return "[REDACTED_PHONE]"
	}
	if len(digits) == 11 && strings.HasPrefix(digits, "1") && strings.ContainsAny(value, " .-()") {
		return "[REDACTED_PHONE]"
	}
	if len(digits) > 11 && strings.HasPrefix(digits, "00") && strings.ContainsAny(value, " .-()") {
		return "[REDACTED_PHONE]"
	}
	if !strings.ContainsAny(value, " .-()") {
		return value
	}
	return value
}

func onlyDigits(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func passesLuhn(digits string) bool {
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		n := int(digits[i] - '0')
		if double {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		double = !double
	}
	return sum%10 == 0
}
