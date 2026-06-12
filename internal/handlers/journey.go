package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/iag/crm/backend/internal/events"
	"github.com/iag/crm/backend/internal/store"
	"github.com/alvor-technologies/iag-platform-go/apierr"
)

func (h *API) ListJourneySteps(c *gin.Context) {
	items, err := h.Repo.ListJourneySteps(c.Request.Context(), c.Param("id"))
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list steps failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *API) CreateJourneyStep(c *gin.Context) {
	var in map[string]any
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid body")
		return
	}
	item, err := h.Repo.CreateJourneyStep(c.Request.Context(), c.Param("id"), in)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "create step failed")
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *API) EnrollJourney(c *gin.Context) {
	var in struct {
		ContactID    string `json:"contact_id"`
		LeadID       string `json:"lead_id"`
		SubjectEmail string `json:"subject_email"`
		SubjectName  string `json:"subject_name"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid body")
		return
	}
	if in.ContactID == "" && in.LeadID == "" {
		badRequest(c, "contact_id or lead_id required")
		return
	}
	var item store.JourneyEnrollment
	if err := h.Repo.WithinTx(c.Request.Context(), func(ctx context.Context) error {
		var e error
		item, e = h.Repo.EnrollInJourney(ctx, c.Param("id"), store.EnrollInput{
			ContactID: in.ContactID, LeadID: in.LeadID,
			SubjectEmail: in.SubjectEmail, SubjectName: in.SubjectName,
		})
		if e != nil {
			return e
		}
		if h.Events != nil {
			return h.Events.EnqueueCommercial(ctx, events.TypeJourneyEnrolled, map[string]any{
				"journey_id": c.Param("id"), "enrollment_id": item.ID,
			}, item.ID)
		}
		return nil
	}); err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "enroll failed")
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *API) ActivateJourney(c *gin.Context) {
	if err := h.Repo.ActivateJourney(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "activate failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id"), "status": "active"})
}

func (h *API) ListJourneyEnrollments(c *gin.Context) {
	items, err := h.Repo.ListJourneyEnrollments(c.Request.Context(), c.Param("id"), 100)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list enrollments failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}
