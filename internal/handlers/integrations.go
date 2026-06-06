package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/iag/crm/backend/internal/integrations"
	"github.com/iag/crm/backend/internal/middleware"
	"github.com/alvor-technologies/iag-platform-go/apierr"
)

func (h *API) IntegrationOAuthStart(c *gin.Context) {
	if h.Integrations == nil {
		apierr.JSONStatus(c, http.StatusNotImplemented, "integrations not configured")
		return
	}
	provider := strings.ToLower(c.Param("provider"))
	claims, ok := middleware.Claims(c)
	if !ok || claims == nil || strings.TrimSpace(claims.Email) == "" {
		apierr.Unauthorized(c, "authentication required")
		return
	}
	returnTo := strings.TrimSpace(c.Query("returnTo"))
	authURL, err := h.Integrations.AuthorizeURL(provider, claims.Email, returnTo)
	if err != nil {
		apierr.JSONStatus(c, http.StatusServiceUnavailable, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"authorize_url": authURL})
}

func (h *API) IntegrationOAuthCallback(c *gin.Context) {
	if h.Integrations == nil {
		apierr.JSONStatus(c, http.StatusNotImplemented, "integrations not configured")
		return
	}
	provider := strings.ToLower(c.Param("provider"))
	code := strings.TrimSpace(c.Query("code"))
	state := strings.TrimSpace(c.Query("state"))
	if code == "" || state == "" {
		apierr.JSONStatus(c, http.StatusBadRequest, "missing code or state")
		return
	}
	returnTo, err := h.Integrations.ExchangeCode(c.Request.Context(), provider, code, state)
	if err != nil {
		apierr.JSONStatus(c, http.StatusBadGateway, err.Error())
		return
	}
	if returnTo != "" {
		c.Redirect(http.StatusFound, returnTo+"?connected="+provider)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "connected", "provider": provider})
}

func (h *API) ListIntegrations(c *gin.Context) {
	claims, ok := middleware.Claims(c)
	if !ok || claims == nil {
		apierr.Unauthorized(c, "authentication required")
		return
	}
	items, err := h.Repo.ListIntegrationConnections(c.Request.Context(), claims.Email)
	if err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "list failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *API) DeleteIntegration(c *gin.Context) {
	claims, ok := middleware.Claims(c)
	if !ok || claims == nil {
		apierr.Unauthorized(c, "authentication required")
		return
	}
	if err := h.Repo.DeleteIntegrationConnection(c.Request.Context(), claims.Email, c.Param("provider")); err != nil {
		apierr.JSONStatus(c, http.StatusInternalServerError, "delete failed")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *API) SyncCalendar(c *gin.Context) {
	h.syncIntegration(c, "calendar")
}

func (h *API) SyncEmail(c *gin.Context) {
	h.syncIntegration(c, "email")
}

func (h *API) syncIntegration(c *gin.Context, kind string) {
	if h.Integrations == nil {
		apierr.JSONStatus(c, http.StatusNotImplemented, "integrations not configured")
		return
	}
	claims, ok := middleware.Claims(c)
	if !ok || claims == nil {
		apierr.Unauthorized(c, "authentication required")
		return
	}
	provider := strings.ToLower(c.DefaultQuery("provider", "google"))
	var (
		res integrations.SyncResult
		err error
	)
	switch kind {
	case "calendar":
		res, err = h.Integrations.SyncCalendar(c.Request.Context(), claims.Email, provider)
	case "email":
		res, err = h.Integrations.SyncEmail(c.Request.Context(), claims.Email, provider)
	default:
		apierr.JSONStatus(c, http.StatusBadRequest, "unknown sync kind")
		return
	}
	if err != nil {
		apierr.JSONStatus(c, http.StatusBadGateway, err.Error())
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *API) IntegrationsStatus(c *gin.Context) {
	providers := []string{}
	if h.Integrations != nil {
		for _, p := range []string{"google", "microsoft"} {
			if h.Integrations.HasProvider(p) {
				providers = append(providers, p)
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"providers": providers})
}
