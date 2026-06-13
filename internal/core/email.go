package core

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	EmailLabelReviewed        = "Robe/Reviewed"
	EmailLabelImportant       = "Robe/Important"
	EmailLabelNeedsAttention  = "Robe/NeedsAttention"
	EmailLabelAdmin           = "Robe/Category/Admin"
	EmailLabelPeople          = "Robe/Category/People"
	EmailLabelOnlinePurchases = "Robe/Category/OnlinePurchases"
	EmailLabelFinance         = "Robe/Category/Finance"
	EmailLabelProjects        = "Robe/Category/Projects"
	EmailLabelNotifications   = "Robe/Category/Notifications"
	EmailLabelOther           = "Robe/Category/Other"
)

type Email interface {
	SearchEmails(ctx context.Context, query EmailQuery) ([]EmailMessage, error)
	GetEmail(ctx context.Context, id string) (EmailMessage, error)
}

type EmailReviewStore interface {
	SearchUnreadUnreviewedEmails(ctx context.Context, reviewedLabel string, limit int) ([]EmailMessage, error)
	EnsureEmailLabel(ctx context.Context, name string) (EmailLabel, error)
	ApplyEmailLabels(ctx context.Context, messageID string, labelIDs []string) error
}

type EmailQuery struct {
	Query string
	Limit int
}

type EmailLabel struct {
	ID   string
	Name string
}

type EmailMessage struct {
	ID           string
	ThreadID     string
	LabelIDs     []string
	From         string
	FromIdentity EmailIdentity
	To           string
	ToIdentities []EmailIdentity
	Cc           string
	CcIdentities []EmailIdentity
	Subject      string
	Date         time.Time
	Snippet      string
	PlainText    string
	WebURL       string
}

func (a *Assistant) handleEmail(ctx context.Context, text string) (string, error) {
	if a.email == nil {
		return emailNotConfiguredMessage(), nil
	}

	arg := strings.TrimSpace(strings.TrimPrefix(text, "/email"))
	switch {
	case arg == "":
		return emailUsage(), nil
	case arg == "review dry-run":
		return a.handleEmailReviewDryRun(ctx)
	case strings.HasPrefix(arg, "search "):
		return a.searchEmail(ctx, strings.TrimSpace(strings.TrimPrefix(arg, "search ")))
	case arg == "show raw":
		return "Usage: /email show raw <message_id>", nil
	case strings.HasPrefix(arg, "show raw "):
		return a.showEmailRaw(ctx, strings.TrimSpace(strings.TrimPrefix(arg, "show raw ")))
	case strings.HasPrefix(arg, "show "):
		return a.showEmail(ctx, strings.TrimSpace(strings.TrimPrefix(arg, "show ")))
	default:
		return emailUsage(), nil
	}
}

func (a *Assistant) searchEmail(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "Usage: /email search <query>", nil
	}

	messages, err := a.email.SearchEmails(ctx, EmailQuery{Query: query, Limit: 5})
	if err != nil {
		return "", err
	}
	if len(messages) == 0 {
		return "No email found.", nil
	}

	var b strings.Builder
	b.WriteString("Email:\n")
	for _, message := range messages {
		b.WriteString("- ")
		b.WriteString(formatEmailSummary(message))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func (a *Assistant) showEmail(ctx context.Context, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "Usage: /email show <message_id>", nil
	}

	message, err := a.email.GetEmail(ctx, id)
	if err != nil {
		return "", err
	}

	return "Email:\n" + formatEmailDetail(message, false), nil
}

func (a *Assistant) showEmailRaw(ctx context.Context, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "Usage: /email show raw <message_id>", nil
	}

	message, err := a.email.GetEmail(ctx, id)
	if err != nil {
		return "", err
	}

	return "Email:\n" + formatEmailDetail(message, true), nil
}

func formatEmailSummary(message EmailMessage) string {
	var b strings.Builder
	if strings.TrimSpace(message.ID) != "" {
		b.WriteString("[")
		b.WriteString(message.ID)
		b.WriteString("] ")
	}
	b.WriteString(nonEmpty(message.Subject, "(no subject)"))
	if strings.TrimSpace(message.From) != "" {
		b.WriteString(" | From: ")
		b.WriteString(formatEmailSender(message))
	}
	if strings.TrimSpace(message.Snippet) != "" {
		b.WriteString(" | ")
		b.WriteString(message.Snippet)
	}
	if strings.TrimSpace(message.WebURL) != "" {
		b.WriteString(" | ")
		b.WriteString(message.WebURL)
	}
	return b.String()
}

