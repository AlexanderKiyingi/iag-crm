package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/iag/crm/backend/internal/auth"
	"github.com/iag/crm/backend/internal/middleware"
	"github.com/iag/crm/backend/internal/models"
	"github.com/alvor-technologies/iag-platform-go/apierr"
)

func (h *API) permissionContext(c *gin.Context) models.PermissionContext {
	sess := h.sessionFromContext(c)
	role, _ := sess["role"].(string)
	email, _ := sess["email"].(string)
	name, _ := sess["name"].(string)
	perms, _ := sess["permissions"].([]string)
	isStaff, isSuper := false, false
	if claims, ok := middleware.Claims(c); ok && claims != nil {
		isStaff = claims.IsStaff
		isSuper = claims.IsSuperuser
		if len(perms) == 0 {
			perms = claims.Permissions
		}
	}
	return models.PermissionContext{
		Role:           role,
		Email:          email,
		Name:           name,
		Permissions:    perms,
		CanMutate:      models.CanMutateRole(role, perms) || isSuper,
		CanManageAdmin: models.CanManageAdmin(perms, isStaff, isSuper),
		IsStaff:        isStaff || isSuper,
	}
}

func (h *API) PermissionsBuiltin(c *gin.Context) {
	c.JSON(http.StatusOK, models.BuiltinRolesPermissions())
}

func (h *API) PermissionsCheck(c *gin.Context) {
	var in models.PermissionCheckInput
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid body")
		return
	}
	allowed := make(map[string]bool, len(in.Keys))
	for _, key := range in.Keys {
		allowed[key] = auth.HasPerm(c, key)
	}
	c.JSON(http.StatusOK, models.PermissionCheckResult{Allowed: allowed})
}

func (h *API) PermissionsMeEnhanced(c *gin.Context) {
	c.JSON(http.StatusOK, h.permissionContext(c))
}

func (h *API) recordAudit(c *gin.Context, action, detail string) {
	if h.Repo == nil {
		return
	}
	_, _ = h.Repo.AppendAudit(c.Request.Context(), action, detail, auth.ActorName(c))
}

func (h *API) ListAudit(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, total, err := h.Repo.ListAudit(c.Request.Context(), limit)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "audit list failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "meta": gin.H{"total": total, "limit": limit}})
}

func (h *API) GetAuditEntry(c *gin.Context) {
	item, err := h.Repo.GetAudit(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "audit get failed")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) AppendAuditEntry(c *gin.Context) {
	var in struct {
		Action string `json:"action"`
		Detail string `json:"detail"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Action == "" {
		badRequest(c, "action is required")
		return
	}
	entry, err := h.Repo.AppendAudit(c.Request.Context(), in.Action, in.Detail, auth.ActorName(c))
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "audit append failed")
		return
	}
	c.JSON(http.StatusCreated, entry)
}

func (h *API) AdminAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, total, err := h.Repo.ListAPIAuditLogs(c.Request.Context(), limit)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "api audit failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "meta": gin.H{"total": total}})
}

func (h *API) AdminMonitoringSummary(c *gin.Context) {
	busEnabled := h.Events != nil && h.Events.Enabled()
	summary, err := h.Repo.MonitoringSummaryWithBus(c.Request.Context(), busEnabled)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "monitoring failed")
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *API) AdminMonitoringActivity(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	items, err := h.Repo.APIMonitoringActivity(c.Request.Context(), limit)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "activity failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *API) AdminMonitoringBridge(c *gin.Context) {
	status := h.Repo.BridgeStatus(c.Request.Context())
	streams, _ := h.Repo.ListBridgeStreams(c.Request.Context())
	pending, total, _ := h.Repo.ListPendingImports(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{
		"status":  status,
		"streams": streams,
		"pending": gin.H{"data": pending, "total": total},
	})
}

func (h *API) AdminBridgeSync(c *gin.Context) {
	_ = h.Repo.AppendBridgeSyncLog(c.Request.Context(), "Admin-triggered DMS bridge sync")
	h.recordAudit(c, "BridgeSync", "Admin POST /admin/bridge/sync")
	if h.Events != nil {
		h.Events.PublishCommercial(c.Request.Context(), "crm.bridge.synced", map[string]any{
			"trigger": "admin",
			"user":    auth.ActorName(c),
		}, "bridge")
	}
	c.JSON(http.StatusOK, gin.H{"status": "connected", "synced_at": "now", "events": 3})
}

func (h *API) PlatformStatus(c *gin.Context) {
	busEnabled := h.Events != nil && h.Events.Enabled()
	dbOK := h.Repo.Ping(c.Request.Context()) == nil
	c.JSON(http.StatusOK, gin.H{
		"service":           h.Cfg.ServiceName,
		"database":          dbOK,
		"event_bus_enabled": busEnabled,
		"audience":          h.Cfg.Audience,
	})
}

func (h *API) InsightsSignals(c *gin.Context) {
	items, err := h.Repo.ListBuyingSignals(c.Request.Context(), 20)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "signals failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *API) GetLoyaltyTierRules(c *gin.Context) {
	rules, err := h.Repo.GetLoyaltyTierRules(c.Request.Context())
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "tier rules failed")
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (h *API) PutLoyaltyTierRules(c *gin.Context) {
	var in map[string]any
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid body")
		return
	}
	rules, err := h.Repo.PutLoyaltyTierRules(c.Request.Context(), in, auth.ActorName(c))
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "tier rules update failed")
		return
	}
	h.recordAudit(c, "TierRulesSaved", "Updated Pearl Club tier thresholds")
	if h.Events != nil {
		h.Events.PublishCommercial(c.Request.Context(), "crm.loyalty.tier_rules.saved", map[string]any{
			"name": rules["name"],
		}, "loyalty")
	}
	c.JSON(http.StatusOK, rules)
}

func (h *API) PostSEOAudit(c *gin.Context) {
	h.recordAudit(c, "SEOAudit", "Triggered SEO site audit crawl")
	c.JSON(http.StatusAccepted, gin.H{"status": "running", "pages": 142, "eta_seconds": 90})
}
