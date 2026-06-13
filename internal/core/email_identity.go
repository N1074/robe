package core

import (
	"net/mail"
	"strings"
	"unicode"
)

type EmailIdentity struct {
	RawName      string
	RawEmail     string
	Alias        string
	Kind         string
	Relationship string
	ProjectSlug  string
	Known        bool
}

func ParseEmailIdentity(raw string) EmailIdentity {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return EmailIdentity{Alias: "Unknown sender", Kind: "unknown_sender"}
	}

	parsed, err := mail.ParseAddress(raw)
	if err == nil {
		name := strings.TrimSpace(parsed.Name)
		email := strings.TrimSpace(parsed.Address)
		alias := AliasDisplayName(name)
		if alias == "" {
			alias = "Unknown sender"
		}
		return EmailIdentity{
			RawName:  name,
			RawEmail: email,
			Alias:    alias,
			Kind:     inferIdentityKind(name, email),
		}
	}

	alias := AliasDisplayName(raw)
	if alias == "" || strings.Contains(alias, "@") {
		alias = "Unknown sender"
	}
	return EmailIdentity{
		RawName: raw,
		Alias:   alias,
		Kind:    inferIdentityKind(raw, ""),
	}
}

func ParseEmailIdentities(raw string) []EmailIdentity {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	addresses, err := mail.ParseAddressList(raw)
	if err == nil {
		out := make([]EmailIdentity, 0, len(addresses))
		for _, address := range addresses {
			if address == nil {
				continue
			}
			out = append(out, ParseEmailIdentity(address.String()))
		}
		return out
	}

	return []EmailIdentity{ParseEmailIdentity(raw)}
}

func AliasDisplayName(name string) string {
	name = strings.TrimSpace(stripEmailLikeTokens(name))
	if name == "" {
		return ""
	}

	tokens := cleanNameTokens(name)
	if len(tokens) == 0 {
		return ""
	}
	if len(tokens) == 1 {
		return tokens[0]
	}

	var b strings.Builder
	b.WriteString(tokens[0])
	for _, token := range tokens[1:] {
		initial := firstLetter(token)
		if initial == "" {
			continue
		}
		b.WriteString(" ")
		b.WriteString(initial)
		b.WriteString(".")
	}
	return b.String()
}

func SanitizedEmailSenderForPrompt(message EmailMessage) string {
	identity := message.FromIdentity
	if strings.TrimSpace(identity.Alias) == "" {
		identity = ParseEmailIdentity(message.From)
	}
	kind := nonEmpty(identity.Kind, "unknown_sender")
	if identity.Known {
		kind = "known_" + kind
	}
	return identity.Alias + " (" + kind + ")"
}

func cleanNameTokens(name string) []string {
	name = asciiFoldName(name)
	fields := strings.FieldsFunc(name, func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == ';' || r == '<' || r == '>' || r == '"' || r == '\'' || r == '(' || r == ')'
	})

	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(field, ".-_")
		if field == "" || strings.Contains(field, "@") {
			continue
		}
		out = append(out, titleToken(field))
	}
	return out
}

func stripEmailLikeTokens(value string) string {
	fields := strings.Fields(value)
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if strings.Contains(field, "@") {
			continue
		}
		out = append(out, field)
	}
	return strings.Join(out, " ")
}

func titleToken(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	upperNext := true
	for _, r := range value {
		if r == '-' {
			b.WriteRune(r)
			upperNext = true
			continue
		}
		if upperNext {
			b.WriteRune(unicode.ToUpper(r))
			upperNext = false
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func firstLetter(value string) string {
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return string(unicode.ToUpper(r))
		}
	}
	return ""
}

func inferIdentityKind(name string, email string) string {
	source := strings.ToLower(strings.TrimSpace(name + " " + email))
	switch {
	case strings.Contains(source, "noreply"), strings.Contains(source, "no-reply"), strings.Contains(source, "newsletter"):
		return "service"
	case strings.Contains(source, "admin"), strings.Contains(source, "administracion"), strings.Contains(source, "support"), strings.Contains(source, "soporte"):
		return "organization"
	case strings.TrimSpace(name) != "":
		return "person_or_org"
	default:
		return "unknown_sender"
	}
}

func asciiFoldName(value string) string {
	replacer := strings.NewReplacer(
		"á", "a", "à", "a", "ä", "a", "â", "a", "Á", "A", "À", "A", "Ä", "A", "Â", "A",
		"é", "e", "è", "e", "ë", "e", "ê", "e", "É", "E", "È", "E", "Ë", "E", "Ê", "E",
		"í", "i", "ì", "i", "ï", "i", "î", "i", "Í", "I", "Ì", "I", "Ï", "I", "Î", "I",
		"ó", "o", "ò", "o", "ö", "o", "ô", "o", "Ó", "O", "Ò", "O", "Ö", "O", "Ô", "O",
		"ú", "u", "ù", "u", "ü", "u", "û", "u", "Ú", "U", "Ù", "U", "Ü", "U", "Û", "U",
		"ñ", "n", "Ñ", "N", "ç", "c", "Ç", "C",
	)
	return replacer.Replace(value)
}
