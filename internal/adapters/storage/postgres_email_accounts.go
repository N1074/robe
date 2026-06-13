package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/N1074/robe/internal/core"
)

func (s *PostgresMemoryStore) UpsertEmailAccount(ctx context.Context, account core.EmailAccount) (core.EmailAccount, error) {
	account = core.NormalizeEmailAccount(account)
	if account.Provider == "" {
		return core.EmailAccount{}, errors.New("email account provider is required")
	}
	if account.AccountKey == ":" || account.AccountKey == "" {
		return core.EmailAccount{}, errors.New("email account key is required")
	}

	now := time.Now()
	var id int64
	err := s.db.QueryRowContext(ctx, `
INSERT INTO email_accounts (provider, account_key, display_name, user_id, credentials_file, token_file, status, autoreview_enabled, notify_telegram_enabled, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (account_key) DO UPDATE SET
	provider = EXCLUDED.provider,
	display_name = EXCLUDED.display_name,
	user_id = EXCLUDED.user_id,
	credentials_file = EXCLUDED.credentials_file,
	token_file = EXCLUDED.token_file,
	status = EXCLUDED.status,
	autoreview_enabled = EXCLUDED.autoreview_enabled,
	notify_telegram_enabled = EXCLUDED.notify_telegram_enabled,
	updated_at = EXCLUDED.updated_at
RETURNING id, created_at, updated_at
`, account.Provider, account.AccountKey, account.DisplayName, account.UserID, account.CredentialsFile, account.TokenFile, account.Status, account.AutoReviewEnabled, account.NotifyTelegramEnabled, now, now).Scan(&id, &account.CreatedAt, &account.UpdatedAt)
	if err != nil {
		return core.EmailAccount{}, err
	}
	account.ID = fmt.Sprintf("%d", id)
	return account, nil
}

func (s *PostgresMemoryStore) ListEmailAccounts(ctx context.Context, autoReviewOnly bool) ([]core.EmailAccount, error) {
	query := `
SELECT id::text, provider, account_key, display_name, user_id, credentials_file, token_file, status, autoreview_enabled, notify_telegram_enabled, created_at, updated_at
FROM email_accounts
WHERE status = 'active'
`
	if autoReviewOnly {
		query += " AND autoreview_enabled = true"
	}
	query += " ORDER BY account_key"

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []core.EmailAccount
	for rows.Next() {
		var account core.EmailAccount
		if err := rows.Scan(&account.ID, &account.Provider, &account.AccountKey, &account.DisplayName, &account.UserID, &account.CredentialsFile, &account.TokenFile, &account.Status, &account.AutoReviewEnabled, &account.NotifyTelegramEnabled, &account.CreatedAt, &account.UpdatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, core.NormalizeEmailAccount(account))
	}
	return accounts, rows.Err()
}
