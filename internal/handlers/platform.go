package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/alvor-technologies/iag-platform-go/apierr"
	"github.com/iag/crm/backend/internal/aiclient"
	"github.com/iag/crm/backend/internal/auth"
	"github.com/iag/crm/backend/internal/bridge"
	"github.com/iag/crm/backend/internal/config"
	"github.com/iag/crm/backend/internal/contractsclient"
	"github.com/iag/crm/backend/internal/dmsclient"
	"github.com/iag/crm/backend/internal/events"
	"github.com/iag/crm/backend/internal/financeclient"
	"github.com/iag/crm/backend/internal/integrations"
	"github.com/iag/crm/backend/internal/middleware"
	"github.com/iag/crm/backend/internal/models"
	"github.com/iag/crm/backend/internal/procurementclient"
	"github.com/iag/crm/backend/internal/store"
	"github.com/iag/crm/backend/internal/usersclient"
)

type API struct {
	Repo         *store.Repository
	Cfg          config.Config
	Events       *events.Bus
	Finance      *financeclient.Client
	Users        *usersclient.Client
	DMS          *dmsclient.Client
	Contracts    *contractsclient.Client
	AI           *aiclient.Client
	Bridge       *bridge.Service
	Integrations *integrations.Service
	Procurement  *procurementclient.Client
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
		Type:   c.Query("type"),
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

// enforceOwner applies the same row-level scoping as scopedListOpts to a single
// fetched record. When the caller is a sales_rep and the record is owned by
// someone else, it responds 404 (don't leak existence) and returns false so the
// handler stops. Managers/superusers are unrestricted, and records with no owner
// are left visible. This is access control by sales-rep ownership, not tenancy.
func enforceOwner(c *gin.Context, ownerEmail string) bool {
	claims, ok := middleware.Claims(c)
	if !ok || claims == nil {
		return true
	}
	if models.RoleFromGroups(claims.Groups, claims.IsSuperuser) != "sales_rep" {
		return true
	}
	if ownerEmail != "" && claims.Email != "" && ownerEmail != claims.Email {
		notFound(c)
		return false
	}
	return true
}

func paginated[T any](c *gin.Context, data []T, total int) {
	opts := scopedListOpts(c)
	c.JSON(http.StatusOK, models.Paginated[T]{
		Data: data,
		Meta: models.ListMeta{Total: total, Limit: opts.Limit, Offset: opts.Offset},
	})
}

func badRequest(c *gin.Context, msg string) {
	apierr.JSONStatus(c, http.StatusBadRequest, msg)
}

func notFound(c *gin.Context) {
	apierr.JSONStatus(c, http.StatusNotFound, "not found")
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
	items, err := h.Repo.LiveNotifications(c.Request.Context(), 9)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": store.Notifications()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *API) Search(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		badRequest(c, "q is required")
		return
	}
	items, err := h.Repo.GlobalSearch(c.Request.Context(), q, 25)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "search failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *API) Overview(c *gin.Context) {
	rangeKey := c.DefaultQuery("range", "week")
	metrics, err := h.Repo.ComputeOverviewMetrics(c.Request.Context(), rangeKey)
	if err != nil {
		metrics = store.OverviewMetrics(rangeKey)
	}
	summary, err := h.Repo.PipelineSummary(c.Request.Context())
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "overview failed")
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
		apierr.JSONStatus(c, http.StatusInternalServerError, "pipeline failed")
		return
	}
	summary, _ := h.Repo.PipelineSummary(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"columns": cols, "summary": summary})
}

func (h *API) BridgeStatus(c *gin.Context) {
	c.JSON(http.StatusOK, h.Repo.BridgeStatus(c.Request.Context()))
}

func (h *API) Sync(c *gin.Context) {
	h.recordAudit(c, "BridgeSync", "POST /bridge/sync")
	var res bridge.Result
	var err error
	if h.Bridge != nil {
		res, err = h.Bridge.Sync(c.Request.Context())
	} else {
		_ = h.Repo.AppendBridgeSyncLog(c.Request.Context(), "Bridge service not configured")
		res.Status = "unconfigured"
	}
	if err != nil {
		apierr.JSONStatus(c, http.StatusBadGateway, "dms sync failed")
		return
	}
	if h.Events != nil {
		h.Events.PublishCommercial(c.Request.Context(), events.TypeBridgeSynced, map[string]any{
			"trigger":          "user",
			"user":             auth.ActorName(c),
			"outlets_fetched":  res.OutletsFetched,
			"outlets_upserted": res.OutletsUpserted,
			"pending_created":  res.PendingCreated,
		}, "bridge")
	}
	c.JSON(http.StatusOK, res)
}
