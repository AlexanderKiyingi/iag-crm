package router

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/iag/crm/backend/internal/auth"
	"github.com/iag/crm/backend/internal/config"
	"github.com/iag/crm/backend/internal/handlers"
	"github.com/iag/crm/backend/internal/middleware"
	"github.com/iag/crm/backend/internal/store"
)

type Options struct {
	Cfg          config.Config
	PlatformAuth *middleware.PlatformAuth
	Repo         *store.Repository
	API          *handlers.API
}

func New(opts Options) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(otelgin.Middleware(opts.Cfg.ServiceName))
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(corsMiddleware(opts.Cfg.CORSOrigin))
	r.Use(securityHeaders())

	api := opts.API
	if api == nil {
		api = &handlers.API{Repo: opts.Repo, Cfg: opts.Cfg}
	}

	r.GET("/healthz", api.Health)
	r.GET("/health", api.Health)
	r.GET("/ready", api.Ready)
	r.GET("/v1/health", api.Health)
	r.GET("/v1/health/live", api.Health)
	r.GET("/v1/health/ready", api.Ready)

	v1 := r.Group("/v1")
	if opts.PlatformAuth != nil {
		v1.Use(opts.PlatformAuth.AttachPrincipal())
	}
	v1.Use(middleware.RequestAudit(opts.Repo))

	registerPlatformRoutes(v1, api)
	registerDashboardRoutes(v1, api)
	registerModuleRecordRoutes(v1, api)
	registerSalesRoutes(v1, api)
	registerMarketingRoutes(v1, api)
	registerEngagementRoutes(v1, api)
	registerBridgeRoutes(v1, api)
	registerIntelligenceRoutes(v1, api)
	registerAuditRoutes(v1, api)
	registerAdminRoutes(v1, api)
	registerIntegrationRoutes(v1, api)

	return r
}

func registerPlatformRoutes(v1 *gin.RouterGroup, api *handlers.API) {
	v1.GET("/bootstrap", auth.RequirePerm("accounts.read"), api.Bootstrap)
	v1.GET("/auth/session", auth.RequirePerm("accounts.read"), api.Session)
	v1.GET("/permissions/catalog", auth.RequirePerm("accounts.read"), api.PermissionsCatalog)
	v1.GET("/permissions/builtin", auth.RequirePerm("accounts.read"), api.PermissionsBuiltin)
	v1.POST("/permissions/check", auth.RequirePerm("accounts.read"), api.PermissionsCheck)
	v1.GET("/permissions/me", auth.RequirePerm("accounts.read"), api.PermissionsMeEnhanced)
	v1.GET("/lookups/:kind", auth.RequirePerm("accounts.read"), api.Lookups)
	v1.GET("/search", auth.RequirePerm("accounts.read"), api.Search)
	v1.GET("/notifications", auth.RequirePerm("accounts.read"), api.Notifications)
	v1.GET("/platform/status", auth.RequireStaff(), api.PlatformStatus)
}

func registerDashboardRoutes(v1 *gin.RouterGroup, api *handlers.API) {
	v1.GET("/overview", auth.RequirePerm("accounts.read"), api.Overview)
	v1.GET("/pipeline/board", auth.RequirePerm("deals.read"), api.PipelineBoard)
	v1.GET("/deals/forecast", auth.RequirePerm("deals.read"), api.GetDealForecast)
}

// registerModuleRecordRoutes exposes generic CRUD for CRM-owned lightweight
// modules (products, services, solutions, projects, documents, …), keyed by the
// :module path segment. Financial/procurement modules are deliberately excluded.
func registerModuleRecordRoutes(v1 *gin.RouterGroup, api *handlers.API) {
	v1.GET("/modules/:module/records", auth.RequirePerm("records.read"), api.ModuleRecordsList)
	v1.POST("/modules/:module/records", auth.RequirePerm("records.create"), api.ModuleRecordCreate)
	v1.GET("/modules/:module/records/:id", auth.RequirePerm("records.read"), api.ModuleRecordGet)
	v1.PATCH("/modules/:module/records/:id", auth.RequirePerm("records.update"), api.ModuleRecordPatch)
	v1.DELETE("/modules/:module/records/:id", auth.RequirePerm("records.delete"), api.ModuleRecordDelete)
}

