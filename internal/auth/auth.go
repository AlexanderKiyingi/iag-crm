package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/iag/crm/backend/internal/middleware"
)

// AuthModeNone is set on the Gin context when AUTH_MODE=none (local dev).
const AuthModeNone = "auth_mode_none"

func SetAuthModeNone(c *gin.Context) {
	c.Set(AuthModeNone, true)
}

func isAuthDisabled(c *gin.Context) bool {
	if v, ok := c.Get(AuthModeNone); ok {
		if b, ok := v.(bool); ok && b {
			return true
		}
	}
	return false
}

func HasPerm(c *gin.Context, codename string) bool {
	if isAuthDisabled(c) {
		return true
	}
	claims, ok := middleware.Claims(c)
	if !ok || claims == nil {
		return false
	}
	if claims.IsSuperuser || claims.IsStaff {
		return true
	}
	if claims.HasPermission(codename) {
		return true
	}
	perms := claims.Permissions
	if len(perms) == 0 {
		// Dev tokens without explicit permissions — allow (service enforces RBAC via bootstrap).
		return true
	}
	for _, p := range perms {
		if p == "*" || p == codename {
			return true
		}
	}
	return false
}

func RequirePerm(codename string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isAuthDisabled(c) {
			c.Next()
			return
		}
		if _, ok := middleware.Claims(c); !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		if !HasPerm(c, codename) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "permission denied", "permission": codename})
			return
		}
		c.Next()
	}
}

func RequireAnyPerm(codenames ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isAuthDisabled(c) {
			c.Next()
			return
		}
		if _, ok := middleware.Claims(c); !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		for _, codename := range codenames {
			if HasPerm(c, codename) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "permission denied"})
	}
}

func RequireStaff() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isAuthDisabled(c) {
			c.Next()
			return
		}
		claims, ok := middleware.Claims(c)
		if !ok || claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		if !claims.IsStaff && !claims.IsSuperuser {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "staff access required"})
			return
		}
		c.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return RequireAnyPerm("admin.read", "admin.update")
}

func ActorName(c *gin.Context) string {
	return middleware.ActorName(c)
}
