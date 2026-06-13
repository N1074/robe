package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

func emailHash(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	sum := sha256.Sum256([]byte(email))
	return fmt.Sprintf("%x", sum[:])
}

func deriveContactEncryptionKey(secret string) []byte {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func deriveContactEncryptionKeys(secrets []string) [][]byte {
	keys := make([][]byte, 0, len(secrets))
	for _, secret := range secrets {
		if key := deriveContactEncryptionKey(secret); len(key) > 0 {
			keys = append(keys, key)
		}
	}
	return keys
}

func (s *PostgresMemoryStore) encryptContactEmail(email string) (string, error) {
	return s.encryptContactPrivateValue(email)
}

func (s *PostgresMemoryStore) decryptContactEmail(value string) string {
	return s.decryptContactPrivateValue(value)
}

func (s *PostgresMemoryStore) encryptContactPrivateValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(s.contactEncryptionKey) == 0 {
		return "", nil
	}
	return encryptContactPrivateValueWithKey(value, s.contactEncryptionKey)
}

func encryptContactPrivateValueWithKey(value string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(value), nil)
	payload := append(nonce, ciphertext...)
	return "enc:v1:" + base64.RawURLEncoding.EncodeToString(payload), nil
}

func (s *PostgresMemoryStore) decryptContactPrivateValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "enc:v1:") {
		return value
	}
	if plaintext, ok := decryptContactPrivateValueWithKey(value, s.contactEncryptionKey); ok {
		return plaintext
	}
	for _, key := range s.previousContactEncryptionKeys {
		if plaintext, ok := decryptContactPrivateValueWithKey(value, key); ok {
			return plaintext
		}
	}
	return ""
}

func decryptContactPrivateValueWithKey(value string, key []byte) (string, bool) {
	if len(key) == 0 {
		return "", false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, "enc:v1:"))
	if err != nil {
		return "", false
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", false
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", false
	}
	if len(payload) < gcm.NonceSize() {
		return "", false
	}
	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", false
	}
	return string(plaintext), true
}

func (s *PostgresMemoryStore) RotateContactEncryption(ctx context.Context) error {
	if len(s.contactEncryptionKey) == 0 {
		return errors.New("contact encryption key is required")
	}

	rows, err := s.db.QueryContext(ctx, `SELECT id, full_name FROM contacts WHERE full_name <> ''`)
	if err != nil {
		return err
	}
	type contactValue struct {
		id    int64
		value string
	}
	var contacts []contactValue
	for rows.Next() {
		var item contactValue
		if err := rows.Scan(&item.id, &item.value); err != nil {
			rows.Close()
			return err
		}
		contacts = append(contacts, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, item := range contacts {
		plain := s.decryptContactPrivateValue(item.value)
		if plain == "" {
			continue
		}
		encrypted, err := s.encryptContactPrivateValue(plain)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE contacts SET full_name = $2, updated_at = now() WHERE id = $1`, item.id, encrypted); err != nil {
			return err
		}
	}

	addressRows, err := s.db.QueryContext(ctx, `SELECT address_hash, email, display_name_seen FROM contact_addresses WHERE email <> '' OR display_name_seen <> ''`)
	if err != nil {
		return err
	}
	type addressValue struct {
		hash        string
		email       string
		displayName string
	}
	var addresses []addressValue
	for addressRows.Next() {
		var item addressValue
		if err := addressRows.Scan(&item.hash, &item.email, &item.displayName); err != nil {
			addressRows.Close()
			return err
		}
		addresses = append(addresses, item)
	}
	if err := addressRows.Err(); err != nil {
		addressRows.Close()
		return err
	}
	addressRows.Close()

	for _, item := range addresses {
		email := s.decryptContactPrivateValue(item.email)
		displayName := s.decryptContactPrivateValue(item.displayName)
		encryptedEmail, err := s.encryptContactPrivateValue(email)
		if err != nil {
			return err
		}
		encryptedDisplayName, err := s.encryptContactPrivateValue(displayName)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE contact_addresses SET email = $2, display_name_seen = $3, updated_at = now() WHERE address_hash = $1`, item.hash, encryptedEmail, encryptedDisplayName); err != nil {
			return err
		}
	}
	return nil
}