func registerSalesRoutes(v1 *gin.RouterGroup, api *handlers.API) {
	v1.GET("/accounts", auth.RequirePerm("accounts.read"), api.ListAccounts)
	v1.POST("/accounts", auth.RequirePerm("accounts.create"), api.CreateAccount)
	v1.GET("/accounts/:id", auth.RequirePerm("accounts.read"), api.GetAccount)
	v1.GET("/accounts/:id/360", auth.RequirePerm("accounts.read"), api.GetAccount360)
	v1.PATCH("/accounts/:id", auth.RequirePerm("accounts.update"), api.PatchAccount)
	v1.DELETE("/accounts/:id", auth.RequirePerm("accounts.delete"), api.DeleteAccount)

	v1.GET("/contacts", auth.RequirePerm("contacts.read"), api.ListContacts)
	v1.POST("/contacts", auth.RequirePerm("contacts.create"), api.CreateContact)
	v1.GET("/contacts/:id", auth.RequirePerm("contacts.read"), api.GetContact)
	v1.PATCH("/contacts/:id", auth.RequirePerm("contacts.update"), api.PatchContact)
	v1.DELETE("/contacts/:id", auth.RequirePerm("contacts.delete"), api.DeleteContact)

	v1.GET("/leads", auth.RequirePerm("leads.read"), api.ListLeads)
	v1.POST("/leads", auth.RequirePerm("leads.create"), api.CreateLead)
	v1.GET("/leads/:id", auth.RequirePerm("leads.read"), api.GetLead)
	v1.PATCH("/leads/:id", auth.RequirePerm("leads.update"), api.PatchLead)
	v1.POST("/leads/:id/convert", auth.RequirePerm("leads.update"), api.ConvertLead)

	v1.GET("/deals", auth.RequirePerm("deals.read"), api.ListDeals)
	v1.POST("/deals", auth.RequirePerm("deals.create"), api.CreateDeal)
	v1.GET("/deals/:id", auth.RequirePerm("deals.read"), api.GetDeal)
	v1.PATCH("/deals/:id", auth.RequirePerm("deals.update"), api.PatchDeal)
	v1.PATCH("/deals/:id/stage", auth.RequirePerm("deals.update"), api.SetDealStage)
	v1.POST("/deals/:id/won", auth.RequirePerm("deals.update"), api.MarkDealWon)
	v1.DELETE("/deals/:id", auth.RequirePerm("deals.delete"), api.DeleteDeal)

	v1.GET("/quotes", auth.RequirePerm("quotes.read"), api.ListQuotes)
	v1.POST("/quotes", auth.RequirePerm("quotes.create"), api.CreateQuote)
	v1.GET("/quotes/:id", auth.RequirePerm("quotes.read"), api.GetQuote)
	v1.PATCH("/quotes/:id", auth.RequirePerm("quotes.update"), api.PatchQuote)
	v1.POST("/quotes/:id/send", auth.RequirePerm("quotes.update"), api.SendQuote)
	v1.POST("/quotes/:id/sign", auth.RequirePerm("quotes.update"), api.SignQuote)
	v1.DELETE("/quotes/:id", auth.RequirePerm("quotes.delete"), api.DeleteQuote)

	v1.DELETE("/leads/:id", auth.RequirePerm("leads.delete"), api.DeleteLead)
}

