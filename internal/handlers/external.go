package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/alvor-technologies/iag-platform-go/apierr"
)

// External read-only proxies. Invoices are owned by iag-finance and vendors /
// purchase orders by iag-procurement; the CRM surfaces them read-only so users
// see the real system of record instead of a local fork. When a downstream
// client is unconfigured the handler returns an empty list rather than erroring,
// so the CRM degrades gracefully in environments without those services.

func emptyList(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []any{}})
}

func (h *API) FinanceInvoicesList(c *gin.Context) {
	if h.Finance == nil || !h.Finance.Enabled() {
		emptyList(c)
		return
	}
	opts := listOpts(c)
	items, err := h.Finance.ListInvoices(c.Request.Context(), opts.Limit, opts.Offset)
	if err != nil {
		apierr.JSONStatus(c, http.StatusBadGateway, "finance invoices unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *API) FinanceInvoiceGet(c *gin.Context) {
	if h.Finance == nil || !h.Finance.Enabled() {
		notFound(c)
		return
	}
	item, err := h.Finance.GetInvoice(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *API) ProcurementVendorsList(c *gin.Context) {
	if h.Procurement == nil || !h.Procurement.Enabled() {
		emptyList(c)
		return
	}
	items, err := h.Procurement.ListVendors(c.Request.Context())
	if err != nil {
		apierr.JSONStatus(c, http.StatusBadGateway, "procurement vendors unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *API) ProcurementPurchaseOrdersList(c *gin.Context) {
	if h.Procurement == nil || !h.Procurement.Enabled() {
		emptyList(c)
		return
	}
	items, err := h.Procurement.ListPurchaseOrders(c.Request.Context())
	if err != nil {
		apierr.JSONStatus(c, http.StatusBadGateway, "procurement purchase orders unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *API) ProcurementPurchaseOrderGet(c *gin.Context) {
	if h.Procurement == nil || !h.Procurement.Enabled() {
		notFound(c)
		return
	}
	item, err := h.Procurement.GetPurchaseOrder(c.Request.Context(), c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	c.JSON(http.StatusOK, item)
}
