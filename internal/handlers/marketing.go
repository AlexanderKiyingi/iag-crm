package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/iag/crm/backend/internal/middleware"
	"github.com/iag/crm/backend/internal/models"
	"github.com/iag/crm/backend/internal/store"
	"github.com/alvor-technologies/iag-platform-go/apierr"
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
		apierr.JSONStatus(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func genericList(c *gin.Context, fn func(ctx *gin.Context, opts store.ListOpts) ([]map[string]any, int, error)) {
	opts := scopedListOpts(c)
	items, total, err := fn(c, opts)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list failed")
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
		apierr.JSONStatus(c, http.StatusInternalServerError, "create failed")
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
		apierr.JSONStatus(c, http.StatusInternalServerError, "brand kit failed")
		return
	}
	c.JSON(http.StatusOK, kit)
}

func (h *API) DemandGenMetrics(c *gin.Context) {
	m, err := h.Repo.DemandGenMetrics(c.Request.Context())
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "demand gen metrics failed")
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *API) BridgeStreams(c *gin.Context) {
	streams, err := h.Repo.ListBridgeStreams(c.Request.Context())
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "streams failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"streams": streams})
}

func (h *API) BridgePendingImports(c *gin.Context) {
	items, total, err := h.Repo.ListPendingImports(c.Request.Context())
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "imports failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "meta": gin.H{"total": total}})
}

func (h *API) BridgeFieldMappings(c *gin.Context) {
	items, err := h.Repo.ListFieldMappings(c.Request.Context())
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "mappings failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *API) BridgeSyncLog(c *gin.Context) {
	items, err := h.Repo.ListSyncLog(c.Request.Context(), 50)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "sync log failed")
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
		apierr.JSONStatus(c, http.StatusInternalServerError, "tiers failed")
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
	s, err := h.Repo.InsightsSummary(c.Request.Context())
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "insights summary failed")
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *API) AISuggestions(c *gin.Context) {
	items, err := h.Repo.AISuggestions(c.Request.Context())
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "ai suggestions failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"suggestions": items})
}

func (h *API) AICopilotChat(c *gin.Context) {
	var in struct {
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid body")
		return
	}
	in.Message = strings.TrimSpace(in.Message)
	if in.Message == "" {
		badRequest(c, "message is required")
		return
	}

	suggestions, _ := h.Repo.AISuggestions(c.Request.Context())
	actions := []map[string]any{}
	for _, s := range suggestions {
		if id, ok := s["deal_id"].(string); ok {
			actions = append(actions, map[string]any{"label": s["title"], "page": "deals", "id": id})
		}
	}
	if len(actions) == 0 {
		actions = append(actions, map[string]any{"label": "Open pipeline", "page": "deals"})
	}

	// Deterministic fallback used when the shared AI platform is not configured
	// or the call fails, so the copilot always returns something useful.
	reply := "Review your top negotiation-stage deals for follow-ups this week."
	if h.AI != nil && h.AI.Enabled() {
		if answer, err := h.AI.Complete(c.Request.Context(), copilotSystemPrompt(suggestions), in.Message, 512); err != nil {
			slog.Warn("ai copilot completion failed; using fallback reply", "err", err)
		} else if answer != "" {
			reply = answer
		}
	}
	c.JSON(http.StatusOK, gin.H{"reply": reply, "actions": actions})
}

// copilotSystemPrompt grounds the model in the caller's live CRM context so the
// reply references real pipeline data rather than generic advice.
func copilotSystemPrompt(suggestions []map[string]any) string {
	var b strings.Builder
	b.WriteString("You are the IAG CRM sales copilot. Answer concisely (2-4 sentences) and be action-oriented for a B2B sales team. ")
	if len(suggestions) > 0 {
		b.WriteString("Current pipeline signals you may reference:\n")
		for i, s := range suggestions {
			if i >= 8 {
				break
			}
			title, _ := s["title"].(string)
			if title != "" {
				b.WriteString("- " + title + "\n")
			}
		}
	}
	return b.String()
}

func genericGet(c *gin.Context, fn func(ctx *gin.Context, id string) (map[string]any, error)) {
	item, err := fn(c, c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	c.JSON(http.StatusOK, item)
}

func genericPatch(c *gin.Context, fn func(ctx *gin.Context, id string, patch map[string]any) (map[string]any, error)) {
	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		badRequest(c, "invalid body")
		return
	}
	item, err := fn(c, c.Param("id"), patch)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "update failed")
		return
	}
	c.JSON(http.StatusOK, item)
}