func registerMarketingRoutes(v1 *gin.RouterGroup, api *handlers.API) {
	v1.GET("/marketing/hub", auth.RequirePerm("campaigns.read"), api.MarketingHub)
	v1.GET("/marketing/demand-gen", auth.RequirePerm("campaigns.read"), api.DemandGenMetrics)
	v1.GET("/segments", auth.RequirePerm("segments.read"), api.ListSegments)
	v1.POST("/segments", auth.RequirePerm("segments.create"), api.CreateSegment)
	v1.GET("/segments/:id", auth.RequirePerm("segments.read"), api.GetSegment)
	v1.PATCH("/segments/:id", auth.RequirePerm("segments.update"), api.PatchSegment)
	v1.DELETE("/segments/:id", auth.RequirePerm("segments.delete"), api.DeleteSegment)
	v1.GET("/journeys", auth.RequirePerm("journeys.read"), api.ListJourneys)
	v1.POST("/journeys", auth.RequirePerm("journeys.create"), api.CreateJourney)
	v1.GET("/journeys/:id/steps", auth.RequirePerm("journeys.read"), api.ListJourneySteps)
	v1.POST("/journeys/:id/steps", auth.RequirePerm("journeys.update"), api.CreateJourneyStep)
	v1.POST("/journeys/:id/enroll", auth.RequirePerm("journeys.update"), api.EnrollJourney)
	v1.POST("/journeys/:id/activate", auth.RequirePerm("journeys.update"), api.ActivateJourney)
	v1.GET("/journeys/:id/enrollments", auth.RequirePerm("journeys.read"), api.ListJourneyEnrollments)
	v1.GET("/journeys/:id", auth.RequirePerm("journeys.read"), api.GetJourney)
	v1.PATCH("/journeys/:id", auth.RequirePerm("journeys.update"), api.PatchJourney)
	v1.DELETE("/journeys/:id", auth.RequirePerm("journeys.delete"), api.DeleteJourney)
	v1.GET("/personas", auth.RequirePerm("personas.read"), api.ListPersonas)
	v1.POST("/personas", auth.RequirePerm("personas.create"), api.CreatePersona)
	v1.GET("/personas/:id", auth.RequirePerm("personas.read"), api.GetPersona)
	v1.PATCH("/personas/:id", auth.RequirePerm("personas.update"), api.PatchPersona)
	v1.DELETE("/personas/:id", auth.RequirePerm("personas.delete"), api.DeletePersona)
	v1.GET("/events", auth.RequirePerm("events.read"), api.ListEvents)
	v1.POST("/events", auth.RequirePerm("events.create"), api.CreateEvent)
	v1.GET("/events/:id", auth.RequirePerm("events.read"), api.GetMarketingEvent)
	v1.PATCH("/events/:id", auth.RequirePerm("events.update"), api.PatchMarketingEvent)
	v1.DELETE("/events/:id", auth.RequirePerm("events.delete"), api.DeleteMarketingEvent)
	v1.GET("/content/assets", auth.RequirePerm("content.read"), api.ListContentAssets)
	v1.POST("/content/assets", auth.RequirePerm("content.create"), api.CreateContentAsset)
	v1.GET("/content/assets/:id", auth.RequirePerm("content.read"), api.GetContentAsset)
	v1.PATCH("/content/assets/:id", auth.RequirePerm("content.update"), api.PatchContentAsset)
	v1.DELETE("/content/assets/:id", auth.RequirePerm("content.delete"), api.DeleteContentAsset)
	v1.GET("/email/sends", auth.RequirePerm("email.read"), api.ListEmailSends)
	v1.POST("/email/sends", auth.RequirePerm("email.create"), api.CreateEmailSend)
	v1.GET("/email/sends/:id", auth.RequirePerm("email.read"), api.GetEmailSend)
	v1.PATCH("/email/sends/:id", auth.RequirePerm("email.update"), api.PatchEmailSend)
	v1.DELETE("/email/sends/:id", auth.RequirePerm("email.delete"), api.DeleteEmailSend)
	v1.GET("/social/posts", auth.RequirePerm("social.read"), api.ListSocialPosts)
	v1.POST("/social/posts", auth.RequirePerm("social.create"), api.CreateSocialPost)
	v1.GET("/social/posts/:id", auth.RequirePerm("social.read"), api.GetSocialPost)
	v1.PATCH("/social/posts/:id", auth.RequirePerm("social.update"), api.PatchSocialPost)
	v1.DELETE("/social/posts/:id", auth.RequirePerm("social.delete"), api.DeleteSocialPost)
	v1.GET("/seo/keywords", auth.RequirePerm("seo.read"), api.ListSEOKeywords)
	v1.POST("/seo/keywords", auth.RequirePerm("seo.create"), api.CreateSEOKeyword)
	v1.GET("/seo/keywords/:id", auth.RequirePerm("seo.read"), api.GetSEOKeyword)
	v1.PATCH("/seo/keywords/:id", auth.RequirePerm("seo.update"), api.PatchSEOKeyword)
	v1.DELETE("/seo/keywords/:id", auth.RequirePerm("seo.delete"), api.DeleteSEOKeyword)
	v1.POST("/seo/audit", auth.RequirePerm("seo.update"), api.PostSEOAudit)
	v1.GET("/seo/audit/:id", auth.RequirePerm("seo.read"), api.GetSEOAuditJob)
	v1.GET("/marketing/budget", auth.RequirePerm("budget.read"), api.ListBudgetPlans)
	v1.POST("/marketing/budget", auth.RequirePerm("budget.create"), api.CreateBudgetPlan)
	v1.GET("/marketing/budget/:id", auth.RequirePerm("budget.read"), api.GetBudgetPlan)
	v1.PATCH("/marketing/budget/:id", auth.RequirePerm("budget.update"), api.PatchBudgetPlan)
	v1.DELETE("/marketing/budget/:id", auth.RequirePerm("budget.delete"), api.DeleteBudgetPlan)
	v1.GET("/mqls", auth.RequirePerm("mqls.read"), api.ListMQLs)
	v1.GET("/brand-kit", auth.RequirePerm("content.read"), api.GetBrandKit)
	v1.GET("/campaigns", auth.RequirePerm("campaigns.read"), api.ListCampaigns)
	v1.POST("/campaigns", auth.RequirePerm("campaigns.create"), api.CreateCampaign)
	v1.GET("/campaigns/:id", auth.RequirePerm("campaigns.read"), api.GetCampaign)
	v1.PATCH("/campaigns/:id", auth.RequirePerm("campaigns.update"), api.PatchCampaign)
	v1.DELETE("/campaigns/:id", auth.RequirePerm("campaigns.delete"), api.DeleteCampaign)
}

