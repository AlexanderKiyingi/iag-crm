package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/iag/crm/backend/internal/events"
	"github.com/iag/crm/backend/internal/store"
	"github.com/alvor-technologies/iag-platform-go/apierr"
)

func (h *API) ListAccounts(c *gin.Context) {
	items, total, err := h.Repo.ListAccounts(c.Request.Context(), scopedListOpts(c))
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list accounts failed")
		return
	}
	paginated(c, items, total)
}

func (h *API) GetAccount(c *gin.Context) {
	item, err := h.Repo.GetAccount(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "get account failed")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) CreateAccount(c *gin.Context) {
	var in store.AccountInput
	if err := c.ShouldBindJSON(&in); err != nil || in.Name == "" {
		badRequest(c, "name is required")
		return
	}
	item, err := h.Repo.CreateAccount(c.Request.Context(), in)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "create account failed")
		return
	}
	h.recordAudit(c, "AccountCreated", store.AuditDetail("account", item.ID, "created"))
	c.JSON(http.StatusCreated, item)
}

func (h *API) PatchAccount(c *gin.Context) {
	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		badRequest(c, "invalid body")
		return
	}
	item, err := h.Repo.PatchAccount(c.Request.Context(), c.Param("id"), patch)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "update account failed")
		return
	}
	h.recordAudit(c, "AccountUpdated", store.AuditDetail("account", item.ID, "updated"))
	c.JSON(http.StatusOK, item)
}

func (h *API) DeleteAccount(c *gin.Context) {
	if err := h.Repo.DeleteAccount(c.Request.Context(), c.Param("id")); err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "delete account failed")
		return
	}
	h.recordAudit(c, "AccountDeleted", store.AuditDetail("account", c.Param("id"), "deleted"))
	c.Status(http.StatusNoContent)
}

func (h *API) ListContacts(c *gin.Context) {
	items, total, err := h.Repo.ListContacts(c.Request.Context(), scopedListOpts(c))
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list contacts failed")
		return
	}
	paginated(c, items, total)
}

func (h *API) GetContact(c *gin.Context) {
	item, err := h.Repo.GetContact(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "get contact failed")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) CreateContact(c *gin.Context) {
	var in store.ContactInput
	if err := c.ShouldBindJSON(&in); err != nil || in.Name == "" || in.Email == "" {
		badRequest(c, "name and email are required")
		return
	}
	item, err := h.Repo.CreateContact(c.Request.Context(), in)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "create contact failed")
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *API) PatchContact(c *gin.Context) {
	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		badRequest(c, "invalid body")
		return
	}
	item, err := h.Repo.PatchContact(c.Request.Context(), c.Param("id"), patch)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "update contact failed")
		return
	}
	h.recordAudit(c, "ContactUpdated", store.AuditDetail("contact", item.ID, "updated"))
	c.JSON(http.StatusOK, item)
}

func (h *API) ListLeads(c *gin.Context) {
	items, total, err := h.Repo.ListLeads(c.Request.Context(), scopedListOpts(c))
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list leads failed")
		return
	}
	paginated(c, items, total)
}

func (h *API) GetLead(c *gin.Context) {
	item, err := h.Repo.GetLead(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "get lead failed")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) CreateLead(c *gin.Context) {
	var in store.LeadInput
	if err := c.ShouldBindJSON(&in); err != nil || in.Name == "" || in.Email == "" {
		badRequest(c, "name and email are required")
		return
	}
	item, err := h.Repo.CreateLead(c.Request.Context(), in)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "create lead failed")
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *API) PatchLead(c *gin.Context) {
	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		badRequest(c, "invalid body")
		return
	}
	item, err := h.Repo.PatchLead(c.Request.Context(), c.Param("id"), patch)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "update lead failed")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) ConvertLead(c *gin.Context) {
	var in store.DealInput
	_ = c.ShouldBindJSON(&in)
	item, err := h.Repo.ConvertLead(c.Request.Context(), c.Param("id"), in)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "convert lead failed")
		return
	}
	h.recordAudit(c, "LeadConverted", store.AuditDetail("lead", c.Param("id"), "converted to deal "+item.ID))
	if h.Events != nil {
		h.Events.PublishCommercial(c.Request.Context(), events.TypeLeadConverted, map[string]any{
			"lead_id": c.Param("id"), "deal_id": item.ID,
		}, item.ID)
	}
	c.JSON(http.StatusCreated, item)
}

func (h *API) ListDeals(c *gin.Context) {
	items, total, err := h.Repo.ListDeals(c.Request.Context(), scopedListOpts(c))
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list deals failed")
		return
	}
	paginated(c, items, total)
}