func genericDelete(c *gin.Context, fn func(ctx *gin.Context, id string) error) {
	if err := fn(c, c.Param("id")); err != nil {
		notFound(c)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *API) GetSegment(c *gin.Context) {
	genericGet(c, func(ctx *gin.Context, id string) (map[string]any, error) {
		return h.Repo.GetGenericRow(ctx.Request.Context(), "crm_segments", "name, kind, refresh, rules, member_count", id)
	})
}
func (h *API) PatchSegment(c *gin.Context) {
	genericPatch(c, func(ctx *gin.Context, id string, patch map[string]any) (map[string]any, error) {
		return h.Repo.PatchGenericRow(ctx.Request.Context(), "crm_segments", id, patch, map[string]string{
			"name": "name", "kind": "kind", "refresh": "refresh", "rules": "rules",
		})
	})
}
func (h *API) DeleteSegment(c *gin.Context) {
	genericDelete(c, func(ctx *gin.Context, id string) error {
		return h.Repo.DeleteGenericRow(ctx.Request.Context(), "crm_segments", id)
	})
}

func (h *API) GetJourney(c *gin.Context) {
	genericGet(c, func(ctx *gin.Context, id string) (map[string]any, error) {
		return h.Repo.GetGenericRow(ctx.Request.Context(), "crm_journeys", "name, trigger, template, goal, status, enrolled, conversion", id)
	})
}
func (h *API) PatchJourney(c *gin.Context) {
	genericPatch(c, func(ctx *gin.Context, id string, patch map[string]any) (map[string]any, error) {
		return h.Repo.PatchGenericRow(ctx.Request.Context(), "crm_journeys", id, patch, map[string]string{
			"name": "name", "trigger": "trigger", "template": "template", "goal": "goal", "status": "status",
		})
	})
}
func (h *API) DeleteJourney(c *gin.Context) {
	genericDelete(c, func(ctx *gin.Context, id string) error {
		return h.Repo.DeleteGenericRow(ctx.Request.Context(), "crm_journeys", id)
	})
}

func (h *API) GetPersona(c *gin.Context) {
	genericGet(c, func(ctx *gin.Context, id string) (map[string]any, error) {
		return h.Repo.GetGenericRow(ctx.Request.Context(), "crm_personas", "name, buyer_role, region, seniority, content_tags, story", id)
	})
}
func (h *API) PatchPersona(c *gin.Context) {
	genericPatch(c, func(ctx *gin.Context, id string, patch map[string]any) (map[string]any, error) {
		return h.Repo.PatchGenericRow(ctx.Request.Context(), "crm_personas", id, patch, map[string]string{
			"name": "name", "buyer_role": "buyer_role", "region": "region", "story": "story",
		})
	})
}
func (h *API) DeletePersona(c *gin.Context) {
	genericDelete(c, func(ctx *gin.Context, id string) error {
		return h.Repo.DeleteGenericRow(ctx.Request.Context(), "crm_personas", id)
	})
}

func (h *API) GetMarketingEvent(c *gin.Context) {
	genericGet(c, func(ctx *gin.Context, id string) (map[string]any, error) {
		return h.Repo.GetGenericRow(ctx.Request.Context(), "crm_events", "name, event_type, city, starts_on, ends_on, budget_usd, mql_target, registrations, status", id)
	})
}
func (h *API) PatchMarketingEvent(c *gin.Context) {
	genericPatch(c, func(ctx *gin.Context, id string, patch map[string]any) (map[string]any, error) {
		return h.Repo.PatchGenericRow(ctx.Request.Context(), "crm_events", id, patch, map[string]string{
			"name": "name", "type": "event_type", "city": "city", "status": "status",
		})
	})
}
func (h *API) DeleteMarketingEvent(c *gin.Context) {
	genericDelete(c, func(ctx *gin.Context, id string) error {
		return h.Repo.DeleteGenericRow(ctx.Request.Context(), "crm_events", id)
	})
}

func (h *API) GetContentAsset(c *gin.Context) {
	genericGet(c, func(ctx *gin.Context, id string) (map[string]any, error) {
		return h.Repo.GetGenericRow(ctx.Request.Context(), "crm_content_assets", "name, asset_type, format, tags, status, usage_count", id)
	})
}
func (h *API) PatchContentAsset(c *gin.Context) {
	genericPatch(c, func(ctx *gin.Context, id string, patch map[string]any) (map[string]any, error) {
		return h.Repo.PatchGenericRow(ctx.Request.Context(), "crm_content_assets", id, patch, map[string]string{
			"name": "name", "type": "asset_type", "format": "format", "tags": "tags", "status": "status",
		})
	})
}
func (h *API) DeleteContentAsset(c *gin.Context) {
	genericDelete(c, func(ctx *gin.Context, id string) error {
		return h.Repo.DeleteGenericRow(ctx.Request.Context(), "crm_content_assets", id)
	})
}

func (h *API) GetEmailSend(c *gin.Context) {
	genericGet(c, func(ctx *gin.Context, id string) (map[string]any, error) {
		return h.Repo.GetGenericRow(ctx.Request.Context(), "crm_email_sends", "subject, template, status, scheduled_at, open_rate", id)
	})
}
func (h *API) PatchEmailSend(c *gin.Context) {
	genericPatch(c, func(ctx *gin.Context, id string, patch map[string]any) (map[string]any, error) {
		return h.Repo.PatchGenericRow(ctx.Request.Context(), "crm_email_sends", id, patch, map[string]string{
			"subject": "subject", "template": "template", "status": "status", "body": "body",
		})
	})
}
func (h *API) DeleteEmailSend(c *gin.Context) {
	genericDelete(c, func(ctx *gin.Context, id string) error {
		return h.Repo.DeleteGenericRow(ctx.Request.Context(), "crm_email_sends", id)
	})
}

func (h *API) GetSocialPost(c *gin.Context) {
	genericGet(c, func(ctx *gin.Context, id string) (map[string]any, error) {
		return h.Repo.GetGenericRow(ctx.Request.Context(), "crm_social_posts", "platforms, content, status, scheduled_at", id)
	})
}
func (h *API) PatchSocialPost(c *gin.Context) {
	genericPatch(c, func(ctx *gin.Context, id string, patch map[string]any) (map[string]any, error) {
		return h.Repo.PatchGenericRow(ctx.Request.Context(), "crm_social_posts", id, patch, map[string]string{
			"platforms": "platforms", "content": "content", "status": "status",
		})
	})
}
func (h *API) DeleteSocialPost(c *gin.Context) {
	genericDelete(c, func(ctx *gin.Context, id string) error {
		return h.Repo.DeleteGenericRow(ctx.Request.Context(), "crm_social_posts", id)
	})
}

func (h *API) GetSEOKeyword(c *gin.Context) {
	genericGet(c, func(ctx *gin.Context, id string) (map[string]any, error) {
		return h.Repo.GetGenericRow(ctx.Request.Context(), "crm_seo_keywords", "term, intent, locale, landing_page, rank", id)
	})
}
func (h *API) PatchSEOKeyword(c *gin.Context) {
	genericPatch(c, func(ctx *gin.Context, id string, patch map[string]any) (map[string]any, error) {
		return h.Repo.PatchGenericRow(ctx.Request.Context(), "crm_seo_keywords", id, patch, map[string]string{
			"term": "term", "intent": "intent", "locale": "locale", "landing_page": "landing_page",
		})
	})
}
func (h *API) DeleteSEOKeyword(c *gin.Context) {
	genericDelete(c, func(ctx *gin.Context, id string) error {
		return h.Repo.DeleteGenericRow(ctx.Request.Context(), "crm_seo_keywords", id)
	})
}

func (h *API) GetBudgetPlan(c *gin.Context) {
	genericGet(c, func(ctx *gin.Context, id string) (map[string]any, error) {
		return h.Repo.GetGenericRow(ctx.Request.Context(), "crm_budget_plans", "name, quarter, owner, channels, mql_target, sql_target, won_target", id)
	})
}
func (h *API) PatchBudgetPlan(c *gin.Context) {
	genericPatch(c, func(ctx *gin.Context, id string, patch map[string]any) (map[string]any, error) {
		return h.Repo.PatchGenericRow(ctx.Request.Context(), "crm_budget_plans", id, patch, map[string]string{
			"name": "name", "quarter": "quarter", "owner": "owner",
		})
	})
}
func (h *API) DeleteBudgetPlan(c *gin.Context) {
	genericDelete(c, func(ctx *gin.Context, id string) error {
		return h.Repo.DeleteGenericRow(ctx.Request.Context(), "crm_budget_plans", id)
	})
}

func (h *API) GetSEOAuditJob(c *gin.Context) {
	job, err := h.Repo.GetSEOAuditJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	c.JSON(http.StatusOK, job)
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
	if !enforceOwner(c, acct.Owner) {
		return
	}
	deals, _, _ := h.Repo.ListDeals(c.Request.Context(), store.ListOpts{Limit: 10, Search: acct.Name})
	contacts, _, _ := h.Repo.ListContacts(c.Request.Context(), store.ListOpts{Limit: 10, Search: acct.Name})
	tickets, _, _ := h.Repo.ListTickets(c.Request.Context(), store.ListOpts{Limit: 5})
	out := gin.H{"account": acct, "deals": deals, "contacts": contacts, "tickets": tickets}
	customerRef := acct.FinanceCustomerRef
	if customerRef == "" && acct.Name != "" {
		customerRef = acct.Name
	}
	if h.Finance != nil && h.Finance.Enabled() && customerRef != "" {
		if stmt, err := h.Finance.CustomerStatement(c.Request.Context(), customerRef); err == nil {
			out["finance"] = stmt
		}
	}
	c.JSON(http.StatusOK, out)
}
