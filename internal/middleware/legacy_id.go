package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/iag/crm/backend/internal/store"
)

// ResolveLegacyID rewrites an :id path parameter that still carries one of the
// prefixed identifiers CRM used before migration 0011 (ACC-500, DEAL-500) into
// the uuid it became. Without it every bookmark, saved link and quoted document
// reference from before the cutover would fail to parse as a uuid.
//
// Routes whose :id belongs to another service — /finance/invoices/:id,
// /procurement/purchase-orders/:id — are unaffected: those ids are not CRM
// legacy codes, so they match nothing and pass through untouched.
func ResolveLegacyID(repo *store.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if repo == nil {
			c.Next()
			return
		}
		for i, p := range c.Params {
			if p.Key != "id" {
				continue
			}
			if resolved := repo.ResolveLegacyID(c.Request.Context(), p.Value); resolved != p.Value {
				c.Params[i].Value = resolved
			}
			break
		}
		c.Next()
	}
}
