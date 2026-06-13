package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/N1074/robe/internal/core"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmailapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

type GoogleConfig struct {
	CredentialsFile string
	TokenFile       string
	UserID          string
}

type GoogleGmail struct {
	service *gmailapi.Service
	userID  string
}

func NewGoogleGmail(ctx context.Context, cfg GoogleConfig) (*GoogleGmail, error) {
	cfg = normalizeGoogleConfig(cfg)
	if cfg.CredentialsFile == "" {
		return nil, errors.New("gmail credentials file is required")
	}
	if cfg.TokenFile == "" {
		return nil, errors.New("gmail token file is required")
	}

	oauthConfig, err := loadOAuthConfig(cfg.CredentialsFile)
	if err != nil {
		return nil, err
	}
	token, err := loadToken(cfg.TokenFile)
	if err != nil {
		return nil, err
	}

	service, err := gmailapi.NewService(ctx, option.WithHTTPClient(oauthConfig.Client(ctx, token)))
	if err != nil {
		return nil, err
	}

	return &GoogleGmail{service: service, userID: cfg.UserID}, nil
}

func AuthURL(credentialsFile string) (string, error) {
	oauthConfig, err := loadOAuthConfig(credentialsFile)
	if err != nil {
		return "", err
	}
	return oauthConfig.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce), nil
}

func ExchangeCode(ctx context.Context, credentialsFile string, tokenFile string, code string) error {
	oauthConfig, err := loadOAuthConfig(credentialsFile)
	if err != nil {
		return err
	}
	token, err := oauthConfig.Exchange(ctx, strings.TrimSpace(code))
	if err != nil {
		return err
	}
	return saveToken(tokenFile, token)
}

func (g *GoogleGmail) SearchEmails(ctx context.Context, query core.EmailQuery) ([]core.EmailMessage, error) {
	limit := emailLimit(query.Limit)

	list, err := g.service.Users.Messages.List(g.userID).
		Q(strings.TrimSpace(query.Query)).
		MaxResults(limit).
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}

	out := make([]core.EmailMessage, 0, len(list.Messages))
	for _, item := range list.Messages {
		if item == nil || item.Id == "" {
			continue
		}
		message, err := g.getMessage(ctx, item.Id, "metadata")
		if err != nil {
			continue
		}
		out = append(out, message)
	}
	return out, nil
}

func (g *GoogleGmail) SearchUnreadUnreviewedEmails(ctx context.Context, reviewedLabel string, limit int) ([]core.EmailMessage, error) {
	return g.SearchEmails(ctx, core.EmailQuery{
		Query: buildUnreadUnreviewedQuery(reviewedLabel),
		Limit: limit,
	})
}

func (g *GoogleGmail) GetEmail(ctx context.Context, id string) (core.EmailMessage, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return core.EmailMessage{}, errors.New("message id is required")
	}
	return g.getMessage(ctx, id, "full")
}

func (g *GoogleGmail) EnsureEmailLabel(ctx context.Context, name string) (core.EmailLabel, error) {
	name = normalizeLabelName(name)
	if name == "" {
		return core.EmailLabel{}, errors.New("label name is required")
	}

	labels, err := g.service.Users.Labels.List(g.userID).Context(ctx).Do()
	if err != nil {
		return core.EmailLabel{}, err
	}
	for _, label := range labels.Labels {
		if label != nil && strings.EqualFold(label.Name, name) {
			return core.EmailLabel{ID: label.Id, Name: label.Name}, nil
		}
	}

	created, err := g.service.Users.Labels.Create(g.userID, &gmailapi.Label{
		Name:                  name,
		LabelListVisibility:   "labelShow",
		MessageListVisibility: "show",
	}).Context(ctx).Do()
	if err != nil {
		return core.EmailLabel{}, err
	}
	return core.EmailLabel{ID: created.Id, Name: created.Name}, nil
}

func (g *GoogleGmail) ApplyEmailLabels(ctx context.Context, messageID string, labelIDs []string) error {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return errors.New("message id is required")
	}
	labelIDs = cleanLabelIDs(labelIDs)
	if len(labelIDs) == 0 {
		return errors.New("at least one label id is required")
	}

	_, err := g.service.Users.Messages.Modify(g.userID, messageID, &gmailapi.ModifyMessageRequest{
		AddLabelIds: labelIDs,
	}).Context(ctx).Do()
	return err
}

