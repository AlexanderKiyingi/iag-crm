package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/iag/crm/backend/internal/middleware"
	"github.com/iag/crm/backend/internal/models"
	"github.com/iag/crm/backend/internal/store"
)

func (h *API) sessionFromContext(c *gin.Context) map[string]any {
	role := "md"
	email := "dev@iag.local"
	name := "Developer"
	perms := []string{}
	if claims, ok := middleware.Claims(c); ok && claims != nil {
		email = claims.Email
		name = claims.Name
		if name == "" {
			name = email
		}
		role = models.RoleFromGroups(claims.Groups, claims.IsSuperuser)
		perms = claims.Permissions
	}
	spec, _ := models.Roles[role]
	return map[string]any{
		"email":       email,
		"name":        name,
		"role":        role,
		"role_label":  spec.Label,
		"role_full":   spec.Full,
		"pages":       spec.Pages,
		"modals":      spec.Modals,
		"permissions": perms,
	}
}

func (h *API) Bootstrap(c *gin.Context) {
	sess := h.sessionFromContext(c)
	pc := h.permissionContext(c)
	c.JSON(http.StatusOK, gin.H{
		"service":      h.Cfg.ServiceName,
		"api_prefix":   h.Cfg.GatewayAPIPrefix + "/v1",
		"public_api":   h.Cfg.PublicAPIURL,
		"session":      sess,
		"permissions":  pc,
		"page_titles":  models.PageTitles,
		"roles":        models.Roles,
		"sync_status":  "connected",
		"modules":      []string{"crm", "dms", "erp", "lims", "wms", "scm"},
	})
}

func (h *API) Session(c *gin.Context) {
	c.JSON(http.StatusOK, h.sessionFromContext(c))
}

func (h *API) PermissionsCatalog(c *gin.Context) {
	c.JSON(http.StatusOK, models.PermissionCatalogData())
}

func (h *API) Lookups(c *gin.Context) {
	items, err := h.Repo.Lookups(c.Request.Context(), c.Param("kind"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func genericList(c *gin.Context, fn func(ctx *gin.Context, opts store.ListOpts) ([]map[string]any, int, error)) {
	opts := scopedListOpts(c)
	items, total, err := fn(c, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}
	paginated(c, items, total)
}

func genericCreate(c *gin.Context, fn func(ctx *gin.Context, in map[string]any) (map[string]any, error)) {
	var in map[string]any
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid body")
		return
	}
	item, err := fn(c, in)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create failed"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *API) MarketingHub(c *gin.Context) {
	summary, _ := h.Repo.MarketingHubSummary(c.Request.Context())
	c.JSON(http.StatusOK, summary)
}

func (h *API) ListSegments(c *gin.Context) {
	genericList(c, func(ctx *gin.Context, opts store.ListOpts) ([]map[string]any, int, error) {
		return h.Repo.ListSegments(ctx.Request.Context(), opts)
	})
}

func (h *API) CreateSegment(c *gin.Context) {
	genericCreate(c, func(ctx *gin.Context, in map[string]any) (map[string]any, error) {
		return h.Repo.CreateSegment(ctx.Request.Context(), in)
	})
}

func (h *API) ListJourneys(c *gin.Context) {
	genericList(c, func(ctx *gin.Context, opts store.ListOpts) ([]map[string]any, int, error) {
		return h.Repo.ListJourneys(ctx.Request.Context(), opts)
	})
}

func (h *API) CreateJourney(c *gin.Context) {
	genericCreate(c, func(ctx *gin.Context, in map[string]any) (map[string]any, error) {
		return h.Repo.CreateJourney(ctx.Request.Context(), in)
	})
}

func (h *API) ListPersonas(c *gin.Context) {
	genericList(c, func(ctx *gin.Context, opts store.ListOpts) ([]map[string]any, int, error) {
		return h.Repo.ListPersonas(ctx.Request.Context(), opts)
	})
}

func (h *API) CreatePersona(c *gin.Context) {
	genericCreate(c, func(ctx *gin.Context, in map[string]any) (map[string]any, error) {
		return h.Repo.CreatePersona(ctx.Request.Context(), in)
	})
}

func (h *API) ListEvents(c *gin.Context) {
	genericList(c, func(ctx *gin.Context, opts store.ListOpts) ([]map[string]any, int, error) {
		return h.Repo.ListEvents(ctx.Request.Context(), opts)
	})
}

func (h *API) CreateEvent(c *gin.Context) {
	genericCreate(c, func(ctx *gin.Context, in map[string]any) (map[string]any, error) {
		return h.Repo.CreateEvent(ctx.Request.Context(), in)
	})
}

func (h *API) ListContentAssets(c *gin.Context) {
	genericList(c, func(ctx *gin.Context, opts store.ListOpts) ([]map[string]any, int, error) {
		return h.Repo.ListContentAssets(ctx.Request.Context(), opts)
	})
}

func (h *API) CreateContentAsset(c *gin.Context) {
	genericCreate(c, func(ctx *gin.Context, in map[string]any) (map[string]any, error) {
		return h.Repo.CreateContentAsset(ctx.Request.Context(), in)
	})
}

func (h *API) ListEmailSends(c *gin.Context) {
	genericList(c, func(ctx *gin.Context, opts store.ListOpts) ([]map[string]any, int, error) {
		return h.Repo.ListEmailSends(ctx.Request.Context(), opts)
	})
}

func (h *API) CreateEmailSend(c *gin.Context) {
	genericCreate(c, func(ctx *gin.Context, in map[string]any) (map[string]any, error) {
		return h.Repo.CreateEmailSend(ctx.Request.Context(), in)
	})
}

func (h *API) ListSocialPosts(c *gin.Context) {
	genericList(c, func(ctx *gin.Context, opts store.ListOpts) ([]map[string]any, int, error) {
		return h.Repo.ListSocialPosts(ctx.Request.Context(), opts)
	})
}

func (h *API) CreateSocialPost(c *gin.Context) {
	genericCreate(c, func(ctx *gin.Context, in map[string]any) (map[string]any, error) {
		return h.Repo.CreateSocialPost(ctx.Request.Context(), in)
	})
}

func (h *API) ListSEOKeywords(c *gin.Context) {
	genericList(c, func(ctx *gin.Context, opts store.ListOpts) ([]map[string]any, int, error) {
		return h.Repo.ListSEOKeywords(ctx.Request.Context(), opts)
	})
}

func (h *API) CreateSEOKeyword(c *gin.Context) {
	genericCreate(c, func(ctx *gin.Context, in map[string]any) (map[string]any, error) {
		return h.Repo.CreateSEOKeyword(ctx.Request.Context(), in)
	})
}

func (h *API) ListBudgetPlans(c *gin.Context) {
	genericList(c, func(ctx *gin.Context, opts store.ListOpts) ([]map[string]any, int, error) {
		return h.Repo.ListBudgetPlans(ctx.Request.Context(), opts)
	})
}

func (h *API) CreateBudgetPlan(c *gin.Context) {
	genericCreate(c, func(ctx *gin.Context, in map[string]any) (map[string]any, error) {
		return h.Repo.CreateBudgetPlan(ctx.Request.Context(), in)
	})
}

func (h *API) ListMQLs(c *gin.Context) {
	genericList(c, func(ctx *gin.Context, opts store.ListOpts) ([]map[string]any, int, error) {
		return h.Repo.ListMQLs(ctx.Request.Context(), opts)
	})
}

func (h *API) GetBrandKit(c *gin.Context) {
	kit, err := h.Repo.GetBrandKit(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "brand kit failed"})
		return
	}
	c.JSON(http.StatusOK, kit)
}

