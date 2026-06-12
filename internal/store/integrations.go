package store

import (
	"context"
	"fmt"
	"time"

	"github.com/iag/crm/backend/internal/crypto"
)

// IntegrationConnection stores OAuth tokens for email/calendar sync.
type IntegrationConnection struct {
	UserEmail       string     `json:"user_email"`
	Provider        string     `json:"provider"`
	ProviderUserID  string     `json:"provider_user_id"`
	AccessToken     string     `json:"-"`
	RefreshToken    string     `json:"-"`
	TokenExpiresAt  *time.Time `json:"token_expires_at,omitempty"`
	Scopes          string     `json:"scopes"`
	CalendarSyncedAt *time.Time `json:"calendar_synced_at,omitempty"`
	EmailSyncedAt   *time.Time `json:"email_synced_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (r *Repository) sealToken(plain string) (string, error) {
	if plain == "" || len(r.tokenKey) == 0 {
		return plain, nil
	}
	return crypto.Seal(r.tokenKey, plain)
}

func (r *Repository) openToken(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !crypto.IsSealed(stored) {
		return stored, nil
	}
	if len(r.tokenKey) == 0 {
		return "", fmt.Errorf("integration token is encrypted but no INTEGRATION_TOKEN_SECRET configured")
	}
	return crypto.Unseal(r.tokenKey, stored)
}

func (r *Repository) UpsertIntegrationConnection(ctx context.Context, c IntegrationConnection) error {
	access, err := r.sealToken(c.AccessToken)
	if err != nil {
		return err
	}
	refresh, err := r.sealToken(c.RefreshToken)
	if err != nil {
		return err
	}
	_, err = r.db(ctx).Exec(ctx, `
		INSERT INTO crm_integration_connections (
			user_email, provider, provider_user_id, access_token, refresh_token,
			token_expires_at, scopes, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())
		ON CONFLICT (user_email, provider) DO UPDATE SET
			provider_user_id = EXCLUDED.provider_user_id,
			access_token = EXCLUDED.access_token,
			refresh_token = CASE WHEN EXCLUDED.refresh_token <> '' THEN EXCLUDED.refresh_token ELSE crm_integration_connections.refresh_token END,
			token_expires_at = EXCLUDED.token_expires_at,
			scopes = EXCLUDED.scopes,
			updated_at = NOW()
	`, c.UserEmail, c.Provider, c.ProviderUserID, access, refresh, c.TokenExpiresAt, c.Scopes)
	return err
}

func (r *Repository) GetIntegrationConnection(ctx context.Context, userEmail, provider string) (IntegrationConnection, error) {
	var c IntegrationConnection
	var accessStored, refreshStored string
	err := r.db(ctx).QueryRow(ctx, `
		SELECT user_email, provider, provider_user_id, access_token, refresh_token,
		       token_expires_at, scopes, calendar_synced_at, email_synced_at, updated_at
		FROM crm_integration_connections WHERE user_email = $1 AND provider = $2
	`, userEmail, provider).Scan(
		&c.UserEmail, &c.Provider, &c.ProviderUserID, &accessStored, &refreshStored,
		&c.TokenExpiresAt, &c.Scopes, &c.CalendarSyncedAt, &c.EmailSyncedAt, &c.UpdatedAt,
	)
	if err != nil {
		return c, err
	}
	c.AccessToken, err = r.openToken(accessStored)
	if err != nil {
		return c, err
	}
	c.RefreshToken, err = r.openToken(refreshStored)
	return c, err
}

func (r *Repository) ListIntegrationConnections(ctx context.Context, userEmail string) ([]IntegrationConnection, error) {
	rows, err := r.db(ctx).Query(ctx, `
		SELECT user_email, provider, provider_user_id, '', '', token_expires_at, scopes,
		       calendar_synced_at, email_synced_at, updated_at
		FROM crm_integration_connections WHERE user_email = $1 ORDER BY provider
	`, userEmail)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IntegrationConnection
	for rows.Next() {
		var c IntegrationConnection
		var empty1, empty2 string
		if err := rows.Scan(&c.UserEmail, &c.Provider, &c.ProviderUserID, &empty1, &empty2,
			&c.TokenExpiresAt, &c.Scopes, &c.CalendarSyncedAt, &c.EmailSyncedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) DeleteIntegrationConnection(ctx context.Context, userEmail, provider string) error {
	_, err := r.db(ctx).Exec(ctx, `DELETE FROM crm_integration_connections WHERE user_email = $1 AND provider = $2`, userEmail, provider)
	return err
}

func (r *Repository) TouchCalendarSync(ctx context.Context, userEmail, provider string) error {
	_, err := r.db(ctx).Exec(ctx, `
		UPDATE crm_integration_connections SET calendar_synced_at = NOW(), updated_at = NOW()
		WHERE user_email = $1 AND provider = $2
	`, userEmail, provider)
	return err
}

func (r *Repository) TouchEmailSync(ctx context.Context, userEmail, provider string) error {
	_, err := r.db(ctx).Exec(ctx, `
		UPDATE crm_integration_connections SET email_synced_at = NOW(), updated_at = NOW()
		WHERE user_email = $1 AND provider = $2
	`, userEmail, provider)
	return err
}

func (r *Repository) TouchContactByEmail(ctx context.Context, email string) error {
	_, err := r.db(ctx).Exec(ctx, `
		UPDATE crm_contacts SET updated_at = NOW() WHERE LOWER(email) = LOWER($1)
	`, email)
	return err
}
