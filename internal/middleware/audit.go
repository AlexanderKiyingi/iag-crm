package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/iag/crm/backend/internal/store"
)

func RequestAudit(repo *store.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if repo == nil {
			return
		}
		path := c.Request.URL.Path
		if path == "/health" || path == "/healthz" || path == "/ready" ||
			path == "/v1/health" || path == "/v1/health/live" || path == "/v1/health/ready" {
			return
		}
		duration := int(time.Since(start).Milliseconds())
		_ = repo.LogAPIRequest(
			c.Request.Context(),
			c.Request.Method,
			path,
			c.Writer.Status(),
			ActorName(c),
			duration,
			c.ClientIP(),
		)
	}
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = c.GetHeader("X-Correlation-ID")
		}
		if id != "" {
			c.Header("X-Request-ID", id)
		}
		c.Next()
	}
}
