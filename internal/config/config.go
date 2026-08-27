package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/alvor-technologies/iag-platform-go/corsenv"
)

type Config struct {
	ServiceName               string
	Addr                      string
	Environment               string
	DatabaseURL               string
	JWTIssuer                 string
	JWKSURL                   string
	Audience                  string
	ServiceClientID           string
	ServiceClientSecret       string
	AuthTokenURL              string
	AuthServiceURL            string
	GatewayAPIPrefix          string
	CORSOrigin                string
	PublicAPIURL              string
	UsersAPIURL               string
	FinanceAPIURL             string
	ProcurementAPIURL         string
	DMSAPIURL                 string
	ContractsAPIURL           string
	AIAPIURL                  string
	AIAudience                string
	AutoMigrate               bool
	SeedOnEmpty               bool
	ReadTimeout               time.Duration
	WriteTimeout              time.Duration
	KafkaBrokers              []string
	EventBusEnabled           bool
	ConsumerEnabled           bool
	ConsumerGroupID           string
	ConsumerDLQTopic          string
	JourneyRunnerInterval     time.Duration
	GoogleOAuthRedirectURL    string
	MicrosoftOAuthRedirectURL string
	IntegrationTokenSecret    string
}

// Load reads configuration from env. Hard cutover: every request must carry a
// verifiable Bearer token with aud=iag.crm.
func Load() (Config, error) {
	env := strings.ToLower(strings.TrimSpace(envOr("ENVIRONMENT", envOr("APP_ENV", "development"))))
	issuer := envOr("JWT_ISSUER", "http://localhost:3001")
	authTokenURL := envOr("AUTH_TOKEN_URL", strings.TrimRight(issuer, "/")+"/oauth/token")
	authServiceURL := strings.TrimSpace(os.Getenv("AUTH_SERVICE_URL"))
	if authServiceURL == "" {
		authServiceURL = authServiceBaseFromTokenURL(authTokenURL)
	}
	if authServiceURL == "" {
		authServiceURL = strings.TrimRight(issuer, "/")
	}
	origins := corsenv.Allowlist(corsenv.DefaultDevOrigins)
	publicAPI := strings.TrimRight(strings.TrimSpace(envOr("PUBLIC_API_URL", "http://localhost:8080")), "/")

	cfg := Config{
		ServiceName:               envOr("SERVICE_NAME", "crm"),
		Addr:                      ListenAddr(),
		Environment:               env,
		DatabaseURL:               strings.TrimSpace(os.Getenv("DATABASE_URL")),
		JWTIssuer:                 issuer,
		JWKSURL:                   envOr("JWKS_URL", strings.TrimRight(issuer, "/")+"/.well-known/jwks.json"),
		Audience:                  envOr("AUDIENCE", "iag.crm"),
		ServiceClientID:           envOr("SERVICE_CLIENT_ID", "iag-crm"),
		ServiceClientSecret:       strings.TrimSpace(os.Getenv("SERVICE_CLIENT_SECRET")),
		AuthTokenURL:              authTokenURL,
		AuthServiceURL:            authServiceURL,
		GatewayAPIPrefix:          strings.TrimSpace(envOr("GATEWAY_API_PREFIX", "/api/v1/crm")),
		CORSOrigin:                origins,
		PublicAPIURL:              publicAPI,
		UsersAPIURL:               usersAPIURL(publicAPI, envOr("USERS_API_URL", "")),
		FinanceAPIURL:             financeAPIURL(publicAPI, envOr("FINANCE_API_URL", "")),
		ProcurementAPIURL:         procurementAPIURL(publicAPI, envOr("PROCUREMENT_API_URL", "")),
		DMSAPIURL:                 dmsAPIURL(publicAPI, envOr("DMS_API_URL", "")),
		ContractsAPIURL:           contractsAPIURL(publicAPI, envOr("CONTRACTS_API_URL", "")),
		AIAPIURL:                  aiAPIURL(publicAPI, envOr("AI_API_URL", "")),
		AIAudience:                envOr("AI_AUDIENCE", "iag.ai-platform"),
		AutoMigrate:               envOr("AUTO_MIGRATE", "true") != "false",
		SeedOnEmpty:               seedOnEmpty(env),
		ReadTimeout:               30 * time.Second,
		WriteTimeout:              30 * time.Second,
		EventBusEnabled:           strings.EqualFold(os.Getenv("EVENT_BUS_ENABLED"), "true"),
		ConsumerEnabled:           strings.EqualFold(os.Getenv("CONSUMER_ENABLED"), "true"),
		ConsumerGroupID:           envOr("CONSUMER_GROUP_ID", "iag.crm.commercial"),
		ConsumerDLQTopic:          envOr("CONSUMER_DLQ_TOPIC", "iag.dlq.crm"),
		JourneyRunnerInterval:     parseDuration("JOURNEY_RUNNER_INTERVAL", 30*time.Second),
		GoogleOAuthRedirectURL:    oauthRedirectURL(publicAPI, envOr("GOOGLE_OAUTH_REDIRECT_URL", ""), "google"),
		MicrosoftOAuthRedirectURL: oauthRedirectURL(publicAPI, envOr("MICROSOFT_OAUTH_REDIRECT_URL", ""), "microsoft"),
		IntegrationTokenSecret:    integrationTokenSecret(),
	}
	if brokers := strings.TrimSpace(os.Getenv("KAFKA_BROKERS")); brokers != "" {
		cfg.KafkaBrokers = ParseBrokers(brokers)
	}
	if cfg.EventBusEnabled && len(cfg.KafkaBrokers) == 0 {
		cfg.KafkaBrokers = []string{"127.0.0.1:19092"}
	}
	return cfg, cfg.Validate()
}

