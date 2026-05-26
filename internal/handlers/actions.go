package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iag/crm/backend/internal/events"
	"github.com/iag/crm/backend/internal/models"
	"github.com/iag/crm/backend/internal/store"
)

func (h *API) MarkDealWon(c *gin.Context) {
	item, err := h.Repo.SetDealStage(c.Request.Context(), c.Param("id"), models.DealStageWon)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mark won failed"})
		return
	}
	h.recordAudit(c, "DealWon", store.AuditDetail("deal", item.ID, "marked won"))
	if h.Events != nil {
		h.Events.PublishCommercial(c.Request.Context(), events.TypeDealWon, map[string]any{
			"deal_id": item.ID, "account": item.Account, "amount": item.Amount, "currency": item.Currency,
		}, item.ID)
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) SendQuote(c *gin.Context) {
	item, err := h.Repo.PatchQuote(c.Request.Context(), c.Param("id"), map[string]any{"status": "sent"})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "send quote failed"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "sign quote failed"})
		return
	}
	h.recordAudit(c, "QuoteSigned", store.AuditDetail("quote", item.ID, "signed"))
	c.JSON(http.StatusOK, item)
}

func (h *API) DeleteContact(c *gin.Context) {
	if err := h.Repo.DeleteContact(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete contact failed"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "assign import failed"})
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
	h.recordAudit(c, "ExportQueued", fmt.Sprintf("page=%s range=%s format=%s job=%s", page, in.Range, in.Format, jobID))
	c.JSON(http.StatusAccepted, gin.H{
		"job_id":  jobID,
		"status":  "queued",
		"page":    page,
		"range":   in.Range,
		"formats": in.Format,
		"eta_sec": 12,
	})
}

func (h *API) ActivitiesStream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
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
