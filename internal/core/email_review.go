package core

import (
	"context"
	"strings"
)

type EmailClassifier interface {
	ClassifyEmail(ctx context.Context, req EmailClassificationRequest) (EmailClassification, error)
}

type EmailClassificationRequest struct {
	Message EmailMessage
	Prompt  string
}

type EmailClassification struct {
	Labels          []string
	Important       bool
	Summary         string
	ContactProposal ContactProfileProposal
}

type EmailReviewOptions struct {
	DryRun bool
	Limit  int
}

type EmailReviewResult struct {
	MessageID       string
	Sender          string
	Subject         string
	Labels          []string
	Important       bool
	Summary         string
	WebURL          string
	DryRun          bool
	ContactProposal ContactProfileProposal
}

func (a *Assistant) handleEmailReviewDryRun(ctx context.Context) (string, error) {
	results, err := a.ReviewUnreadEmails(ctx, EmailReviewOptions{DryRun: true, Limit: 10})
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "No unread unreviewed email found.", nil
	}

	var b strings.Builder
	b.WriteString("Email review dry-run:\n")
	for _, result := range results {
		b.WriteString("- [")
		b.WriteString(result.MessageID)
		b.WriteString("] ")
		b.WriteString(result.Subject)
		b.WriteString(" | From: ")
		b.WriteString(result.Sender)
		b.WriteString(" | Labels: ")
		b.WriteString(strings.Join(result.Labels, ", "))
		if result.Important {
			b.WriteString(" | Important")
		}
		if strings.TrimSpace(result.Summary) != "" {
			b.WriteString(" | ")
			b.WriteString(result.Summary)
		}
		if strings.TrimSpace(result.WebURL) != "" {
			b.WriteString(" | ")
			b.WriteString(result.WebURL)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func (a *Assistant) ReviewUnreadEmails(ctx context.Context, opts EmailReviewOptions) ([]EmailReviewResult, error) {
	store, ok := a.email.(EmailReviewStore)
	if !ok || store == nil {
		return nil, emailNotConfiguredMessageError()
	}

	limit := opts.Limit
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	messages, err := store.SearchUnreadUnreviewedEmails(ctx, EmailLabelReviewed, limit)
	if err != nil {
		return nil, err
	}

	results := make([]EmailReviewResult, 0, len(messages))
	for _, message := range messages {
		result := a.classifyEmailForReview(ctx, message)
		result.DryRun = opts.DryRun
		results = append(results, result)

		action := emailLabelAction(message.ID, result.Labels, opts.DryRun)
		decision := a.decide(action)
		if decision.Decision == DecisionDeny {
			a.recordAudit(ctx, action, decision, AuditResultRejected, nil)
			continue
		}

		if opts.DryRun {
			a.recordAudit(ctx, action, decision, AuditResultProposed, nil)
			a.auditContactProposalIfPresent(ctx, message, result)
			continue
		}

		var labelIDs []string
		for _, labelName := range result.Labels {
			label, err := store.EnsureEmailLabel(ctx, labelName)
			if err != nil {
				a.recordAudit(ctx, action, decision, AuditResultFailed, err)
				return results, err
			}
			labelIDs = append(labelIDs, label.ID)
		}
		if err := store.ApplyEmailLabels(ctx, message.ID, labelIDs); err != nil {
			a.recordAudit(ctx, action, decision, AuditResultFailed, err)
			return results, err
		}
		a.recordAudit(ctx, action, decision, AuditResultExecuted, nil)
		a.persistContactProposalIfPresent(ctx, message, result)
	}

	return results, nil
}

func (a *Assistant) classifyEmailForReview(ctx context.Context, message EmailMessage) EmailReviewResult {
	if a.emailClassifier != nil {
		classification, err := a.emailClassifier.ClassifyEmail(ctx, EmailClassificationRequest{
			Message: message,
			Prompt:  EmailMessageForPrompt(message),
		})
		if err == nil {
			if result, ok := emailReviewResultFromClassification(message, classification); ok {
				return result
			}
		}
	}
	return classifyEmailForReviewRules(message)
}

func classifyEmailForReviewRules(message EmailMessage) EmailReviewResult {
	text := strings.ToLower(message.Subject + " " + message.Snippet + " " + message.PlainText + " " + message.FromIdentity.Kind)
	labels := []string{EmailLabelReviewed}
	important := false

	switch {
	case containsAny(text, "factura", "invoice", "pedido", "order", "envio", "shipping", "delivery", "receipt", "recibo"):
		labels = append(labels, EmailLabelOnlinePurchases)
	case containsAny(text, "agencia tributaria", "administracion", "admin", "tax", "hacienda", "seguridad social", "government"):
		labels = append(labels, EmailLabelAdmin)
		important = true
	case containsAny(text, "payment", "bank", "banco", "card", "tarjeta", "transfer", "finance"):
		labels = append(labels, EmailLabelFinance)
		important = true
	case containsAny(text, "newsletter", "unsubscribe", "noreply", "no-reply"):
		labels = append(labels, EmailLabelNotifications)
	default:
		labels = append(labels, EmailLabelOther)
	}

	if important {
		labels = append(labels, EmailLabelImportant, EmailLabelNeedsAttention)
	}

	return EmailReviewResult{
		MessageID: message.ID,
		Sender:    formatEmailSender(message),
		Subject:   RedactExternalContentForPrompt(message.Subject),
		Labels:    uniqueStrings(labels),
		Important: important,
		Summary:   reviewSummary(message),
		WebURL:    message.WebURL,
	}
}

func emailReviewResultFromClassification(message EmailMessage, classification EmailClassification) (EmailReviewResult, bool) {
	labels := validEmailReviewLabels(classification.Labels)
	if len(labels) == 0 {
		return EmailReviewResult{}, false
	}
	if !containsString(labels, EmailLabelReviewed) {
		labels = append([]string{EmailLabelReviewed}, labels...)
	}
	if classification.Important {
		labels = append(labels, EmailLabelImportant, EmailLabelNeedsAttention)
	}
	summary := strings.TrimSpace(classification.Summary)
	if summary == "" {
		summary = reviewSummary(message)
	}
	return EmailReviewResult{
		MessageID:       message.ID,
		Sender:          formatEmailSender(message),
		Subject:         RedactExternalContentForPrompt(message.Subject),
		Labels:          uniqueStrings(labels),
		Important:       classification.Important,
		Summary:         RedactExternalContentForPrompt(summary),
		WebURL:          message.WebURL,
		ContactProposal: classification.ContactProposal,
	}, true
}

func validEmailReviewLabels(labels []string) []string {
	allowed := map[string]bool{}
	for _, label := range DefaultEmailReviewLabels() {
		allowed[label] = true
	}
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if allowed[label] {
			out = append(out, label)
		}
	}
	return uniqueStrings(out)
}

func (a *Assistant) persistContactProposalIfPresent(ctx context.Context, message EmailMessage, result EmailReviewResult) {
	if a.contactDirectory == nil {
		return
	}
	proposal := contactProposalForReview(message, result)
	if contactProposalIsEmpty(proposal) {
		return
	}
	contact, err := a.contactDirectory.UpsertEmailContact(ctx, message.FromIdentity)
	if err != nil {
		return
	}
	proposal.ContactID = contact.ID
	if strings.TrimSpace(proposal.Alias) == "" {
		proposal.Alias = contact.Alias
	}
	_, _ = a.ApplyContactProfileProposal(ctx, proposal)
}

func (a *Assistant) auditContactProposalIfPresent(ctx context.Context, message EmailMessage, result EmailReviewResult) {
	proposal := contactProposalForReview(message, result)
	if contactProposalIsEmpty(proposal) {
		return
	}

	action := contactProfileAction("", proposal)
	if err := validateContactProfileProposal(proposal); err != nil {
		a.recordAudit(ctx, action, PermissionDecision{RiskLevel: RiskMedium, Decision: DecisionDeny, Reason: err.Error()}, AuditResultRejected, err)
		return
	}

	decision := a.decide(action)
	if decision.Decision == DecisionDeny {
		a.recordAudit(ctx, action, decision, AuditResultRejected, nil)
		return
	}
	a.recordAudit(ctx, action, decision, AuditResultProposed, nil)
}

func contactProposalForReview(message EmailMessage, result EmailReviewResult) ContactProfileProposal {
	proposal := result.ContactProposal
	if strings.TrimSpace(proposal.Alias) == "" {
		proposal.Alias = message.FromIdentity.Alias
	}
	if strings.TrimSpace(proposal.Alias) == "" {
		proposal.Alias = formatEmailSender(message)
	}
	return proposal
}

func contactProposalIsEmpty(proposal ContactProfileProposal) bool {
	return strings.TrimSpace(proposal.Kind) == "" && strings.TrimSpace(proposal.Relationship) == "" && proposal.Importance == 0 && strings.TrimSpace(proposal.Reason) == ""
}

func reviewSummary(message EmailMessage) string {
	source := strings.TrimSpace(message.Snippet)
	if source == "" {
		source = strings.TrimSpace(message.Subject)
	}
	return RedactExternalContentForPrompt(source)
}

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func emailNotConfiguredMessageError() error {
	return simpleError(emailNotConfiguredMessage())
}

type simpleError string

func (e simpleError) Error() string {
	return string(e)
}
