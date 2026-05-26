package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	ServiceName         string
	Addr                string
	Environment         string
	DatabaseURL         string
	JWTIssuer           string
	JWKSURL             string
	Audience            string
	AuthMode            string
	ServiceClientID     string
	ServiceClientSecret string
	AuthTokenURL        string
	GatewayAPIPrefix    string
	CORSOrigin          string
	PublicAPIURL        string
	AutoMigrate         bool
	SeedOnEmpty         bool
	ServeUI             bool
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
}

func Load() (Config, error) {
	env := strings.ToLower(strings.TrimSpace(envOr("ENVIRONMENT", envOr("APP_ENV", "development"))))
	issuer := envOr("JWT_ISSUER", "http://localhost:3001")
	authMode := strings.ToLower(strings.TrimSpace(envOr("AUTH_MODE", "jwt")))
	switch authMode {
	case "jwt", "none":
	default:
		return Config{}, fmt.Errorf("AUTH_MODE must be jwt or none (got %q)", authMode)
	}

	origins := envOr("ALLOWED_ORIGINS", envOr("CORS_ORIGIN", "http://localhost:3000,http://localhost:5173"))

	cfg := Config{
		ServiceName:         envOr("SERVICE_NAME", "crm"),
		Addr:                ListenAddr(),
		Environment:         env,
		DatabaseURL:         strings.TrimSpace(os.Getenv("DATABASE_URL")),
		JWTIssuer:           issuer,
		JWKSURL:             envOr("JWKS_URL", strings.TrimRight(issuer, "/")+"/.well-known/jwks.json"),
		Audience:            envOr("AUDIENCE", "iag.crm"),
		AuthMode:            authMode,
		ServiceClientID:     envOr("SERVICE_CLIENT_ID", "iag-crm"),
		ServiceClientSecret: strings.TrimSpace(os.Getenv("SERVICE_CLIENT_SECRET")),
		AuthTokenURL:        envOr("AUTH_TOKEN_URL", strings.TrimRight(issuer, "/")+"/oauth/token"),
		GatewayAPIPrefix:    strings.TrimSpace(envOr("GATEWAY_API_PREFIX", "/api/v1/crm")),
		CORSOrigin:          origins,
		PublicAPIURL:        strings.TrimRight(strings.TrimSpace(envOr("PUBLIC_API_URL", "http://localhost:8080")), "/"),
		AutoMigrate:         envOr("AUTO_MIGRATE", "true") != "false",
		SeedOnEmpty:         envOr("SEED_ON_EMPTY", "true") != "false",
		ServeUI:             envOr("SERVE_UI", "true") != "false",
		ReadTimeout:         30 * time.Second,
		WriteTimeout:        30 * time.Second,
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.Audience == "" {
		return fmt.Errorf("AUDIENCE is required (e.g. iag.crm)")
	}
	if c.IsProduction() && c.AuthMode != "jwt" {
		return fmt.Errorf("AUTH_MODE must be jwt in production")
	}
	if c.IsProduction() && c.CORSOrigin == "*" {
		return fmt.Errorf("set ALLOWED_ORIGINS in production (not *)")
	}
	if c.IsProduction() && strings.TrimSpace(c.ServiceClientSecret) == "" {
		return fmt.Errorf("SERVICE_CLIENT_SECRET is required in production")
	}
	if c.IsProduction() && len(strings.TrimSpace(c.ServiceClientSecret)) < 16 {
		return fmt.Errorf("SERVICE_CLIENT_SECRET must be at least 16 characters in production")
	}
	return nil
}

func (c Config) IsProduction() bool {
	return c.Environment == "production" || c.Environment == "prod"
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
