package crypto

import "testing"

func TestSealRoundTrip(t *testing.T) {
	key := DeriveKey("integration-token-secret-32chars-min")
	plain := "ya29.access-token-value"
	sealed, err := Seal(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	if sealed == plain {
		t.Fatal("expected sealed ciphertext")
	}
	got, err := Unseal(key, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("got %q want %q", got, plain)
	}
}

func TestUnsealPlaintextPassthrough(t *testing.T) {
	got, err := Unseal(DeriveKey("integration-token-secret-32chars-min"), "plain-token")
	if err != nil || got != "plain-token" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestDeriveKeyTooShort(t *testing.T) {
	if got := DeriveKey("short"); got != nil {
		t.Fatalf("expected nil key, got %v", got)
	}
}