func (h *API) DemandGenMetrics(c *gin.Context) {
	m, _ := h.Repo.DemandGenMetrics(c.Request.Context())
	c.JSON(http.StatusOK, m)
}

func (h *API) BridgeStreams(c *gin.Context) {
	streams, err := h.Repo.ListBridgeStreams(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streams failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"streams": streams})
}

func (h *API) BridgePendingImports(c *gin.Context) {
	items, total, err := h.Repo.ListPendingImports(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "imports failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "meta": gin.H{"total": total}})
}

func (h *API) BridgeFieldMappings(c *gin.Context) {
	items, err := h.Repo.ListFieldMappings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mappings failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *API) BridgeSyncLog(c *gin.Context) {
	items, err := h.Repo.ListSyncLog(c.Request.Context(), 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sync log failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *API) ListOutlets(c *gin.Context) {
	genericList(c, func(ctx *gin.Context, opts store.ListOpts) ([]map[string]any, int, error) {
		return h.Repo.ListOutlets(ctx.Request.Context(), opts)
	})
}

func (h *API) GetOutlet360(c *gin.Context) {
	item, err := h.Repo.GetOutlet360(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) ListExportCustomers(c *gin.Context) {
	genericList(c, func(ctx *gin.Context, opts store.ListOpts) ([]map[string]any, int, error) {
		return h.Repo.ListExportCustomers(ctx.Request.Context(), opts)
	})
}

func (h *API) CreateExportCustomer(c *gin.Context) {
	genericCreate(c, func(ctx *gin.Context, in map[string]any) (map[string]any, error) {
		return h.Repo.CreateExportCustomer(ctx.Request.Context(), in)
	})
}

func (h *API) ListLoyaltyTiers(c *gin.Context) {
	items, err := h.Repo.ListLoyaltyTiers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tiers failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *API) ListLoyaltyOutlets(c *gin.Context) {
	genericList(c, func(ctx *gin.Context, opts store.ListOpts) ([]map[string]any, int, error) {
		return h.Repo.ListLoyaltyOutlets(ctx.Request.Context(), opts)
	})
}

func (h *API) CreateLoyaltyPromotion(c *gin.Context) {
	genericCreate(c, func(ctx *gin.Context, in map[string]any) (map[string]any, error) {
		return h.Repo.CreateLoyaltyPromotion(ctx.Request.Context(), in)
	})
}

func (h *API) InsightsSummary(c *gin.Context) {
	s, _ := h.Repo.InsightsSummary(c.Request.Context())
	c.JSON(http.StatusOK, s)
}

func (h *API) AISuggestions(c *gin.Context) {
	items, _ := h.Repo.AISuggestions(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"suggestions": items})
}

func (h *API) AICopilotChat(c *gin.Context) {
	var in struct {
		Message string `json:"message"`
	}
	_ = c.ShouldBindJSON(&in)
	c.JSON(http.StatusOK, gin.H{
		"reply": "Based on your pipeline, Matsuri Q3 ($280K) and Amsterdam dual-quarter ($340K) are the highest-impact deals this week. Seoul Bean Lab viewed quote QTE-0421 four times — consider a follow-up call.",
		"actions": []map[string]any{
			{"label": "Open deal DEAL-0421", "page": "deals"},
			{"label": "View quote QTE-0421", "page": "quotes"},
		},
	})
}

func (h *API) GetDealForecast(c *gin.Context) {
	summary, _ := h.Repo.PipelineSummary(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"quarter": c.DefaultQuery("quarter", "Q2"), "summary": summary})
}

func (h *API) GetAccount360(c *gin.Context) {
	acct, err := h.Repo.GetAccount(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	deals, _, _ := h.Repo.ListDeals(c.Request.Context(), store.ListOpts{Limit: 10, Search: acct.Name})
	c.JSON(http.StatusOK, gin.H{"account": acct, "deals": deals})
}