func (g *GoogleGmail) getMessage(ctx context.Context, id string, format string) (core.EmailMessage, error) {
	message, err := g.service.Users.Messages.Get(g.userID, id).
		Format(format).
		Context(ctx).
		Do()
	if err != nil {
		return core.EmailMessage{}, err
	}
	return convertMessage(message, g.userID), nil
}

func normalizeGoogleConfig(cfg GoogleConfig) GoogleConfig {
	cfg.CredentialsFile = strings.TrimSpace(cfg.CredentialsFile)
	cfg.TokenFile = strings.TrimSpace(cfg.TokenFile)
	cfg.UserID = strings.TrimSpace(cfg.UserID)
	if cfg.UserID == "" {
		cfg.UserID = "me"
	}
	return cfg
}

func loadOAuthConfig(credentialsFile string) (*oauth2.Config, error) {
	data, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, err
	}
	return google.ConfigFromJSON(data, gmailapi.GmailModifyScope)
}

func loadToken(path string) (*oauth2.Token, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var token oauth2.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, err
	}
	return &token, nil
}

func saveToken(path string, token *oauth2.Token) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(token)
}

func convertMessage(message *gmailapi.Message, userID string) core.EmailMessage {
	if message == nil {
		return core.EmailMessage{}
	}

	headers := messageHeaders(message.Payload)
	return core.EmailMessage{
		ID:           message.Id,
		ThreadID:     message.ThreadId,
		LabelIDs:     append([]string(nil), message.LabelIds...),
		From:         headers["from"],
		FromIdentity: core.ParseEmailIdentity(headers["from"]),
		To:           headers["to"],
		ToIdentities: core.ParseEmailIdentities(headers["to"]),
		Cc:           headers["cc"],
		CcIdentities: core.ParseEmailIdentities(headers["cc"]),
		Subject:      headers["subject"],
		Date:         parseEmailDate(headers["date"]),
		Snippet:      strings.TrimSpace(message.Snippet),
		PlainText:    strings.TrimSpace(plainTextPart(message.Payload)),
		WebURL:       gmailMessageURL(userID, message.Id),
	}
}

func emailLimit(limit int) int64 {
	if limit <= 0 {
		return 5
	}
	if limit > 10 {
		return 10
	}
	return int64(limit)
}

func buildUnreadUnreviewedQuery(reviewedLabel string) string {
	reviewedLabel = normalizeLabelName(reviewedLabel)
	if reviewedLabel == "" {
		return "is:unread"
	}
	return fmt.Sprintf("is:unread -label:%q", reviewedLabel)
}

func normalizeLabelName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "/")
	fields := strings.Fields(name)
	return strings.Join(fields, " ")
}

func cleanLabelIDs(labelIDs []string) []string {
	out := make([]string, 0, len(labelIDs))
	seen := map[string]bool{}
	for _, labelID := range labelIDs {
		labelID = strings.TrimSpace(labelID)
		if labelID == "" || seen[labelID] {
			continue
		}
		seen[labelID] = true
		out = append(out, labelID)
	}
	return out
}

func gmailMessageURL(userID string, messageID string) string {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return ""
	}
	user := strings.TrimSpace(userID)
	if user == "" || user == "me" {
		user = "0"
	}
	return "https://mail.google.com/mail/u/" + url.PathEscape(user) + "/#inbox/" + url.PathEscape(messageID)
}

func messageHeaders(part *gmailapi.MessagePart) map[string]string {
	out := map[string]string{}
	if part == nil {
		return out
	}
	for _, header := range part.Headers {
		if header == nil {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(header.Name))] = strings.TrimSpace(header.Value)
	}
	return out
}

func plainTextPart(part *gmailapi.MessagePart) string {
	if part == nil {
		return ""
	}
	if strings.EqualFold(part.MimeType, "text/plain") && part.Body != nil {
		return decodeBody(part.Body.Data)
	}
	for _, child := range part.Parts {
		if text := plainTextPart(child); text != "" {
			return text
		}
	}
	return ""
}

func decodeBody(data string) string {
	if strings.TrimSpace(data) == "" {
		return ""
	}
	decoded, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(data)
	}
	if err != nil {
		return ""
	}
	return string(decoded)
}

func parseEmailDate(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}
