package journey

import "testing"

func TestStrConfigFallback(t *testing.T) {
	if got := strConfig(nil, "subject", "fallback"); got != "fallback" {
		t.Fatalf("got %q", got)
	}
	if got := strConfig(map[string]any{"subject": "Hello"}, "subject", "fallback"); got != "Hello" {
		t.Fatalf("got %q", got)
	}
}

func TestIntNumConfig(t *testing.T) {
	if got := intNumConfig(map[string]any{"delta": float64(8)}, "delta", 1); got != 8 {
		t.Fatalf("got %d", got)
	}
}
