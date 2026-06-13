package core

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	emailPattern          = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	ibanPattern           = regexp.MustCompile(`(?i)\b[A-Z]{2}\d{2}[A-Z0-9]{11,30}\b`)
	secretPattern         = regexp.MustCompile(`(?i)\b(api[_ -]?key|access[_ -]?token|refresh[_ -]?token|id[_ -]?token|bot[_ -]?token|token|client[_ -]?secret|credential|secret|password|passwd|contrasena|private[_ -]?key)\s*[:=]\s*("[^"]+"|'[^']+'|[^\s,;]+)`)
	oauthCodePattern      = regexp.MustCompile(`(?i)\b(code|oauth_code)\s*[:=]\s*[A-Za-z0-9._\-]{12,}\b`)
	authHeaderPattern     = regexp.MustCompile(`(?i)\b(authorization)\s*:\s*(bearer|basic)\s+[A-Za-z0-9._~+/=\-]+`)
	credentialURLPattern  = regexp.MustCompile(`(?i)\b([a-z][a-z0-9+\-.]*://)([^/\s:@]+):([^@\s/]+)@`)
	signedURLPattern      = regexp.MustCompile(`(?i)\bhttps?://[^\s]+(?:token|signature|sig|expires|X-Amz-Signature|X-Goog-Signature|access_token|auth|key)=[^\s]+`)
	unsubscribeURLPattern = regexp.MustCompile(`(?i)\bhttps?://[^\s]*(?:unsubscribe|optout|opt-out|email-preferences|preferences)[^\s]*`)
	privateKeyPattern     = regexp.MustCompile(`(?is)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`)
	jwtPattern            = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)?\b`)
	awsAccessKeyPattern   = regexp.MustCompile(`\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`)
	githubTokenPattern    = regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9_]{36,}\b`)
	slackTokenPattern     = regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`)
	googleAPIKeyPattern   = regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{30,45}\b`)
	ssnPattern            = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	spanishIDPattern      = regexp.MustCompile(`(?i)\b(?:\d{8}[A-Z]|[XYZ]\d{7}[A-Z])\b`)
	phonePattern          = regexp.MustCompile(`(?:\+\d{1,3}[\s.-]?)?(?:\(?\d{2,4}\)?[\s.-]?){2,4}\d{2,4}`)
	cardNumberPattern     = regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`)
)

func redactForPrompt(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	text = privateKeyPattern.ReplaceAllString(text, "[REDACTED_PRIVATE_KEY]")
	text = credentialURLPattern.ReplaceAllString(text, "$1[REDACTED_CREDENTIALS]@")
	text = unsubscribeURLPattern.ReplaceAllString(text, "[REDACTED_UNSUBSCRIBE_URL]")
	text = signedURLPattern.ReplaceAllString(text, "[REDACTED_SIGNED_URL]")
	text = authHeaderPattern.ReplaceAllString(text, "$1: [REDACTED_AUTH_HEADER]")
	text = secretPattern.ReplaceAllString(text, "$1=[REDACTED_SECRET]")
	text = oauthCodePattern.ReplaceAllString(text, "$1=[REDACTED_OAUTH_CODE]")
	text = jwtPattern.ReplaceAllString(text, "[REDACTED_JWT]")
	text = awsAccessKeyPattern.ReplaceAllString(text, "[REDACTED_AWS_ACCESS_KEY]")
	text = githubTokenPattern.ReplaceAllString(text, "[REDACTED_GITHUB_TOKEN]")
	text = slackTokenPattern.ReplaceAllString(text, "[REDACTED_SLACK_TOKEN]")
	text = googleAPIKeyPattern.ReplaceAllString(text, "[REDACTED_GOOGLE_API_KEY]")
	text = ssnPattern.ReplaceAllString(text, "[REDACTED_GOVERNMENT_ID]")
	text = spanishIDPattern.ReplaceAllString(text, "[REDACTED_GOVERNMENT_ID]")
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
