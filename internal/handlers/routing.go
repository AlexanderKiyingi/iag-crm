package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/iag/crm/backend/internal/store"
	"github.com/alvor-technologies/iag-platform-go/apierr"
)

func (h *API) ListTerritories(c *gin.Context) {
	items, err := h.Repo.ListTerritories(c.Request.Context())
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list territories failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *API) CreateTerritory(c *gin.Context) {
	var in struct {
		Name       string `json:"name"`
		Region     string `json:"region"`
		OwnerEmail string `json:"owner_email"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.OwnerEmail) == "" {
		badRequest(c, "name and owner_email are required")
		return
	}
	item, err := h.Repo.CreateTerritory(c.Request.Context(), in.Name, in.Region, in.OwnerEmail)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "create territory failed")
		return
	}
	h.recordAudit(c, "TerritoryCreated", store.AuditDetail("territory", item.ID, "created"))
	c.JSON(http.StatusCreated, item)
}

func (h *API) ListAssignmentRules(c *gin.Context) {
	items, err := h.Repo.ListAssignmentRules(c.Request.Context())
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list assignment rules failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *API) CreateAssignmentRule(c *gin.Context) {
	var in store.AssignmentRule
	if err := c.ShouldBindJSON(&in); err != nil || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.OwnerEmail) == "" {
		badRequest(c, "name and owner_email are required")
		return
	}
	in.Active = true
	item, err := h.Repo.CreateAssignmentRule(c.Request.Context(), in)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "create assignment rule failed")
		return
	}
	h.recordAudit(c, "AssignmentRuleCreated", store.AuditDetail("assignment_rule", item.ID, "created"))
	c.JSON(http.StatusCreated, item)
}

func (h *API) ScoreLead(c *gin.Context) {
	id := c.Param("id")
	lead, err := h.Repo.GetLead(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "get lead failed")
		return
	}
	if !enforceOwner(c, lead.Owner) {
		return
	}

	score := lead.Score
	usedAI := false
	if h.AI != nil && h.AI.Enabled() {
		reply, aiErr := h.AI.Complete(
			c.Request.Context(),
			"You score B2B sales leads from 0 to 100. Reply with only an integer.",
			"Lead name: "+lead.Name+"\nCompany: "+lead.Company+"\nSource: "+lead.Source+
				"\nSegment: "+lead.Segment+"\nRegion: "+lead.Region+"\nNotes: "+lead.Notes+"\nCurrent score: "+strconv.Itoa(lead.Score),
			64,
		)
		if aiErr == nil {
			digits := strings.TrimSpace(reply)
			for _, part := range strings.FieldsFunc(digits, func(r rune) bool {
				return r < '0' || r > '9'
			}) {
				if n, err := strconv.Atoi(part); err == nil {
					score = n
					usedAI = true
					break
				}
			}
		}
	}
	if !usedAI {
		// Deterministic heuristic when AI platform is offline.
		score = 20
		if lead.Email != "" {
			score += 15
		}
		if lead.Company != "" {
			score += 15
		}
		if lead.Phone != "" {
			score += 10
		}
		if lead.Segment != "" {
			score += 10
		}
		if lead.Region != "" {
			score += 10
		}
		if lead.EstimatedValue != nil && *lead.EstimatedValue > 0 {
			score += 20
		}
		if score > 100 {
			score = 100
		}
	}

	item, err := h.Repo.SetLeadScore(c.Request.Context(), id, score)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "score lead failed")
		return
	}
	method := "heuristic"
	if usedAI {
		method = "ai"
	}
	h.recordAudit(c, "LeadScored", store.AuditDetail("lead", item.ID, "score="+strconv.Itoa(item.Score)+";method="+method))
	c.JSON(http.StatusOK, gin.H{
		"id":           item.ID,
		"score":        item.Score,
		"score_method": method,
		"name":         item.Name,
		"company":      item.Company,
		"email":        item.Email,
		"status":       item.Status,
		"owner":        item.Owner,
		"updated_at":   item.UpdatedAt,
	})
}
