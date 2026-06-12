// Package aiclient calls the shared iag-ai-platform gateway for LLM completions,
// so the CRM never embeds model credentials or provider SDKs of its own.
package aiclient

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

// Client posts completions to the shared AI platform.
type Client struct {
	baseURL    string
	httpClient *http.Client
	sa         *platformserviceauth.Client
}

// Config wires the AI platform upstream.
type Config struct {
	BaseURL         string
	TokenURL        string
	ServiceClientID string
	ServiceSecret   string
	Audience        string
}

// New returns a Client. When BaseURL is empty the client is disabled and
// Complete returns an error so callers can fall back gracefully.
func New(cfg Config) *Client {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		return &Client{}
	}
	aud := strings.TrimSpace(cfg.Audience)
	if aud == "" {
		aud = "iag.ai-platform"
	}
	var sa *platformserviceauth.Client
	if cfg.ServiceSecret != "" {
		sa = platformserviceauth.NewClient(platformserviceauth.Options{
			TokenURL:     cfg.TokenURL,
			ClientID:     cfg.ServiceClientID,
			ClientSecret: cfg.ServiceSecret,
			Audience:     aud,
		})
	}
	return &Client{
		baseURL:    base,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		sa:         sa,
	}
}

// Enabled reports whether an upstream is configured.
func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != ""
}

// Complete runs a single-prompt completion and returns the model's reply text.
// The AI platform applies its own default model when none is requested.
func (c *Client) Complete(ctx context.Context, system, prompt string, maxTokens int) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("ai client not configured")
	}
	if maxTokens <= 0 {
		maxTokens = 512
	}
	body, _ := json.Marshal(map[string]any{
		"system":    system,
		"prompt":    prompt,
		"maxTokens": maxTokens,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/completions", bytes.NewReader(body))
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
		return "", fmt.Errorf("ai completions %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("ai completions decode: %w", err)
	}
	return strings.TrimSpace(out.Text), nil
}

func (c *Client) setAuth(ctx context.Context, req *http.Request) error {
	if c.sa == nil {
		return nil
	}
	tok, err := c.sa.Token(ctx)
	if err != nil {
		return fmt.Errorf("ai token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	return nil
}
