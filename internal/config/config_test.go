package config

import "testing"

func TestValidateProductionRequiresJWKSAndSecret(t *testing.T) {
	cfg := Config{
		Environment: "production",
		DatabaseURL: "postgres://u:p@localhost/db",
		Audience:    "iag.crm",
		JWKSURL:     "https://auth.example.com/.well-known/jwks.json",
		CORSOrigin:  "https://app.example.com",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected SERVICE_CLIENT_SECRET requirement in production")
	}

	cfg.ServiceClientSecret = "short"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected SERVICE_CLIENT_SECRET min length in production")
	}

	cfg.ServiceClientSecret = "production-secret-min-16"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid production config: %v", err)
	}
}

func TestValidateRejectsWildcardCORSInProduction(t *testing.T) {
	cfg := Config{
		Environment:         "production",
		DatabaseURL:         "postgres://u:p@localhost/db",
		Audience:            "iag.crm",
		JWKSURL:             "https://auth.example.com/.well-known/jwks.json",
		CORSOrigin:          "*",
		ServiceClientSecret: "production-secret-min-16",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected wildcard CORS rejection in production")
	}
}

func TestValidateRequiresJWKSURL(t *testing.T) {
	cfg := Config{
		Environment: "development",
		DatabaseURL: "postgres://u:p@localhost/db",
		Audience:    "iag.crm",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected JWKS_URL requirement")
	}
}
