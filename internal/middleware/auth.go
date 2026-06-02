package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/iag/crm/backend/internal/ctxkeys"
	"github.com/iag/crm/backend/internal/platformauth"
	"github.com/alvor-technologies/iag-platform-go/apierr"
)

// PlatformAuth verifies every inbound request's Bearer JWT (issuer + aud
// enforced by the verifier). No header trust, no dev bypass.
type PlatformAuth struct {
	verifier *platformauth.Verifier
}

func NewPlatformAuth(verifier *platformauth.Verifier) *PlatformAuth {
	return &PlatformAuth{verifier: verifier}
}

func isPublicPath(path string) bool {
	switch path {
	case "/health", "/healthz", "/ready", "/", "/ui":
		return true
	case "/v1/health", "/v1/health/live", "/v1/health/ready":
		return true
	default:
		return false
	}
}

func (m *PlatformAuth) AttachPrincipal() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isPublicPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		if m.verifier == nil {
			apierr.Write(c, http.StatusServiceUnavailable, apierr.CodeServiceUnavailable, "JWT verifier not configured")
			return
		}
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			apierr.Unauthorized(c, "missing bearer token")
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := m.verifier.Verify(token)
		if err != nil {
			apierr.Unauthorized(c, "invalid or expired token")
			return
		}
		c.Set(ctxkeys.Claims, claims)
		c.Next()
	}
}

func Claims(c *gin.Context) (*platformauth.Claims, bool) {
	v, ok := c.Get(ctxkeys.Claims)
	if !ok {
		return nil, false
	}
	cl, ok := v.(*platformauth.Claims)
	return cl, ok
}

func ActorName(c *gin.Context) string {
	if claims, ok := Claims(c); ok && claims != nil {
		if n := strings.TrimSpace(claims.Name); n != "" {
			return n
		}
		if e := strings.TrimSpace(claims.Email); e != "" {
			return e
		}
	}
	return "system"
}
