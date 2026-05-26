package config

import "testing"

func TestValidateProductionRequiresJWTAndSecret(t *testing.T) {
	cfg := Config{
		Environment: "production",
		DatabaseURL: "postgres://u:p@localhost/db",
		Audience:    "iag.crm",
		AuthMode:    "none",
		CORSOrigin:  "https://app.example.com",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected AUTH_MODE=jwt requirement in production")
	}

	cfg.AuthMode = "jwt"
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
		AuthMode:            "jwt",
		CORSOrigin:          "*",
		ServiceClientSecret: "production-secret-min-16",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected wildcard CORS rejection in production")
	}
}