// seedOnEmpty reports whether CRM demo data may be written at boot. Production
// never seeds: the demo accounts, contacts and deals were purged and must not
// come back, so the environment check wins over SEED_ON_EMPTY rather than
// merely defaulting it. Elsewhere the flag still applies, defaulting on.
func seedOnEmpty(env string) bool {
	if env == "production" || env == "prod" {
		return false
	}
	return envOr("SEED_ON_EMPTY", "true") != "false"
}

func (c Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.Audience == "" {
		return fmt.Errorf("AUDIENCE is required (e.g. iag.crm)")
	}
	if c.JWKSURL == "" {
		return fmt.Errorf("JWKS_URL is required")
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

func authServiceBaseFromTokenURL(tokenURL string) string {
	tokenURL = strings.TrimSpace(tokenURL)
	if tokenURL == "" {
		return ""
	}
	if u, err := url.Parse(tokenURL); err == nil && u.Scheme != "" && u.Host != "" {
		u.Path = ""
		u.RawQuery = ""
		u.Fragment = ""
		return strings.TrimRight(u.String(), "/")
	}
	if i := strings.LastIndex(tokenURL, "/oauth/token"); i > 0 {
		return strings.TrimRight(tokenURL[:i], "/")
	}
	return strings.TrimRight(tokenURL, "/")
}

func ParseBrokers(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func serviceAPIURL(publicAPI, explicit, localDefault, suffix string) string {
	if e := strings.TrimRight(strings.TrimSpace(explicit), "/"); e != "" {
		return e
	}
	if p := strings.TrimRight(strings.TrimSpace(publicAPI), "/"); p != "" {
		return p + suffix
	}
	return localDefault
}

func usersAPIURL(publicAPI, explicit string) string {
	return serviceAPIURL(publicAPI, explicit, "http://localhost:8080/api/v1/users", "/api/v1/users")
}

func financeAPIURL(publicAPI, explicit string) string {
	return serviceAPIURL(publicAPI, explicit, "http://localhost:8080/api/v1/finance", "/api/v1/finance")
}

func procurementAPIURL(publicAPI, explicit string) string {
	return serviceAPIURL(publicAPI, explicit, "http://localhost:8080/api/v1/procurement", "/api/v1/procurement")
}

func dmsAPIURL(publicAPI, explicit string) string {
	return serviceAPIURL(publicAPI, explicit, "http://localhost:8080/api/v1/dms", "/api/v1/dms")
}

func contractsAPIURL(publicAPI, explicit string) string {
	return serviceAPIURL(publicAPI, explicit, "http://localhost:8080/api/v1/contract-management", "/api/v1/contract-management")
}

// aiAPIURL resolves the shared iag-ai-platform base. When neither AI_API_URL nor
// PUBLIC_API_URL is set it falls back to the service's local dev port. An empty
// result disables the AI copilot client (it degrades to a deterministic reply).
func aiAPIURL(publicAPI, explicit string) string {
	return serviceAPIURL(publicAPI, explicit, "http://localhost:3007", "/api/v1/ai-platform")
}

func parseDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}

func integrationTokenSecret() string {
	if v := strings.TrimSpace(os.Getenv("INTEGRATION_TOKEN_SECRET")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("SERVICE_CLIENT_SECRET"))
}

func oauthRedirectURL(publicAPI, explicit, provider string) string {
	if e := strings.TrimRight(strings.TrimSpace(explicit), "/"); e != "" {
		return e
	}
	base := strings.TrimRight(strings.TrimSpace(publicAPI), "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	return base + "/api/v1/crm/v1/integrations/oauth/" + provider + "/callback"
}
