package core

import (
	"context"
	"strings"
	"time"
)

const (
	EmailAccountProviderGmail = "gmail"
	EmailAccountStatusActive  = "active"
)

type EmailAccount struct {
	ID                    string
	Provider              string
	AccountKey            string
	DisplayName           string
	UserID                string
	CredentialsFile       string
	TokenFile             string
	Status                string
	AutoReviewEnabled     bool
	NotifyTelegramEnabled bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type EmailAccountStore interface {
	UpsertEmailAccount(ctx context.Context, account EmailAccount) (EmailAccount, error)
	ListEmailAccounts(ctx context.Context, autoReviewOnly bool) ([]EmailAccount, error)
}

func NormalizeEmailAccount(account EmailAccount) EmailAccount {
	account.Provider = strings.ToLower(strings.TrimSpace(account.Provider))
	account.AccountKey = strings.TrimSpace(account.AccountKey)
	account.DisplayName = strings.TrimSpace(account.DisplayName)
	account.UserID = strings.TrimSpace(account.UserID)
	account.CredentialsFile = strings.TrimSpace(account.CredentialsFile)
	account.TokenFile = strings.TrimSpace(account.TokenFile)
	account.Status = strings.ToLower(strings.TrimSpace(account.Status))
	if account.Status == "" {
		account.Status = EmailAccountStatusActive
	}
	if account.Provider == EmailAccountProviderGmail && account.UserID == "" {
		account.UserID = "me"
	}
	if account.AccountKey == "" {
		account.AccountKey = account.Provider + ":" + account.UserID
	}
	return account
}
