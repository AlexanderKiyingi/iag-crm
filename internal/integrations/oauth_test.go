package integrations

import (
	"strings"
	"testing"
)

func TestOAuthStateRoundTripSigned(t *testing.T) {
	s := &Service{stateSecret: []byte("prod-oauth-state-secret-key")}
	state, err := s.encodeState("user@example.com", "/crm/settings")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(state, ".") {
		t.Fatal("expected signed state")
	}
	email, ret, err := s.decodeState(state)
	if err != nil {
		t.Fatal(err)
	}
	if email != "user@example.com" || ret != "/crm/settings" {
		t.Fatalf("got %q %q", email, ret)
	}
}

func TestOAuthStateRejectsTamper(t *testing.T) {
	s := &Service{stateSecret: []byte("prod-oauth-state-secret-key"), requireSignedState: true}
	state, err := s.encodeState("user@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(state, ".", 2)
	if _, _, err := s.decodeState(parts[0] + ".bad-signature"); err == nil {
		t.Fatal("expected signature error")
	}
}

func TestOAuthStateRequiresSecretInProduction(t *testing.T) {
	s := &Service{requireSignedState: true}
	if _, err := s.encodeState("user@example.com", ""); err == nil {
		t.Fatal("expected error without signing secret")
	}
}

func TestOAuthStateDevUnsigned(t *testing.T) {
	s := &Service{}
	state, err := s.encodeState("dev@local", "")
	if err != nil {
		t.Fatal(err)
	}
	email, _, err := s.decodeState(state)
	if err != nil || email != "dev@local" {
		t.Fatalf("email=%q err=%v", email, err)
	}
}