func (h *API) GetDeal(c *gin.Context) {
	item, err := h.Repo.GetDeal(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "get deal failed")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) CreateDeal(c *gin.Context) {
	var in store.DealInput
	if err := c.ShouldBindJSON(&in); err != nil || in.Name == "" {
		badRequest(c, "name is required")
		return
	}
	item, err := h.Repo.CreateDeal(c.Request.Context(), in)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "create deal failed")
		return
	}
	h.recordAudit(c, "DealCreated", store.AuditDetail("deal", item.ID, "created"))
	c.JSON(http.StatusCreated, item)
}

func (h *API) PatchDeal(c *gin.Context) {
	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		badRequest(c, "invalid body")
		return
	}
	item, err := h.Repo.PatchDeal(c.Request.Context(), c.Param("id"), patch)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "update deal failed")
		return
	}
	h.recordAudit(c, "DealUpdated", store.AuditDetail("deal", item.ID, "updated"))
	if h.Events != nil {
		h.Events.PublishCommercial(c.Request.Context(), events.TypeDealUpdated, map[string]any{
			"deal_id": item.ID, "stage": item.Stage,
		}, item.ID)
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) SetDealStage(c *gin.Context) {
	var in struct {
		Stage string `json:"stage"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Stage == "" {
		badRequest(c, "stage is required")
		return
	}
	item, err := h.Repo.SetDealStage(c.Request.Context(), c.Param("id"), in.Stage)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "update stage failed")
		return
	}
	h.recordAudit(c, "DealStageChanged", store.AuditDetail("deal", item.ID, "stage="+in.Stage))
	if h.Events != nil {
		h.Events.PublishCommercial(c.Request.Context(), events.TypeDealUpdated, map[string]any{
			"deal_id": item.ID, "stage": item.Stage,
		}, item.ID)
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) ListQuotes(c *gin.Context) {
	items, total, err := h.Repo.ListQuotes(c.Request.Context(), scopedListOpts(c))
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list quotes failed")
		return
	}
	paginated(c, items, total)
}

func (h *API) CreateQuote(c *gin.Context) {
	var in store.QuoteInput
	if err := c.ShouldBindJSON(&in); err != nil || in.Total == 0 {
		badRequest(c, "total is required")
		return
	}
	item, err := h.Repo.CreateQuote(c.Request.Context(), in)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "create quote failed")
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *API) GetQuote(c *gin.Context) {
	item, err := h.Repo.GetQuote(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "get quote failed")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) PatchQuote(c *gin.Context) {
	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		badRequest(c, "invalid body")
		return
	}
	item, err := h.Repo.PatchQuote(c.Request.Context(), c.Param("id"), patch)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "update quote failed")
		return
	}
	h.recordAudit(c, "QuoteUpdated", store.AuditDetail("quote", item.ID, "updated"))
	c.JSON(http.StatusOK, item)
}

func (h *API) ListActivities(c *gin.Context) {
	items, total, err := h.Repo.ListActivities(c.Request.Context(), scopedListOpts(c))
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list activities failed")
		return
	}
	paginated(c, items, total)
}

func (h *API) CreateActivity(c *gin.Context) {
	var in store.ActivityInput
	if err := c.ShouldBindJSON(&in); err != nil || in.Subject == "" {
		badRequest(c, "subject is required")
		return
	}
	item, err := h.Repo.CreateActivity(c.Request.Context(), in)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "create activity failed")
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *API) ListTickets(c *gin.Context) {
	items, total, err := h.Repo.ListTickets(c.Request.Context(), scopedListOpts(c))
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list tickets failed")
		return
	}
	paginated(c, items, total)
}

func (h *API) CreateTicket(c *gin.Context) {
	var in store.TicketInput
	if err := c.ShouldBindJSON(&in); err != nil || in.Subject == "" {
		badRequest(c, "subject is required")
		return
	}
	item, err := h.Repo.CreateTicket(c.Request.Context(), in)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "create ticket failed")
		return
	}
	h.recordAudit(c, "TicketCreated", store.AuditDetail("ticket", item.ID, "created"))
	if h.Events != nil {
		h.Events.PublishCommercial(c.Request.Context(), events.TypeTicketCreated, map[string]any{
			"ticket_id": item.ID, "subject": item.Subject,
		}, item.ID)
	}
	c.JSON(http.StatusCreated, item)
}

func (h *API) PatchTicket(c *gin.Context) {
	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		badRequest(c, "invalid body")
		return
	}
	item, err := h.Repo.PatchTicket(c.Request.Context(), c.Param("id"), patch)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "update ticket failed")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) ListCampaigns(c *gin.Context) {
	items, total, err := h.Repo.ListCampaigns(c.Request.Context(), scopedListOpts(c))
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list campaigns failed")
		return
	}
	paginated(c, items, total)
}

func (h *API) CreateCampaign(c *gin.Context) {
	var in store.CampaignInput
	if err := c.ShouldBindJSON(&in); err != nil || in.Name == "" {
		badRequest(c, "name is required")
		return
	}
	item, err := h.Repo.CreateCampaign(c.Request.Context(), in)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "create campaign failed")
		return
	}
	c.JSON(http.StatusCreated, item)
}
