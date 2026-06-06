// Package integrations connects CRM to Google/Microsoft email and calendar.
package integrations

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/iag/crm/backend/internal/store"
)

// ProviderConfig describes an OAuth provider for CRM sync.
type ProviderConfig struct {
	Name         string
	AuthorizeURL string
	TokenURL     string
	UserinfoURL  string
	Scope        string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// Service manages OAuth and sync for calendar/email.
type Service struct {
	Repo        *store.Repository
	providers   map[string]ProviderConfig
	client      *http.Client
	stateSecret        []byte
	requireSignedState bool
}

// Config holds OAuth client credentials.
type Config struct {
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	MicrosoftClientID  string
	MicrosoftClientSecret string
	MicrosoftRedirectURL  string
	StateSecret        string
	RequireSignedState bool
}

// New builds a Service from env-style config.
func New(repo *store.Repository, cfg Config) *Service {
	s := &Service{
		Repo:        repo,
		providers:   map[string]ProviderConfig{},
		client:      &http.Client{Timeout: 20 * time.Second},
		stateSecret:        []byte(strings.TrimSpace(cfg.StateSecret)),
		requireSignedState: cfg.RequireSignedState,
	}
	add := func(p ProviderConfig) {
		if strings.TrimSpace(p.ClientID) != "" && strings.TrimSpace(p.RedirectURL) != "" {
			s.providers[p.Name] = p
		}
	}
	add(ProviderConfig{
		Name:         "google",
		AuthorizeURL: "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:     "https://oauth2.googleapis.com/token",
		UserinfoURL:  "https://openidconnect.googleapis.com/v1/userinfo",
		Scope: strings.Join([]string{
			"openid", "email", "profile",
			"https://www.googleapis.com/auth/calendar.readonly",
			"https://www.googleapis.com/auth/gmail.readonly",
		}, " "),
		ClientID:     firstNonEmpty(cfg.GoogleClientID, os.Getenv("GOOGLE_OAUTH_CLIENT_ID")),
		ClientSecret: firstNonEmpty(cfg.GoogleClientSecret, os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")),
		RedirectURL:  firstNonEmpty(cfg.GoogleRedirectURL, os.Getenv("GOOGLE_OAUTH_REDIRECT_URL")),
	})
	add(ProviderConfig{
		Name:         "microsoft",
		AuthorizeURL: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		UserinfoURL:  "https://graph.microsoft.com/oidc/userinfo",
		Scope: strings.Join([]string{
			"openid", "email", "profile", "offline_access",
			"Calendars.Read", "Mail.Read",
		}, " "),
		ClientID:     firstNonEmpty(cfg.MicrosoftClientID, os.Getenv("MICROSOFT_OAUTH_CLIENT_ID")),
		ClientSecret: firstNonEmpty(cfg.MicrosoftClientSecret, os.Getenv("MICROSOFT_OAUTH_CLIENT_SECRET")),
		RedirectURL:  firstNonEmpty(cfg.MicrosoftRedirectURL, os.Getenv("MICROSOFT_OAUTH_REDIRECT_URL")),
	})
	return s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// HasProvider reports whether provider OAuth is configured.
func (s *Service) HasProvider(name string) bool {
	_, ok := s.providers[strings.ToLower(name)]
	return ok
}

// AuthorizeURL returns the provider authorize redirect URL.
func (s *Service) AuthorizeURL(provider, userEmail, returnTo string) (string, error) {
	p, ok := s.providers[strings.ToLower(provider)]
	if !ok {
		return "", ErrProviderUnavailable
	}
	if s.requireSignedState && len(s.stateSecret) == 0 {
		return "", errors.New("oauth state signing secret required in production")
	}
	q := url.Values{}
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", p.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", p.Scope)
	state, err := s.encodeState(userEmail, returnTo)
	if err != nil {
		return "", err
	}
	q.Set("state", state)
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	return p.AuthorizeURL + "?" + q.Encode(), nil
}

// TokenSet is the result of a code exchange.
type TokenSet struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    *time.Time
	ProviderSub  string
	Email        string
	Name         string
}

// ExchangeCode exchanges an OAuth code and persists tokens for userEmail from state.
func (s *Service) ExchangeCode(ctx context.Context, provider, code, state string) (string, error) {
	p, ok := s.providers[strings.ToLower(provider)]
	if !ok {
		return "", ErrProviderUnavailable
	}
	userEmail, returnTo, err := s.decodeState(state)
	if err != nil {
		return "", err
	}
	tok, err := s.exchangeToken(ctx, p, code)
	if err != nil {
		return "", err
	}
	info, err := s.fetchUserinfo(ctx, p, tok.AccessToken)
	if err != nil {
		return "", err
	}
	if err := s.Repo.UpsertIntegrationConnection(ctx, store.IntegrationConnection{
		UserEmail:      userEmail,
		Provider:       p.Name,
		ProviderUserID: info.Sub,
		AccessToken:    tok.AccessToken,
		RefreshToken:   tok.RefreshToken,
		TokenExpiresAt: tok.ExpiresAt,
		Scopes:         p.Scope,
	}); err != nil {
		return "", err
	}
	return returnTo, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (s *Service) exchangeToken(ctx context.Context, p ProviderConfig, code string) (TokenSet, error) {
	form := url.Values{}
	form.Set("client_id", p.ClientID)
	form.Set("client_secret", p.ClientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", p.RedirectURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenSet{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(req)
	if err != nil {
		return TokenSet{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return TokenSet{}, fmt.Errorf("token exchange %s: %s", resp.Status, string(raw))
	}
	var tr tokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return TokenSet{}, err
	}
	out := TokenSet{AccessToken: tr.AccessToken, RefreshToken: tr.RefreshToken}
	if tr.ExpiresIn > 0 {
		t := time.Now().UTC().Add(time.Duration(tr.ExpiresIn) * time.Second)
		out.ExpiresAt = &t
	}
	return out, nil
}

type userInfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (s *Service) fetchUserinfo(ctx context.Context, p ProviderConfig, accessToken string) (userInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.UserinfoURL, nil)
	if err != nil {
		return userInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := s.client.Do(req)
	if err != nil {
		return userInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return userInfo{}, fmt.Errorf("userinfo %s: %s", resp.Status, string(raw))
	}
	var info userInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return userInfo{}, err
	}
	return info, nil
}

func (s *Service) ensureAccessToken(ctx context.Context, conn store.IntegrationConnection) (string, error) {
	if conn.TokenExpiresAt == nil || conn.TokenExpiresAt.After(time.Now().UTC().Add(2*time.Minute)) {
		return conn.AccessToken, nil
	}
	if conn.RefreshToken == "" {
		return conn.AccessToken, nil
	}
	p, ok := s.providers[conn.Provider]
	if !ok {
		return "", ErrProviderUnavailable
	}
	form := url.Values{}
	form.Set("client_id", p.ClientID)
	form.Set("client_secret", p.ClientSecret)
	form.Set("refresh_token", conn.RefreshToken)
	form.Set("grant_type", "refresh_token")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("refresh %s: %s", resp.Status, string(raw))
	}
	var tr tokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return "", err
	}
	exp := conn.TokenExpiresAt
	if tr.ExpiresIn > 0 {
		t := time.Now().UTC().Add(time.Duration(tr.ExpiresIn) * time.Second)
		exp = &t
	}
	_ = s.Repo.UpsertIntegrationConnection(ctx, store.IntegrationConnection{
		UserEmail: conn.UserEmail, Provider: conn.Provider,
		ProviderUserID: conn.ProviderUserID,
		AccessToken: tr.AccessToken, RefreshToken: conn.RefreshToken,
		TokenExpiresAt: exp, Scopes: conn.Scopes,
	})
	return tr.AccessToken, nil
}

