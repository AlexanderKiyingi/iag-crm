package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/iag/crm/backend/internal/events"
	"github.com/iag/crm/backend/internal/journey"
	"github.com/iag/crm/backend/internal/models"
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
	if !enforceOwner(c, item.Owner) {
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
	var item models.Account
	if err := h.Repo.WithinTx(c.Request.Context(), func(ctx context.Context) error {
		var e error
		item, e = h.Repo.CreateAccount(ctx, in)
		if e != nil {
			return e
		}
		if h.Events != nil {
			return h.Events.EnqueueCommercial(ctx, events.TypeAccountCreated, map[string]any{
				"account_id": item.ID, "name": item.Name, "owner": item.Owner,
			}, item.ID)
		}
		return nil
	}); err != nil {
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
	if !enforceOwner(c, item.Owner) {
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
	var item models.Contact
	if err := h.Repo.WithinTx(c.Request.Context(), func(ctx context.Context) error {
		var e error
		item, e = h.Repo.CreateContact(ctx, in)
		if e != nil {
			return e
		}
		if h.Events != nil {
			return h.Events.EnqueueCommercial(ctx, events.TypeContactCreated, map[string]any{
				"contact_id": item.ID, "name": item.Name, "account": item.Account,
			}, item.ID)
		}
		return nil
	}); err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "create contact failed")
		return
	}
	h.recordAudit(c, "ContactCreated", store.AuditDetail("contact", item.ID, "created"))
	_ = journey.AutoEnrollContact(c.Request.Context(), h.Repo, item.ID, item.Email, item.Name)
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
	if !enforceOwner(c, item.Owner) {
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
	h.recordAudit(c, "LeadCreated", store.AuditDetail("lead", item.ID, "created"))
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
	h.recordAudit(c, "LeadUpdated", store.AuditDetail("lead", item.ID, "updated"))
	c.JSON(http.StatusOK, item)
}

func (h *API) ConvertLead(c *gin.Context) {
	var in store.DealInput
	_ = c.ShouldBindJSON(&in)
	var item models.Deal
	if err := h.Repo.WithinTx(c.Request.Context(), func(ctx context.Context) error {
		var e error
		item, e = h.Repo.ConvertLead(ctx, c.Param("id"), in)
		if e != nil {
			return e
		}
		if h.Events != nil {
			return h.Events.EnqueueCommercial(ctx, events.TypeLeadConverted, map[string]any{
				"lead_id": c.Param("id"), "deal_id": item.ID,
			}, item.ID)
		}
		return nil
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "convert lead failed")
		return
	}
	h.recordAudit(c, "LeadConverted", store.AuditDetail("lead", c.Param("id"), "converted to deal "+item.ID))
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
	if !enforceOwner(c, item.Owner) {
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
	var item models.Deal
	if err := h.Repo.WithinTx(c.Request.Context(), func(ctx context.Context) error {
		var e error
		item, e = h.Repo.PatchDeal(ctx, c.Param("id"), patch)
		if e != nil {
			return e
		}
		// The won path publishes after external finance booking, so it can't be
		// part of this transaction; only the plain stage update is atomic here.
		if item.Stage != models.DealStageWon && h.Events != nil {
			return h.Events.EnqueueCommercial(ctx, events.TypeDealUpdated, map[string]any{
				"deal_id": item.ID, "stage": item.Stage,
			}, item.ID)
		}
		return nil
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "update deal failed")
		return
	}
	h.recordAudit(c, "DealUpdated", store.AuditDetail("deal", item.ID, "updated"))
	if item.Stage == models.DealStageWon {
		item = h.finalizeDealWon(c, item)
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
	var item models.Deal
	if err := h.Repo.WithinTx(c.Request.Context(), func(ctx context.Context) error {
		var e error
		item, e = h.Repo.SetDealStage(ctx, c.Param("id"), in.Stage)
		if e != nil {
			return e
		}
		if item.Stage != models.DealStageWon && h.Events != nil {
			return h.Events.EnqueueCommercial(ctx, events.TypeDealUpdated, map[string]any{
				"deal_id": item.ID, "stage": item.Stage,
			}, item.ID)
		}
		return nil
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "update stage failed")
		return
	}
	h.recordAudit(c, "DealStageChanged", store.AuditDetail("deal", item.ID, "stage="+in.Stage))
	if item.Stage == models.DealStageWon {
		item = h.finalizeDealWon(c, item)
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
	if !enforceOwner(c, item.Owner) {
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
	h.recordAudit(c, "ActivityCreated", store.AuditDetail("activity", item.ID, "created"))
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
	var item models.Ticket
	if err := h.Repo.WithinTx(c.Request.Context(), func(ctx context.Context) error {
		var e error
		item, e = h.Repo.CreateTicket(ctx, in)
		if e != nil {
			return e
		}
		if h.Events != nil {
			return h.Events.EnqueueCommercial(ctx, events.TypeTicketCreated, map[string]any{
				"ticket_id": item.ID, "subject": item.Subject,
			}, item.ID)
		}
		return nil
	}); err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "create ticket failed")
		return
	}
	h.recordAudit(c, "TicketCreated", store.AuditDetail("ticket", item.ID, "created"))
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
	var item models.Campaign
	if err := h.Repo.WithinTx(c.Request.Context(), func(ctx context.Context) error {
		var e error
		item, e = h.Repo.CreateCampaign(ctx, in)
		if e != nil {
			return e
		}
		if h.Events != nil {
			return h.Events.EnqueueCommercial(ctx, events.TypeCampaignLaunched, map[string]any{
				"campaign_id": item.ID, "name": item.Name,
			}, item.ID)
		}
		return nil
	}); err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "create campaign failed")
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *API) DeleteDeal(c *gin.Context) {
	if err := h.Repo.DeleteDeal(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "delete deal failed")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *API) DeleteLead(c *gin.Context) {
	if err := h.Repo.DeleteLead(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "delete lead failed")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *API) DeleteQuote(c *gin.Context) {
	if err := h.Repo.DeleteQuote(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "delete quote failed")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *API) DeleteTicket(c *gin.Context) {
	if err := h.Repo.DeleteTicket(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "delete ticket failed")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *API) DeleteCampaign(c *gin.Context) {
	if err := h.Repo.DeleteCampaign(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "delete campaign failed")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *API) PatchActivity(c *gin.Context) {
	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		badRequest(c, "invalid body")
		return
	}
	item, err := h.Repo.PatchActivity(c.Request.Context(), c.Param("id"), patch)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "update activity failed")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) DeleteActivity(c *gin.Context) {
	if err := h.Repo.DeleteActivity(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "delete activity failed")
		return
	}
	c.Status(http.StatusNoContent)
}
