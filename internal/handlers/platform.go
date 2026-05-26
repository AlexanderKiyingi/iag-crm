package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/iag/crm/backend/internal/auth"
	"github.com/iag/crm/backend/internal/config"
	"github.com/iag/crm/backend/internal/events"
	"github.com/iag/crm/backend/internal/middleware"
	"github.com/iag/crm/backend/internal/models"
	"github.com/iag/crm/backend/internal/store"
)

type API struct {
	Repo   *store.Repository
	Cfg    config.Config
	Events *events.Bus
}

func listOpts(c *gin.Context) store.ListOpts {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	return store.ListOpts{
		Limit:  limit,
		Offset: offset,
		Owner:  c.Query("owner"),
		Stage:  c.Query("stage"),
		Status: c.Query("status"),
		Search: c.Query("q"),
	}
}

func scopedListOpts(c *gin.Context) store.ListOpts {
	opts := listOpts(c)
	if claims, ok := middleware.Claims(c); ok && claims != nil {
		role := models.RoleFromGroups(claims.Groups, claims.IsSuperuser)
		if role == "sales_rep" && opts.Owner == "" && claims.Email != "" {
			opts.Owner = claims.Email
		}
	}
	return opts
}

func paginated[T any](c *gin.Context, data []T, total int) {
	opts := scopedListOpts(c)
	c.JSON(http.StatusOK, models.Paginated[T]{
		Data: data,
		Meta: models.ListMeta{Total: total, Limit: opts.Limit, Offset: opts.Offset},
	})
}

func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": msg})
}

func notFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

func (h *API) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "crm"})
}

func (h *API) Ready(c *gin.Context) {
	if err := h.Repo.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "database": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "database": true})
}

func (h *API) Notifications(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": store.Notifications()})
}

func (h *API) Overview(c *gin.Context) {
	rangeKey := c.DefaultQuery("range", "week")
	metrics := store.OverviewMetrics(rangeKey)
	summary, err := h.Repo.PipelineSummary(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "overview failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"range":    rangeKey,
		"metrics":  metrics,
		"pipeline": summary,
	})
}

func (h *API) PipelineBoard(c *gin.Context) {
	cols, err := h.Repo.PipelineBoard(c.Request.Context(), c.Query("owner"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pipeline failed"})
		return
	}
	summary, _ := h.Repo.PipelineSummary(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"columns": cols, "summary": summary})
}

func (h *API) BridgeStatus(c *gin.Context) {
	c.JSON(http.StatusOK, h.Repo.BridgeStatus(c.Request.Context()))
}

func (h *API) Sync(c *gin.Context) {
	_ = h.Repo.AppendBridgeSyncLog(c.Request.Context(), "Manual bridge sync triggered")
	h.recordAudit(c, "BridgeSync", "POST /bridge/sync")
	if h.Events != nil {
		h.Events.PublishCommercial(c.Request.Context(), "crm.bridge.synced", map[string]any{
			"trigger": "user",
			"user":    auth.ActorName(c),
		}, "bridge")
	}
	c.JSON(http.StatusOK, gin.H{
		"synced_at": "now",
		"events":    3,
		"status":    "connected",
	})
}
