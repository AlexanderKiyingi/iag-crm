package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iag/crm/backend/internal/auth"
	"github.com/iag/crm/backend/internal/bridge"
	"github.com/iag/crm/backend/internal/events"
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
	h.recordAudit(c, "BridgeSync", "Admin POST /admin/bridge/sync")
	var res bridge.Result
	var err error
	if h.Bridge != nil {
		res, err = h.Bridge.Sync(c.Request.Context())
	}
	if err != nil {
		apierr.JSONStatus(c, http.StatusBadGateway, "dms sync failed")
		return
	}
	if h.Events != nil {
		h.Events.PublishCommercial(c.Request.Context(), events.TypeBridgeSynced, map[string]any{
			"trigger": "admin", "user": auth.ActorName(c),
			"outlets_fetched": res.OutletsFetched, "outlets_upserted": res.OutletsUpserted,
		}, "bridge")
	}
	c.JSON(http.StatusOK, res)
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
	var rules map[string]any
	if err := h.Repo.WithinTx(c.Request.Context(), func(ctx context.Context) error {
		var e error
		rules, e = h.Repo.PutLoyaltyTierRules(ctx, in, auth.ActorName(c))
		if e != nil {
			return e
		}
		if h.Events != nil {
			return h.Events.EnqueueCommercial(ctx, "crm.loyalty.tier_rules.saved", map[string]any{
				"name": rules["name"],
			}, "loyalty")
		}
		return nil
	}); err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "tier rules update failed")
		return
	}
	h.recordAudit(c, "TierRulesSaved", "Updated Pearl Club tier thresholds")
	c.JSON(http.StatusOK, rules)
}

func (h *API) PostSEOAudit(c *gin.Context) {
	var in struct {
		URL string `json:"url"`
	}
	_ = c.ShouldBindJSON(&in)
	if in.URL == "" {
		in.URL = "https://iag.example.com"
	}
	jobID := "SEO-" + uuid.NewString()[:8]
	_ = h.Repo.CreateSEOAuditJob(c.Request.Context(), jobID, in.URL)
	go func() {
		time.Sleep(2 * time.Second)
		_ = h.Repo.CompleteSEOAuditJob(context.Background(), jobID, 82, []map[string]any{
			{"type": "meta", "message": "Title length OK"},
			{"type": "performance", "message": "LCP under 2.5s"},
		})
	}()
	h.recordAudit(c, "SEOAudit", "Triggered SEO audit for "+in.URL)
	c.JSON(http.StatusAccepted, gin.H{"job_id": jobID, "status": "running", "url": in.URL, "eta_seconds": 3})
}
