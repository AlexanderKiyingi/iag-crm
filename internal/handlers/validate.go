package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/alvor-technologies/iag-platform-go/apierr"
	"github.com/iag/crm/backend/internal/middleware"
	"github.com/iag/crm/backend/internal/models"
)

// Write-side guards shared by the entity handlers.
//
// Two things live here. Owner stamping: a caller in a scoped role (sales_rep)
// sees only rows whose owner equals their email, so a record they create with
// a blank or hand-typed owner vanishes from their own list the moment it is
// saved. The create path now fills the owner in from the token, and the patch
// path refuses to let a scoped caller hand a record to someone else — the same
// disappearing act, one save later.
//
// Vocabulary and range checks: the columns a query reads get a 400 for a value
// no query will match, rather than storing it. See models/vocab.go.

func callerEmail(c *gin.Context) string {
	claims, ok := middleware.Claims(c)
	if !ok || claims == nil {
		return ""
	}
	return strings.TrimSpace(claims.Email)
}

func callerIsScoped(c *gin.Context) bool {
	claims, ok := middleware.Claims(c)
	if !ok || claims == nil {
		return false
	}
	return models.IsScopedRole(models.RoleFromGroups(claims.Groups, claims.IsSuperuser))
}

// stampOwner fills a blank owner from the caller. Only the blank case: a
// manager creating a lead for a rep types the rep, and that must stand.
func stampOwner(c *gin.Context, owner *string) {
	if strings.TrimSpace(*owner) == "" {
		*owner = callerEmail(c)
	}
}

// guardOwnerPatch aborts with 403 when a scoped caller tries to set owner to
// anyone but themselves. Returns false when it aborted.
func guardOwnerPatch(c *gin.Context, patch map[string]any) bool {
	if !callerIsScoped(c) {
		return true
	}
	v, ok := patch["owner"].(string)
	if !ok {
		return true
	}
	if email := callerEmail(c); strings.TrimSpace(v) != "" && email != "" && strings.TrimSpace(v) != email {
		apierr.JSONStatus(c, http.StatusForbidden, "a sales rep cannot reassign a record to another owner")
		return false
	}
	return true
}

// requireVocab 400s when value is set and not in allowed. Blank passes so the
// store can apply its default; the store is where blanks are decided.
func requireVocab(c *gin.Context, field, value string, allowed []string) bool {
	if value == "" || models.InVocab(value, allowed) {
		return true
	}
	badRequest(c, models.VocabError(field, allowed))
	return false
}

// patchVocab is requireVocab for a sparse PATCH body: absent passes, present
// must be a string in the vocabulary.
func patchVocab(c *gin.Context, patch map[string]any, field string, allowed []string) bool {
	raw, ok := patch[field]
	if !ok {
		return true
	}
	s, isStr := raw.(string)
	if !isStr {
		badRequest(c, field+" must be a string")
		return false
	}
	return requireVocab(c, field, s, allowed)
}

func requireRange(c *gin.Context, field string, v, lo, hi int) bool {
	if models.InRange(v, lo, hi) {
		return true
	}
	badRequest(c, field+" must be between 0 and 100")
	return false
}

// patchRange checks a numeric key in a sparse PATCH body. JSON numbers arrive
// as float64; the record clients send strings, which the store already reads,
// so both are accepted here.
func patchRange(c *gin.Context, patch map[string]any, field string, lo, hi int) bool {
	raw, ok := patch[field]
	if !ok {
		return true
	}
	var n float64
	switch v := raw.(type) {
	case float64:
		n = v
	case string:
		if strings.TrimSpace(v) == "" {
			return true
		}
		parsed, ok := parseFloat(v)
		if !ok {
			badRequest(c, field+" must be a number")
			return false
		}
		n = parsed
	case nil:
		return true
	default:
		badRequest(c, field+" must be a number")
		return false
	}
	return requireRange(c, field, int(n), lo, hi)
}

// requireCurrency defaults a blank code and rejects a malformed one.
func requireCurrency(c *gin.Context, code *string) bool {
	*code = strings.ToUpper(strings.TrimSpace(*code))
	if *code == "" {
		*code = models.DefaultCurrency
		return true
	}
	if models.ValidCurrency(*code) {
		return true
	}
	badRequest(c, "currency must be a three-letter ISO code")
	return false
}

// patchCurrency upper-cases and validates a currency in a sparse body. A blank
// on update is the client clearing the field; the store keeps '' for that, and
// the overview treats '' as the default currency.
func patchCurrency(c *gin.Context, patch map[string]any) bool {
	raw, ok := patch["currency"]
	if !ok {
		return true
	}
	s, isStr := raw.(string)
	if !isStr {
		badRequest(c, "currency must be a string")
		return false
	}
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		patch["currency"] = models.DefaultCurrency
		return true
	}
	if !models.ValidCurrency(s) {
		badRequest(c, "currency must be a three-letter ISO code")
		return false
	}
	patch["currency"] = s
	return true
}

func parseFloat(s string) (float64, bool) {
	n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
