package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"log/slog"

	"github.com/iag/crm/backend/internal/commerce"
	"github.com/iag/crm/backend/internal/contractsclient"
	"github.com/iag/crm/backend/internal/events"
	"github.com/iag/crm/backend/internal/models"
	"github.com/iag/crm/backend/internal/store"
	"github.com/alvor-technologies/iag-platform-go/apierr"
)

func (h *API) MarkDealWon(c *gin.Context) {
	item, err := h.Repo.SetDealStage(c.Request.Context(), c.Param("id"), models.DealStageWon)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "mark won failed")
		return
	}
	h.recordAudit(c, "DealWon", store.AuditDetail("deal", item.ID, "marked won"))
	item = h.finalizeDealWon(c, item)
	c.JSON(http.StatusOK, item)
}

func (h *API) finalizeDealWon(c *gin.Context, item models.Deal) models.Deal {
	arRef, arErr := commerce.BookDealAR(c.Request.Context(), h.Finance, h.Repo, h.Users, item)
	if arErr != nil {
		slog.Warn("deal won finance ar", "deal", item.ID, "err", arErr)
	}
	if arRef != "" {
		item.FinanceARRef = arRef
	}
	if h.Events != nil {
		h.Events.PublishCommercial(c.Request.Context(), events.TypeDealWon, map[string]any{
			"deal_id": item.ID, "account": item.Account, "amount": item.Amount, "currency": item.Currency,
			"finance_ar_ref": arRef,
		}, item.ID)
	}
	return item
}

func (h *API) SendQuote(c *gin.Context) {
	var item models.Quote
	if err := h.Repo.WithinTx(c.Request.Context(), func(ctx context.Context) error {
		var e error
		item, e = h.Repo.PatchQuote(ctx, c.Param("id"), map[string]any{"status": "sent"})
		if e != nil {
			return e
		}
		if h.Events != nil {
			return h.Events.EnqueueCommercial(ctx, events.TypeQuoteSent, map[string]any{
				"quote_id": item.ID, "ref": item.Ref, "account": item.Account, "total": item.Total,
			}, item.ID)
		}
		return nil
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "send quote failed")
		return
	}
	h.recordAudit(c, "QuoteSent", store.AuditDetail("quote", item.ID, "sent"))
	c.JSON(http.StatusOK, item)
}

func (h *API) SignQuote(c *gin.Context) {
	item, err := h.Repo.PatchQuote(c.Request.Context(), c.Param("id"), map[string]any{"status": "signed"})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "sign quote failed")
		return
	}
	h.recordAudit(c, "QuoteSigned", store.AuditDetail("quote", item.ID, "signed"))
	arRef, arErr := commerce.BookQuoteAR(c.Request.Context(), h.Finance, h.Repo, h.Users, item)
	if arErr != nil {
		slog.Warn("quote signed finance ar", "quote", item.ID, "err", arErr)
	}
	if arRef != "" {
		item.FinanceARRef = arRef
	}
	contractRef := ""
	if h.Contracts != nil && h.Contracts.Enabled() {
		zone := item.Account
		if zone == "" {
			zone = "CRM"
		}
		cs := int64(item.Total)
		if cs <= 0 {
			cs = 1
		}
		ref, err := h.Contracts.CreateContract(c.Request.Context(), contractsclient.CreateInput{
			No:      "CRM-" + item.Ref,
			Name:    "Quote " + item.Ref,
			Zone:    zone,
			Sup:     item.Owner,
			Remarks: "Signed from CRM quote " + item.ID,
			Cs:      cs,
		})
		if err != nil {
			slog.Warn("quote signed contract", "quote", item.ID, "err", err)
		} else {
			contractRef = ref
			_ = h.Repo.SetQuoteContractRef(c.Request.Context(), item.ID, ref)
			item.ContractRef = ref
		}
	}
	if h.Events != nil {
		h.Events.PublishCommercial(c.Request.Context(), events.TypeQuoteSigned, map[string]any{
			"quote_id": item.ID, "ref": item.Ref, "account": item.Account, "total": item.Total,
			"finance_ar_ref": arRef, "contract_ref": contractRef, "crmQuoteId": item.ID,
		}, item.ID)
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) DeleteContact(c *gin.Context) {
	if err := h.Repo.DeleteContact(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "delete contact failed")
		return
	}
	h.recordAudit(c, "ContactDeleted", store.AuditDetail("contact", c.Param("id"), "deleted"))
	c.Status(http.StatusNoContent)
}

func (h *API) AssignPendingImport(c *gin.Context) {
	var in struct {
		Owner string `json:"owner"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Owner == "" {
		badRequest(c, "owner is required")
		return
	}
	item, err := h.Repo.AssignPendingImport(c.Request.Context(), c.Param("id"), in.Owner)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "assign import failed")
		return
	}
	h.recordAudit(c, "BridgeImportAssigned", fmt.Sprintf("pending import %s → %s", c.Param("id"), in.Owner))
	c.JSON(http.StatusOK, item)
}

func (h *API) ExportView(c *gin.Context) {
	page := c.Param("page")
	if page == "" {
		page = "overview"
	}
	var in struct {
		Range  string `json:"range"`
		Format string `json:"format"`
	}
	_ = c.ShouldBindJSON(&in)
	if in.Range == "" {
		in.Range = c.Query("range")
	}
	if in.Range == "" {
		in.Range = "week"
	}
	if in.Format == "" {
		in.Format = "csv,pdf"
	}
	jobID := "EXP-" + uuid.NewString()[:8]
	_ = h.Repo.CreateExportJob(c.Request.Context(), jobID, page, in.Range, in.Format)
	go func() {
		time.Sleep(2 * time.Second)
		_ = h.Repo.CompleteExportJob(context.Background(), jobID)
	}()
	h.recordAudit(c, "ExportQueued", fmt.Sprintf("page=%s range=%s format=%s job=%s", page, in.Range, in.Format, jobID))
	c.JSON(http.StatusAccepted, gin.H{
		"job_id":  jobID,
		"status":  "queued",
		"page":    page,
		"range":   in.Range,
		"formats": in.Format,
		"eta_sec": 3,
	})
}

func (h *API) GetExportJob(c *gin.Context) {
	job, err := h.Repo.GetExportJob(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "export job failed")
		return
	}
	c.JSON(http.StatusOK, job)
}

func (h *API) ActivitiesStream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		apierr.JSONStatus(c, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	send := func() bool {
		items, _, err := h.Repo.ListActivities(c.Request.Context(), store.ListOpts{Limit: 8})
		if err != nil {
			return false
		}
		payload, _ := json.Marshal(map[string]any{
			"type": "activities",
			"data": items,
			"at":   time.Now().UTC().Format(time.RFC3339),
		})
		if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", payload); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	if !send() {
		return
	}

	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			if !send() {
				return
			}
		}
	}
}
