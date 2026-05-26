package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/iag/crm/backend/internal/ctxkeys"
	"github.com/iag/crm/backend/internal/platformauth"
)

type PlatformAuth struct {
	mode     string
	verifier *platformauth.Verifier
}

func NewPlatformAuth(mode string, verifier *platformauth.Verifier) *PlatformAuth {
	return &PlatformAuth{mode: mode, verifier: verifier}
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
		if m.mode == "none" {
			c.Next()
			return
		}
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := m.verifier.Verify(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
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
