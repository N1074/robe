package storage

import (
	"testing"
)

func TestEncodeDecodeEmbedding(t *testing.T) {
	encoded, err := encodeEmbedding([]float64{0.1, 0.2, 0.3})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got, err := decodeEmbedding([]byte(encoded.(string)))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(got) != 3 || got[0] != 0.1 || got[2] != 0.3 {
		t.Fatalf("unexpected embedding: %#v", got)
	}
}

func TestEncodeEmptyEmbeddingReturnsNil(t *testing.T) {
	encoded, err := encodeEmbedding(nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if encoded != nil {
		t.Fatalf("expected nil encoded embedding, got %#v", encoded)
	}
}

func TestCosineSimilarity(t *testing.T) {
	similar := cosineSimilarity([]float64{1, 0}, []float64{1, 0})
	orthogonal := cosineSimilarity([]float64{1, 0}, []float64{0, 1})

	if similar != 1 {
		t.Fatalf("expected perfect similarity, got %f", similar)
	}
	if orthogonal != 0 {
		t.Fatalf("expected orthogonal similarity 0, got %f", orthogonal)
	}
}

func TestEmailHashNormalizesCaseAndSpace(t *testing.T) {
	a := emailHash(" Sender@Example.com ")
	b := emailHash("sender@example.com")

	if a == "" || a != b {
		t.Fatalf("expected normalized email hashes to match: %q %q", a, b)
	}
}

func TestNormalizeProjectSlugStorage(t *testing.T) {
	if got := normalizeProjectSlugStorage("My_Project"); got != "my-project" {
		t.Fatalf("unexpected project slug: %q", got)
	}
	if got := normalizeProjectSlugStorage("global"); got != "" {
		t.Fatalf("expected global to normalize empty, got %q", got)
	}
}

func TestEncryptContactEmail(t *testing.T) {
	store := &PostgresMemoryStore{contactEncryptionKey: deriveContactEncryptionKey("test-secret")}

	encrypted, err := store.encryptContactEmail("sender@example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if encrypted == "" || encrypted == "sender@example.com" {
		t.Fatalf("expected encrypted value, got %q", encrypted)
	}
	if got := store.decryptContactEmail(encrypted); got != "sender@example.com" {
		t.Fatalf("expected decrypted email, got %q", got)
	}
}

func TestEncryptContactEmailWithoutKeyStoresNoRawEmail(t *testing.T) {
	store := &PostgresMemoryStore{}

	encrypted, err := store.encryptContactEmail("sender@example.com")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if encrypted != "" {
		t.Fatalf("expected no stored raw email without key, got %q", encrypted)
	}
}