func formatEmailDetail(message EmailMessage, raw bool) string {
	var b strings.Builder
	b.WriteString("ID: ")
	b.WriteString(message.ID)
	if strings.TrimSpace(message.ThreadID) != "" {
		b.WriteString("\nThread: ")
		b.WriteString(message.ThreadID)
	}
	b.WriteString("\nFrom: ")
	if raw {
		b.WriteString(message.From)
	} else {
		b.WriteString(formatEmailSender(message))
	}
	if strings.TrimSpace(message.To) != "" {
		b.WriteString("\nTo: ")
		if raw {
			b.WriteString(message.To)
		} else {
			b.WriteString(formatEmailParticipants(message.ToIdentities))
		}
	}
	if strings.TrimSpace(message.Cc) != "" {
		b.WriteString("\nCc: ")
		if raw {
			b.WriteString(message.Cc)
		} else {
			b.WriteString(formatEmailParticipants(message.CcIdentities))
		}
	}
	b.WriteString("\nSubject: ")
	b.WriteString(nonEmpty(message.Subject, "(no subject)"))
	if !message.Date.IsZero() {
		b.WriteString("\nDate: ")
		b.WriteString(formatTime(message.Date))
	}
	if strings.TrimSpace(message.WebURL) != "" {
		b.WriteString("\nLink: ")
		b.WriteString(message.WebURL)
	}
	if raw && strings.TrimSpace(message.PlainText) != "" {
		b.WriteString("\n\n")
		b.WriteString(message.PlainText)
	} else if !raw {
		safe := EmailMessageForPrompt(message)
		if strings.TrimSpace(safe) != "" {
			b.WriteString("\n\n")
			b.WriteString(safe)
		}
	} else if strings.TrimSpace(message.Snippet) != "" {
		b.WriteString("\n\n")
		b.WriteString(message.Snippet)
	}
	return b.String()
}

func emailNotConfiguredMessage() string {
	return "Email is not configured yet. Set EMAIL_PROVIDER=gmail and configure Gmail OAuth token files, then restart Robe."
}

func emailUsage() string {
	return "Email commands:\n/email search <query>\n/email show <message_id>\n/email show raw <message_id>\n/email review dry-run"
}

func formatEmailSender(message EmailMessage) string {
	identity := message.FromIdentity
	if strings.TrimSpace(identity.Alias) == "" {
		identity = ParseEmailIdentity(message.From)
	}
	return nonEmpty(identity.Alias, "Unknown sender")
}

func formatEmailParticipants(identities []EmailIdentity) string {
	if len(identities) == 0 {
		return "Unknown recipient"
	}
	aliases := make([]string, 0, len(identities))
	for _, identity := range identities {
		if strings.TrimSpace(identity.Alias) != "" {
			aliases = append(aliases, identity.Alias)
		}
	}
	if len(aliases) == 0 {
		return "Unknown recipient"
	}
	return strings.Join(aliases, ", ")
}

func externalContentForPrompt(label string, content string) string {
	label = strings.TrimSpace(label)
	content = RedactExternalContentForPrompt(content)
	if label == "" {
		return content
	}
	if content == "" {
		return label + ":"
	}
	return fmt.Sprintf("%s:\n%s", label, content)
}

func EmailMessageForPrompt(message EmailMessage) string {
	identity := message.FromIdentity
	if strings.TrimSpace(identity.Alias) == "" {
		identity = ParseEmailIdentity(message.From)
	}

	var b strings.Builder
	b.WriteString("From: ")
	b.WriteString(identity.Alias)
	b.WriteString(" (")
	b.WriteString(nonEmpty(identity.Kind, "unknown_sender"))
	b.WriteString(")")
	if len(message.ToIdentities) > 0 {
		b.WriteString("\nTo: ")
		b.WriteString(formatEmailParticipants(message.ToIdentities))
	}
	if len(message.CcIdentities) > 0 {
		b.WriteString("\nCc: ")
		b.WriteString(formatEmailParticipants(message.CcIdentities))
	}
	b.WriteString("\nSubject: ")
	b.WriteString(redactEmailIdentityFromPromptText(message.Subject, identity))

	body := strings.TrimSpace(message.PlainText)
	if body == "" {
		body = message.Snippet
	}
	if strings.TrimSpace(body) != "" {
		b.WriteString("\nBody:\n")
		b.WriteString(redactEmailIdentityFromPromptText(body, identity))
	}
	return b.String()
}

func redactEmailIdentityFromPromptText(text string, identity EmailIdentity) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if strings.TrimSpace(identity.RawName) != "" && strings.TrimSpace(identity.Alias) != "" {
		text = strings.ReplaceAll(text, identity.RawName, identity.Alias)
	}
	if strings.TrimSpace(identity.RawEmail) != "" {
		text = strings.ReplaceAll(text, identity.RawEmail, "[REDACTED_EMAIL]")
	}
	return RedactExternalContentForPrompt(text)
}

func DefaultEmailReviewLabels() []string {
	return []string{
		EmailLabelReviewed,
		EmailLabelImportant,
		EmailLabelNeedsAttention,
		EmailLabelAdmin,
		EmailLabelPeople,
		EmailLabelOnlinePurchases,
		EmailLabelFinance,
		EmailLabelProjects,
		EmailLabelNotifications,
		EmailLabelOther,
	}
}
