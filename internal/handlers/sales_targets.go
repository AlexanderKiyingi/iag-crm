package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/alvor-technologies/iag-platform-go/apierr"
	"github.com/iag/crm/backend/internal/store"
)

// GetSalesTargetProgress returns won-deal actuals vs active sales quotas.
func (h *API) GetSalesTargetProgress(c *gin.Context) {
	period := strings.TrimSpace(c.Query("period"))
	progress, err := h.Repo.SalesTargetProgress(c.Request.Context(), period)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "sales target progress failed")
		return
	}
	c.JSON(http.StatusOK, progress)
}

func (h *API) ListSalesTargets(c *gin.Context) {
	period := strings.TrimSpace(c.Query("period"))
	items, err := h.Repo.ListSalesTargets(c.Request.Context(), period)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list sales targets failed")
		return
	}
	if items == nil {
		items = []store.SalesTarget{}
	}
	paginated(c, items, len(items))
}

func (h *API) CreateSalesTarget(c *gin.Context) {
	var in store.SalesTargetInput
	if err := bindJSONCoerced(c, &in); err != nil || strings.TrimSpace(in.Name) == "" {
		badRequest(c, "name is required")
		return
	}
	item, err := h.Repo.CreateSalesTarget(c.Request.Context(), in)
	if err != nil {
		if errors.Is(err, store.ErrInvalidInput) {
			badRequest(c, err.Error())
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "create sales target failed")
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *API) PatchSalesTarget(c *gin.Context) {
	var patch map[string]any
	if err := c.ShouldBindJSON(&patch); err != nil {
		badRequest(c, "invalid body")
		return
	}
	item, err := h.Repo.PatchSalesTarget(c.Request.Context(), c.Param("id"), patch)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "update sales target failed")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) DeleteSalesTarget(c *gin.Context) {
	if err := h.Repo.DeleteSalesTarget(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "delete sales target failed")
		return
	}
	c.Status(http.StatusNoContent)
}
