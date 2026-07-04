// Package financeclient calls iag-finance for AR booking and customer statements.
package financeclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	platformserviceauth "github.com/alvor-technologies/iag-platform-go/serviceauth"
)

// Client posts AR open items and reads customer statements.
type Client struct {
	baseURL    string
	httpClient *http.Client
	sa         *platformserviceauth.Client
}

// Config wires the finance upstream.
type Config struct {
	BaseURL         string
	TokenURL        string
	ServiceClientID string
	ServiceSecret   string
}

// New returns a Client. When BaseURL is empty the client is disabled.
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
			Audience:     "iag.finance",
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

// CreateARInput is the AR payload for a won deal or signed quote.
type CreateARInput struct {
	OrgID             string
	BillingIdentityID string
	CustomerRef       string
	DocumentRef       string
	Description       string
	Amount            string
	Currency          string
}

// CreateARItem opens an AR line in iag-finance.
func (c *Client) CreateARItem(ctx context.Context, in CreateARInput) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("finance client not configured")
	}
	body, _ := json.Marshal(map[string]any{
		"orgId":             strings.TrimSpace(in.OrgID),
		"billingIdentityId": strings.TrimSpace(in.BillingIdentityID),
		"customerRef":       strings.TrimSpace(in.CustomerRef),
		"documentRef":       in.DocumentRef,
		"description":       in.Description,
		"amount":            in.Amount,
		"currency":          in.Currency,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/ar/items", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := c.setAuth(ctx, req); err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("finance ar %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var out struct {
		DocumentRef string `json:"documentRef"`
		ID          string `json:"id"`
	}
	_ = json.Unmarshal(raw, &out)
	if out.DocumentRef != "" {
		return out.DocumentRef, nil
	}
	if out.ID != "" {
		return out.ID, nil
	}
	return in.DocumentRef, nil
}

// CustomerStatement is a lightweight AR summary for account 360.
type CustomerStatement struct {
	CustomerRef string `json:"customerRef"`
	OpenBalance string `json:"openBalance"`
	Overdue     string `json:"overdue"`
	OpenItems   int    `json:"openItems"`
	Currency    string `json:"currency"`
}

func (c *Client) CustomerStatement(ctx context.Context, customerRef string) (*CustomerStatement, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("finance client not configured")
	}
	ref := strings.TrimSpace(customerRef)
	if ref == "" {
		return nil, fmt.Errorf("customerRef required")
	}
	var out CustomerStatement
	if err := c.getJSON(ctx, "/v1/ar/customers/"+ref+"/statement", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListInvoices reads billing invoices from iag-finance. The finance list
// endpoint wraps rows in {"items": [...]}; we return the raw rows so the CRM
// layer can surface them read-only without re-modelling finance's schema.
func (c *Client) ListInvoices(ctx context.Context, limit, offset int) ([]map[string]any, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("finance client not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	var out struct {
		Items []map[string]any `json:"items"`
		Data  []map[string]any `json:"data"`
	}
	path := fmt.Sprintf("/v1/billing/invoices?limit=%d&offset=%d", limit, offset)
	if err := c.getJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	if len(out.Items) > 0 {
		return out.Items, nil
	}
	return out.Data, nil
}

// GetInvoice reads a single billing invoice by id.
func (c *Client) GetInvoice(ctx context.Context, id string) (map[string]any, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("finance client not configured")
	}
	var out map[string]any
	if err := c.getJSON(ctx, "/v1/billing/invoices/"+strings.TrimSpace(id), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) setAuth(ctx context.Context, req *http.Request) error {
	if c.sa == nil {
		return nil
	}
	tok, err := c.sa.Token(ctx)
	if err != nil {
		return fmt.Errorf("finance token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	return nil
}

func (c *Client) getJSON(ctx context.Context, path string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if err := c.setAuth(ctx, req); err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("finance %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(raw, dest)
}