func registerEngagementRoutes(v1 *gin.RouterGroup, api *handlers.API) {
	v1.GET("/activities", auth.RequirePerm("activities.read"), api.ListActivities)
	v1.POST("/activities", auth.RequirePerm("activities.create"), api.CreateActivity)
	v1.GET("/activities/:id", auth.RequirePerm("activities.read"), api.GetActivity)
	v1.PATCH("/activities/:id", auth.RequirePerm("activities.update"), api.PatchActivity)
	v1.DELETE("/activities/:id", auth.RequirePerm("activities.delete"), api.DeleteActivity)
	v1.GET("/activities/stream", auth.RequirePerm("activities.read"), api.ActivitiesStream)
	v1.POST("/exports/views/:page", auth.RequirePerm("exports.create"), api.ExportView)
	v1.GET("/exports/jobs/:id", auth.RequirePerm("exports.read"), api.GetExportJob)
	v1.GET("/tickets", auth.RequirePerm("tickets.read"), api.ListTickets)
	v1.POST("/tickets", auth.RequirePerm("tickets.create"), api.CreateTicket)
	v1.GET("/tickets/:id", auth.RequirePerm("tickets.read"), api.GetTicket)
	v1.PATCH("/tickets/:id", auth.RequirePerm("tickets.update"), api.PatchTicket)
	v1.DELETE("/tickets/:id", auth.RequirePerm("tickets.delete"), api.DeleteTicket)
	v1.GET("/loyalty/tiers", auth.RequirePerm("loyalty.read"), api.ListLoyaltyTiers)
	v1.GET("/loyalty/tier-rules", auth.RequirePerm("loyalty.read"), api.GetLoyaltyTierRules)
	v1.PUT("/loyalty/tier-rules", auth.RequirePerm("loyalty.update"), api.PutLoyaltyTierRules)
	v1.GET("/loyalty/outlets", auth.RequirePerm("loyalty.read"), api.ListLoyaltyOutlets)
	v1.POST("/loyalty/promotions", auth.RequirePerm("loyalty.create"), api.CreateLoyaltyPromotion)
}

func registerBridgeRoutes(v1 *gin.RouterGroup, api *handlers.API) {
	v1.GET("/bridge/status", auth.RequirePerm("bridge.read"), api.BridgeStatus)
	v1.POST("/bridge/sync", auth.RequirePerm("bridge.update"), api.Sync)
	v1.GET("/bridge/streams", auth.RequirePerm("bridge.read"), api.BridgeStreams)
	v1.GET("/bridge/pending-imports", auth.RequirePerm("bridge.read"), api.BridgePendingImports)
	v1.POST("/bridge/pending-imports/:id/assign", auth.RequirePerm("bridge.update"), api.AssignPendingImport)
	v1.GET("/bridge/mappings", auth.RequirePerm("bridge.read"), api.BridgeFieldMappings)
	v1.GET("/bridge/sync-log", auth.RequirePerm("bridge.read"), api.BridgeSyncLog)
	v1.GET("/outlets", auth.RequirePerm("outlets.read"), api.ListOutlets)
	v1.GET("/outlets/:id", auth.RequirePerm("outlets.read"), api.GetOutlet)
	v1.GET("/outlets/:id/360", auth.RequirePerm("outlets.read"), api.GetOutlet360)
	v1.GET("/export-customers", auth.RequirePerm("exports.read"), api.ListExportCustomers)
	v1.POST("/export-customers", auth.RequirePerm("exports.create"), api.CreateExportCustomer)
}

