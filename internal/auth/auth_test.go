package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/iag/crm/backend/internal/ctxkeys"
	"github.com/iag/crm/backend/internal/platformauth"
)

func TestHasPerm_StrictRBACDeniesEmptyPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetStrictRBAC(true)
	defer SetStrictRBAC(false)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(ctxkeys.Claims, &platformauth.Claims{Email: "rep@example.com"})

	if HasPerm(c, "accounts.read") {
		t.Fatal("expected deny when strict RBAC and no permissions")
	}
}

func TestHasPerm_DevAllowsEmptyPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetStrictRBAC(false)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(ctxkeys.Claims, &platformauth.Claims{Email: "rep@example.com"})

	if !HasPerm(c, "accounts.read") {
		t.Fatal("expected allow in dev mode without permissions")
	}
}
