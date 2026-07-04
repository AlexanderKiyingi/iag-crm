// Package procurementclient reads vendors and purchase orders from
// iag-procurement so the CRM can surface them read-only. Procurement remains the
// system of record; nothing here mutates it.
package procurementclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	platformserviceauth "github.com/alvor-technologies/iag-platform-go/serviceauth"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	sa         *platformserviceauth.Client
}

type Config struct {
	BaseURL         string
	TokenURL        string
	ServiceClientID string
	ServiceSecret   string
}

// New returns a Client. When BaseURL is empty the client is disabled and callers
// degrade gracefully (the CRM surfaces an empty list rather than erroring).
func New(cfg Config) *Client {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		return &Client{}
	}
	var sa *platformserviceauth.Client
	if cfg.ServiceSecret != "" {
		sa = platformserviceauth.NewClient(platformserviceauth.Options{
			TokenURL:     cfg.TokenURL,
			ClientID:     cfg.ServiceClientID,
			ClientSecret: cfg.ServiceSecret,
			Audience:     "iag.procurement",
		})
	}
	return &Client{
		baseURL:    base,
		httpClient: &http.Client{Timeout: 12 * time.Second},
		sa:         sa,
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != ""
}

// ListVendors reads the procurement supplier master.
func (c *Client) ListVendors(ctx context.Context) ([]map[string]any, error) {
	return c.listRows(ctx, "/api/v1/vendors")
}

// ListPurchaseOrders reads procurement purchase orders.
func (c *Client) ListPurchaseOrders(ctx context.Context) ([]map[string]any, error) {
	return c.listRows(ctx, "/api/v1/purchase-orders")
}

// GetPurchaseOrder reads a single purchase order by id.
func (c *Client) GetPurchaseOrder(ctx context.Context, id string) (map[string]any, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("procurement client not configured")
	}
	var out map[string]any
	if err := c.getJSON(ctx, "/api/v1/purchase-orders/"+strings.TrimSpace(id), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// listRows fetches a collection and normalises the envelope. Procurement returns
// either a bare array or {"data": [...]}/{"items": [...]}; all are handled.
func (c *Client) listRows(ctx context.Context, path string) ([]map[string]any, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("procurement client not configured")
	}
	raw, err := c.getRaw(ctx, path)
	if err != nil {
		return nil, err
	}
	var wrapped struct {
		Data  []map[string]any `json:"data"`
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil {
		if len(wrapped.Data) > 0 {
			return wrapped.Data, nil
		}
		if len(wrapped.Items) > 0 {
			return wrapped.Items, nil
		}
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	return []map[string]any{}, nil
}

func (c *Client) setAuth(ctx context.Context, req *http.Request) error {
	if c.sa == nil {
		return nil
	}
	tok, err := c.sa.Token(ctx)
	if err != nil {
		return fmt.Errorf("procurement token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	return nil
}

func (c *Client) getRaw(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if err := c.setAuth(ctx, req); err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("procurement %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

func (c *Client) getJSON(ctx context.Context, path string, dest any) error {
	raw, err := c.getRaw(ctx, path)
	if err != nil {
		return err
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(raw, dest)
}