var ErrProviderUnavailable = errors.New("integrations: provider not configured")

func (s *Service) encodeState(userEmail, returnTo string) (string, error) {
	payload, _ := json.Marshal(map[string]any{
		"u": userEmail, "r": returnTo, "exp": time.Now().UTC().Add(15 * time.Minute).Unix(),
	})
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	if len(s.stateSecret) == 0 {
		if s.requireSignedState {
			return "", errors.New("oauth state signing secret required in production")
		}
		return encoded, nil
	}
	sig := signState(s.stateSecret, encoded)
	return encoded + "." + sig, nil
}

func (s *Service) decodeState(encoded string) (string, string, error) {
	payloadPart, sigPart, signed := strings.Cut(encoded, ".")
	raw, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return "", "", errors.New("invalid oauth state")
	}
	if s.requireSignedState {
		if len(s.stateSecret) == 0 || !signed {
			return "", "", errors.New("invalid oauth state signature")
		}
	}
	if len(s.stateSecret) > 0 && signed {
		if !hmac.Equal([]byte(sigPart), []byte(signState(s.stateSecret, payloadPart))) {
			return "", "", errors.New("invalid oauth state signature")
		}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", "", errors.New("invalid oauth state")
	}
	userEmail, _ := m["u"].(string)
	returnTo, _ := m["r"].(string)
	if strings.TrimSpace(userEmail) == "" {
		return "", "", errors.New("invalid oauth state")
	}
	switch exp := m["exp"].(type) {
	case float64:
		if time.Now().UTC().Unix() > int64(exp) {
			return "", "", errors.New("oauth state expired")
		}
	}
	return userEmail, returnTo, nil
}

func signState(secret []byte, payload string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
