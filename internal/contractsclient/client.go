// Package contractsclient creates contracts in iag-contract-management.
package contractsclient

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

// Client posts contracts to contract-management.
type Client struct {
	baseURL    string
	httpClient *http.Client
	sa         *platformserviceauth.Client
}

// Config wires the contracts upstream.
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
			Audience:     "iag.contract-management",
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

// CreateInput is the contract payload from a signed CRM quote.
type CreateInput struct {
	No      string `json:"no,omitempty"`
	Name    string `json:"name"`
	Zone    string `json:"zone"`
	Sup     string `json:"sup,omitempty"`
	Remarks string `json:"remarks,omitempty"`
	Cs      int64  `json:"cs,omitempty"`
}

// CreateContract opens a contract from a signed quote.
func (c *Client) CreateContract(ctx context.Context, in CreateInput) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("contracts client not configured")
	}
	body, _ := json.Marshal(in)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/contracts", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.sa != nil {
		tok, err := c.sa.Token(ctx)
		if err != nil {
			return "", fmt.Errorf("contracts token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("contracts create %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var out struct {
		No string `json:"no"`
	}
	_ = json.Unmarshal(raw, &out)
	if out.No != "" {
		return out.No, nil
	}
	if in.No != "" {
		return in.No, nil
	}
	return in.Name, nil
}
