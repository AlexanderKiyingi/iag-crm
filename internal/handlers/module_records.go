package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/alvor-technologies/iag-platform-go/apierr"
	"github.com/iag/crm/backend/internal/store"
)

// Module records are the generic, CRM-owned store for lightweight modules
// (products, services, solutions, projects, documents, …) that have no
// dedicated table. The module name is taken from the path so one handler set
// serves every such module.

func (h *API) ModuleRecordsList(c *gin.Context) {
	module := c.Param("module")
	items, total, err := h.Repo.ListModuleRecords(c.Request.Context(), module, scopedListOpts(c))
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list records failed")
		return
	}
	paginated(c, items, total)
}

func (h *API) ModuleRecordGet(c *gin.Context) {
	item, err := h.Repo.GetModuleRecord(c.Request.Context(), c.Param("module"), c.Param("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "get record failed")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) ModuleRecordCreate(c *gin.Context) {
	module := c.Param("module")
	var in store.ModuleRecordInput
	if err := bindJSONCoerced(c, &in); err != nil {
		badRequest(c, "invalid body")
		return
	}
	if in.Name == "" {
		badRequest(c, "name is required")
		return
	}
	item, err := h.Repo.CreateModuleRecord(c.Request.Context(), module, in)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "create record failed")
		return
	}
	h.recordAudit(c, "ModuleRecordCreated", store.AuditDetail(module, item.ID, "created"))
	c.JSON(http.StatusCreated, item)
}

func (h *API) ModuleRecordPatch(c *gin.Context) {
	module := c.Param("module")
	var in store.ModuleRecordInput
	if err := bindJSONCoerced(c, &in); err != nil {
		badRequest(c, "invalid body")
		return
	}
	item, err := h.Repo.UpdateModuleRecord(c.Request.Context(), module, c.Param("id"), in)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "update record failed")
		return
	}
	h.recordAudit(c, "ModuleRecordUpdated", store.AuditDetail(module, item.ID, "updated"))
	c.JSON(http.StatusOK, item)
}

func (h *API) ModuleRecordDelete(c *gin.Context) {
	module := c.Param("module")
	id := c.Param("id")
	if err := h.Repo.DeleteModuleRecord(c.Request.Context(), module, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			notFound(c)
			return
		}
		apierr.JSONStatus(c, http.StatusInternalServerError, "delete record failed")
		return
	}
	h.recordAudit(c, "ModuleRecordDeleted", store.AuditDetail(module, id, "deleted"))
	c.Status(http.StatusNoContent)
}