func registerIntelligenceRoutes(v1 *gin.RouterGroup, api *handlers.API) {
	v1.GET("/insights/summary", auth.RequirePerm("insights.read"), api.InsightsSummary)
	v1.GET("/insights/signals", auth.RequirePerm("insights.read"), api.InsightsSignals)
	v1.GET("/ai/suggestions", auth.RequirePerm("ai.read"), api.AISuggestions)
	v1.POST("/ai/copilot/chat", auth.RequirePerm("ai.create"), api.AICopilotChat)
}

func registerAuditRoutes(v1 *gin.RouterGroup, api *handlers.API) {
	v1.GET("/audit", auth.RequirePerm("audit.read"), api.ListAudit)
	v1.POST("/audit", auth.RequirePerm("audit.create"), api.AppendAuditEntry)
	v1.GET("/audit/:id", auth.RequirePerm("audit.read"), api.GetAuditEntry)
}

func registerIntegrationRoutes(v1 *gin.RouterGroup, api *handlers.API) {
	v1.GET("/integrations/status", auth.RequirePerm("integrations.read"), api.IntegrationsStatus)
	v1.GET("/integrations/connections", auth.RequirePerm("integrations.read"), api.ListIntegrations)
	v1.DELETE("/integrations/connections/:provider", auth.RequirePerm("integrations.update"), api.DeleteIntegration)
	v1.GET("/integrations/oauth/:provider/start", auth.RequirePerm("integrations.update"), api.IntegrationOAuthStart)
	v1.GET("/integrations/oauth/:provider/callback", api.IntegrationOAuthCallback)
	v1.POST("/integrations/calendar/sync", auth.RequirePerm("integrations.update"), api.SyncCalendar)
	v1.POST("/integrations/email/sync", auth.RequirePerm("integrations.update"), api.SyncEmail)
}

func registerAdminRoutes(v1 *gin.RouterGroup, api *handlers.API) {
	admin := v1.Group("/admin")
	admin.Use(auth.RequireStaff())
	{
		admin.GET("/audit", auth.RequirePerm("audit.read"), api.ListAudit)
		admin.GET("/audit-logs", auth.RequirePerm("admin.read"), api.AdminAuditLogs)
		admin.GET("/monitoring/summary", auth.RequirePerm("admin.read"), api.AdminMonitoringSummary)
		admin.GET("/monitoring/activity", auth.RequirePerm("admin.read"), api.AdminMonitoringActivity)
		admin.GET("/monitoring/bridge", auth.RequirePerm("admin.read"), api.AdminMonitoringBridge)
		admin.POST("/bridge/sync", auth.RequirePerm("admin.update"), api.AdminBridgeSync)
	}
}

func corsMiddleware(allowed string) gin.HandlerFunc {
	allowAny := allowed == "" || allowed == "*"
	allowedOrigins := splitAllowedOrigins(allowed)
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowAny || (origin != "" && originAllowed(origin, allowedOrigins)) {
			if origin != "" {
				c.Header("Access-Control-Allow-Origin", origin)
			} else if allowAny {
				c.Header("Access-Control-Allow-Origin", "*")
			}
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, If-Match, X-Requested-With, X-Request-ID")
		c.Header("Access-Control-Expose-Headers", "ETag, X-Request-ID")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func splitAllowedOrigins(allowed string) []string {
	if allowed == "" || allowed == "*" {
		return nil
	}
	parts := strings.Split(allowed, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func originAllowed(origin string, allowed []string) bool {
	for _, candidate := range allowed {
		if origin == candidate {
			return true
		}
	}
	return false
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		if os.Getenv("ENVIRONMENT") == "production" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}
