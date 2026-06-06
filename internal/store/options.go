package store

import (
	"github.com/iag/crm/backend/internal/crypto"
)

// Option configures a Repository.
type Option func(*Repository)

// WithIntegrationTokenSecret enables AES-GCM encryption for OAuth tokens at rest.
func WithIntegrationTokenSecret(secret string) Option {
	return func(r *Repository) {
		r.tokenKey = crypto.DeriveKey(secret)
	}
}
