package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/iag/crm/backend/internal/models"
)

func testCtx() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/", nil)
	return c, w
}

func TestPatchVocabRejectsUntranslatedWords(t *testing.T) {
	c, w := testCtx()
	if patchVocab(c, map[string]any{"status": "Open"}, "status", models.TicketStatuses) {
		t.Fatal("a Title-case status must be refused, not stored")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	c, _ = testCtx()
	if !patchVocab(c, map[string]any{"status": "open"}, "status", models.TicketStatuses) {
		t.Error("a vocabulary word must pass")
	}
	if !patchVocab(c, map[string]any{"other": 1}, "status", models.TicketStatuses) {
		t.Error("an absent key must pass — the patch is sparse")
	}
}

// The record clients send every scalar as a string; the store already reads
// them, so the range check must too.
func TestPatchRangeAcceptsStringsAndNumbers(t *testing.T) {
	for _, v := range []any{float64(50), "50", "", nil} {
		c, _ := testCtx()
		if !patchRange(c, map[string]any{"score": v}, "score", 0, 100) {
			t.Errorf("score=%v (%T) should pass", v, v)
		}
	}
	for _, v := range []any{float64(101), "-1", "abc", true} {
		c, w := testCtx()
		if patchRange(c, map[string]any{"score": v}, "score", 0, 100) {
			t.Errorf("score=%v (%T) should be refused", v, v)
		} else if w.Code != http.StatusBadRequest {
			t.Errorf("score=%v: status %d, want 400", v, w.Code)
		}
	}
}

func TestCurrencyDefaultsAndUppercases(t *testing.T) {
	c, _ := testCtx()
	code := ""
	if !requireCurrency(c, &code) || code != models.DefaultCurrency {
		t.Errorf("blank currency should default to %s, got %q", models.DefaultCurrency, code)
	}
	code = " usd "
	if !requireCurrency(c, &code) || code != "USD" {
		t.Errorf("currency should be trimmed and upper-cased, got %q", code)
	}
	c, w := testCtx()
	code = "shillings"
	if requireCurrency(c, &code) || w.Code != http.StatusBadRequest {
		t.Error("a non-ISO currency must be refused")
	}
	patch := map[string]any{"currency": "ugx"}
	c, _ = testCtx()
	if !patchCurrency(c, patch) || patch["currency"] != "UGX" {
		t.Errorf("patchCurrency should normalise in place, got %v", patch["currency"])
	}
}

// No claims on the context means an unscoped caller; the guard must not abort.
func TestGuardOwnerPatchWithoutClaimsPasses(t *testing.T) {
	c, _ := testCtx()
	if !guardOwnerPatch(c, map[string]any{"owner": "someone@else"}) {
		t.Error("no claims → not scoped → must pass")
	}
	owner := ""
	stampOwner(c, &owner)
	if owner != "" {
		t.Error("no claims → nothing to stamp")
	}
}
