package store

import (
	"testing"

	"github.com/iag/crm/backend/internal/crypto"
)

func TestRepositoryTokenSealRoundTrip(t *testing.T) {
	r := &Repository{tokenKey: crypto.DeriveKey("integration-token-secret-32chars-min")}
	plain := "refresh-token-abc"
	sealed, err := r.sealToken(plain)
	if err != nil {
		t.Fatal(err)
	}
	if sealed == plain {
		t.Fatal("expected sealed token")
	}
	got, err := r.openToken(sealed)
	if err != nil || got != plain {
		t.Fatalf("got %q err %v", got, err)
	}
}
