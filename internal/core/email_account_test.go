package core

import "testing"

func TestNormalizeEmailAccount(t *testing.T) {
	account := NormalizeEmailAccount(EmailAccount{
		Provider: " GMAIL ",
		UserID:   "",
	})

	if account.Provider != EmailAccountProviderGmail {
		t.Fatalf("unexpected provider: %q", account.Provider)
	}
	if account.UserID != "me" {
		t.Fatalf("unexpected user id: %q", account.UserID)
	}
	if account.AccountKey != "gmail:me" {
		t.Fatalf("unexpected account key: %q", account.AccountKey)
	}
	if account.Status != EmailAccountStatusActive {
		t.Fatalf("unexpected status: %q", account.Status)
	}
}
