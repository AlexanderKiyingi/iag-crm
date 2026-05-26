package platformauth

import (
	"context"
	"time"

	platformauthclient "github.com/alvor-technologies/iag-platform-go/authclient"
)

type Claims = platformauthclient.Claims

type Verifier struct {
	inner *platformauthclient.Verifier
}

func NewVerifier(jwksURL, issuer, audience string) *Verifier {
	return &Verifier{
		inner: platformauthclient.NewVerifier(platformauthclient.Options{
			JWKSURL:  jwksURL,
			Issuer:   issuer,
			Audience: audience,
		}),
	}
}

func (v *Verifier) Refresh(ctx context.Context) error { return v.inner.Refresh(ctx) }

func (v *Verifier) StartRefreshLoop(ctx context.Context, interval time.Duration) {
	v.inner.StartRefreshLoop(ctx, interval)
}

func (v *Verifier) Verify(token string) (*Claims, error) { return v.inner.Verify(token) }
