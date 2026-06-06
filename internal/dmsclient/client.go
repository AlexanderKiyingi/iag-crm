// Package dmsclient fetches outlet universe data from iag-dms.
package dmsclient

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

// Outlet mirrors iag-dms outlet JSON.
type Outlet struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Address       string  `json:"address"`
	Channel       string  `json:"channel"`
	DistributorID string  `json:"distributorId"`
	BeatID        string  `json:"beatId"`
	QTDValueUGX   float64 `json:"qtdValueUgx"`
	Frequency     string  `json:"frequency"`
	Score         string  `json:"score"`
	Status        string  `json:"status"`
}

// Client talks to iag-dms.
type Client struct {
	baseURL    string
	httpClient *http.Client
	sa         *platformserviceauth.Client
}

// Config wires the DMS upstream.
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
			Audience:     "iag.dms",
		})
	}
	return &Client{
		baseURL:    base,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		sa:         sa,
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != ""
}

// ListOutlets returns a page of DMS outlets.
func (c *Client) ListOutlets(ctx context.Context, limit, offset int) ([]Outlet, int, error) {
	if !c.Enabled() {
		return nil, 0, fmt.Errorf("dms client not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	path := fmt.Sprintf("/v1/outlets?limit=%d&offset=%d", limit, offset)
	var wrapped struct {
		Items []Outlet `json:"items"`
		Data  []Outlet `json:"data"`
		Meta  struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := c.getJSON(ctx, path, &wrapped); err != nil {
		return nil, 0, err
	}
	items := wrapped.Items
	if len(items) == 0 {
		items = wrapped.Data
	}
	if len(items) > 0 {
		total := wrapped.Meta.Total
		if total == 0 {
			total = len(items)
		}
		return items, total, nil
	}
	var flat []Outlet
	if err := c.getJSON(ctx, path, &flat); err != nil {
		return nil, 0, err
	}
	return flat, len(flat), nil
}

// GetOutlet fetches a single outlet by id.
func (c *Client) GetOutlet(ctx context.Context, id string) (*Outlet, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("dms client not configured")
	}
	var out Outlet
	if err := c.getJSON(ctx, "/v1/outlets/"+id, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) getJSON(ctx context.Context, path string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.sa != nil {
		tok, err := c.sa.Token(ctx)
		if err != nil {
			return fmt.Errorf("dms token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("dms %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return json.Unmarshal(raw, dest)
}
