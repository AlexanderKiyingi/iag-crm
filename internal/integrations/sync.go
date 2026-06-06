package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/iag/crm/backend/internal/store"
)

// SyncResult summarizes an import run.
type SyncResult struct {
	Imported int    `json:"imported"`
	Provider string `json:"provider"`
	Kind     string `json:"kind"`
}

// SyncCalendar imports upcoming calendar events as CRM activities.
func (s *Service) SyncCalendar(ctx context.Context, userEmail, provider string) (SyncResult, error) {
	conn, err := s.Repo.GetIntegrationConnection(ctx, userEmail, provider)
	if err != nil {
		return SyncResult{}, err
	}
	tok, err := s.ensureAccessToken(ctx, conn)
	if err != nil {
		return SyncResult{}, err
	}
	var events []calendarEvent
	switch provider {
	case "google":
		events, err = s.fetchGoogleCalendar(ctx, tok)
	case "microsoft":
		events, err = s.fetchMicrosoftCalendar(ctx, tok)
	default:
		return SyncResult{}, fmt.Errorf("unsupported provider %s", provider)
	}
	if err != nil {
		return SyncResult{}, err
	}
	imported := 0
	for _, ev := range events {
		_, err := s.Repo.CreateActivity(ctx, store.ActivityInput{
			Type:    "calendar",
			Subject: ev.Subject,
			Body:    ev.Body,
			Owner:   userEmail,
			OccurredAt: ev.Start,
		})
		if err == nil {
			imported++
		}
	}
	_ = s.Repo.TouchCalendarSync(ctx, userEmail, provider)
	return SyncResult{Imported: imported, Provider: provider, Kind: "calendar"}, nil
}

// SyncEmail touches contacts seen in recent mailbox messages.
func (s *Service) SyncEmail(ctx context.Context, userEmail, provider string) (SyncResult, error) {
	conn, err := s.Repo.GetIntegrationConnection(ctx, userEmail, provider)
	if err != nil {
		return SyncResult{}, err
	}
	tok, err := s.ensureAccessToken(ctx, conn)
	if err != nil {
		return SyncResult{}, err
	}
	var addrs []string
	switch provider {
	case "google":
		addrs, err = s.fetchGoogleEmailAddresses(ctx, tok)
	case "microsoft":
		addrs, err = s.fetchMicrosoftEmailAddresses(ctx, tok)
	default:
		return SyncResult{}, fmt.Errorf("unsupported provider %s", provider)
	}
	if err != nil {
		return SyncResult{}, err
	}
	for _, addr := range addrs {
		_ = s.Repo.TouchContactByEmail(ctx, addr)
		_, _ = s.Repo.CreateActivity(ctx, store.ActivityInput{
			Type: "email", Subject: "Synced message from "+addr, Owner: userEmail,
			OccurredAt: time.Now().UTC(),
		})
	}
	imported := len(addrs)
	_ = s.Repo.TouchEmailSync(ctx, userEmail, provider)
	return SyncResult{Imported: imported, Provider: provider, Kind: "email"}, nil
}

type calendarEvent struct {
	Subject string
	Body    string
	Start   time.Time
}

func (s *Service) fetchGoogleCalendar(ctx context.Context, token string) ([]calendarEvent, error) {
	q := url.Values{}
	q.Set("maxResults", "25")
	q.Set("singleEvents", "true")
	q.Set("orderBy", "startTime")
	q.Set("timeMin", time.Now().UTC().Format(time.RFC3339))
	u := "https://www.googleapis.com/calendar/v3/calendars/primary/events?" + q.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google calendar %s", resp.Status)
	}
	var payload struct {
		Items []struct {
			Summary     string `json:"summary"`
			Description string `json:"description"`
			Start       struct {
				DateTime string `json:"dateTime"`
			} `json:"start"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := make([]calendarEvent, 0, len(payload.Items))
	for _, it := range payload.Items {
		start, _ := time.Parse(time.RFC3339, it.Start.DateTime)
		out = append(out, calendarEvent{Subject: it.Summary, Body: it.Description, Start: start})
	}
	return out, nil
}

func (s *Service) fetchMicrosoftCalendar(ctx context.Context, token string) ([]calendarEvent, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://graph.microsoft.com/v1.0/me/events?$top=25&$orderby=start/dateTime", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("microsoft calendar %s", resp.Status)
	}
	var payload struct {
		Value []struct {
			Subject string `json:"subject"`
			Body    struct {
				Content string `json:"content"`
			} `json:"body"`
			Start struct {
				DateTime string `json:"dateTime"`
			} `json:"start"`
		} `json:"value"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := make([]calendarEvent, 0, len(payload.Value))
	for _, it := range payload.Value {
		start, _ := time.Parse(time.RFC3339, it.Start.DateTime)
		out = append(out, calendarEvent{Subject: it.Subject, Body: it.Body.Content, Start: start})
	}
	return out, nil
}

func (s *Service) fetchGoogleEmailAddresses(ctx context.Context, token string) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://gmail.googleapis.com/gmail/v1/users/me/messages?maxResults=20", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gmail list %s", resp.Status)
	}
	var payload struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	var out []string
	for _, m := range payload.Messages {
		addr, err := s.fetchGmailFrom(ctx, token, m.ID)
		if err == nil && addr != "" {
			out = append(out, addr)
		}
	}
	return out, nil
}

func (s *Service) fetchGmailFrom(ctx context.Context, token, msgID string) (string, error) {
	u := "https://gmail.googleapis.com/gmail/v1/users/me/messages/" + msgID + "?format=metadata&metadataHeaders=From"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var payload struct {
		Payload struct {
			Headers []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"headers"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}
	for _, h := range payload.Payload.Headers {
		if strings.EqualFold(h.Name, "From") {
			return parseEmailAddress(h.Value), nil
		}
	}
	return "", nil
}

func (s *Service) fetchMicrosoftEmailAddresses(ctx context.Context, token string) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://graph.microsoft.com/v1.0/me/messages?$top=20&$select=from", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("outlook mail %s", resp.Status)
	}
	var payload struct {
		Value []struct {
			From struct {
				EmailAddress struct {
					Address string `json:"address"`
				} `json:"emailAddress"`
			} `json:"from"`
		} `json:"value"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	var out []string
	for _, m := range payload.Value {
		if a := m.From.EmailAddress.Address; a != "" {
			out = append(out, a)
		}
	}
	return out, nil
}

func parseEmailAddress(raw string) string {
	if i := strings.Index(raw, "<"); i >= 0 {
		raw = strings.Trim(raw[i+1:], "> ")
	}
	return strings.TrimSpace(raw)
}

