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
