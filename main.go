package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "embed"

	platformotel "github.com/alvor-technologies/iag-platform-go/otel"
	platformserviceauth "github.com/alvor-technologies/iag-platform-go/serviceauth"
	"github.com/jackc/pgx/v5/pgxpool"

	crmDB "github.com/iag/crm/backend/db"
	"github.com/iag/crm/backend/internal/aiclient"
	"github.com/iag/crm/backend/internal/auth"
	"github.com/iag/crm/backend/internal/bridge"
	crmconsumer "github.com/iag/crm/backend/internal/consumer"
	"github.com/iag/crm/backend/internal/config"
	"github.com/iag/crm/backend/internal/contractsclient"
	"github.com/iag/crm/backend/internal/db"
	"github.com/iag/crm/backend/internal/dmsclient"
	"github.com/iag/crm/backend/internal/events"
	"github.com/iag/crm/backend/internal/financeclient"
	"github.com/iag/crm/backend/internal/handlers"
	"github.com/iag/crm/backend/internal/integrations"
	journeyrunner "github.com/iag/crm/backend/internal/journey"
	"github.com/iag/crm/backend/internal/migrate"
	"github.com/iag/crm/backend/internal/middleware"
	"github.com/iag/crm/backend/internal/models"
	"github.com/iag/crm/backend/internal/outbox"
	"github.com/iag/crm/backend/internal/platformauth"
	"github.com/iag/crm/backend/internal/router"
	"github.com/iag/crm/backend/internal/seed"
	"github.com/iag/crm/backend/internal/store"
	"github.com/iag/crm/backend/internal/usersclient"
)

//go:embed index.html
var indexHTML []byte

func main() {
	configureLogger()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	auth.SetStrictRBAC(cfg.IsProduction())

	// OpenTelemetry → otel-collector:4317 (non-blocking dial).
	if tp, err := platformotel.Init(context.Background(), platformotel.Config{
		ServiceName: cfg.ServiceName,
		Environment: cfg.Environment,
	}); err != nil {
		slog.Warn("otel disabled", "err", err)
	} else {
		defer func() {
			sc, c := context.WithTimeout(context.Background(), 5*time.Second)
			defer c()
			_ = tp.Shutdown(sc)
		}()
	}

	connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := db.Connect(connectCtx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		slog.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.AutoMigrate {
		if err := autoMigrate(context.Background(), pool); err != nil {
			slog.Error("auto-migrate", "err", err)
			os.Exit(1)
		}
	}
	if err := seed.EnsureJourneySteps(context.Background(), pool); err != nil {
		slog.Error("journey steps", "err", err)
		os.Exit(1)
	}

	if cfg.SeedOnEmpty {
		if err := seed.Run(context.Background(), pool); err != nil {
			slog.Error("seed", "err", err)
			os.Exit(1)
		}
	}

	ctx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()

	verifier := platformauth.NewVerifier(cfg.JWKSURL, cfg.JWTIssuer, cfg.Audience)
	jwksCtx, jwksCancel := context.WithTimeout(ctx, 10*time.Second)
	if err := verifier.Refresh(jwksCtx); err != nil {
		jwksCancel()
		slog.Error("jwks refresh", "err", err)
		os.Exit(1)
	}
	jwksCancel()
	verifier.StartRefreshLoop(ctx, 15*time.Minute)

	if cfg.ServiceClientSecret != "" {
		go registerPermissionsLoop(ctx, cfg)
	}

	platformAuth := middleware.NewPlatformAuth(verifier)
	repo := store.New(pool, store.WithIntegrationTokenSecret(cfg.IntegrationTokenSecret))

	eventBus := events.New(events.Config{
		Brokers: cfg.KafkaBrokers,
		Enabled: cfg.EventBusEnabled,
	})
	defer eventBus.Close()
	if eventBus.Enabled() {
		slog.Info("event bus enabled", "brokers", cfg.KafkaBrokers)
	}

	outboxStore := outbox.NewStore(pool)
	eventBus.SetOutbox(outboxStore)
	if eventBus.Enabled() {
		outboxPublisher := outbox.NewPublisher(outboxStore, outboxDispatcher{bus: eventBus})
		go outboxPublisher.Run(ctx)
		slog.Info("outbox publisher started")
	}

	saCfg := struct {
		TokenURL, ClientID, Secret string
	}{
		TokenURL: cfg.AuthTokenURL,
		ClientID: cfg.ServiceClientID,
		Secret:   cfg.ServiceClientSecret,
	}
	financeClient := financeclient.New(financeclient.Config{
		BaseURL: cfg.FinanceAPIURL, TokenURL: saCfg.TokenURL,
		ServiceClientID: saCfg.ClientID, ServiceSecret: saCfg.Secret,
	})
	usersClient := usersclient.New(usersclient.Config{
		BaseURL: cfg.UsersAPIURL, TokenURL: saCfg.TokenURL,
		ServiceClientID: saCfg.ClientID, ServiceSecret: saCfg.Secret,
	})
	dmsClient := dmsclient.New(dmsclient.Config{
		BaseURL: cfg.DMSAPIURL, TokenURL: saCfg.TokenURL,
		ServiceClientID: saCfg.ClientID, ServiceSecret: saCfg.Secret,
	})
	contractsClient := contractsclient.New(contractsclient.Config{
		BaseURL: cfg.ContractsAPIURL, TokenURL: saCfg.TokenURL,
		ServiceClientID: saCfg.ClientID, ServiceSecret: saCfg.Secret,
	})
	aiClient := aiclient.New(aiclient.Config{
		BaseURL: cfg.AIAPIURL, TokenURL: saCfg.TokenURL,
		ServiceClientID: saCfg.ClientID, ServiceSecret: saCfg.Secret,
		Audience: cfg.AIAudience,
	})
	bridgeSvc := &bridge.Service{Repo: repo, DMS: dmsClient}
	integrationsSvc := integrations.New(repo, integrations.Config{
		GoogleRedirectURL:       cfg.GoogleOAuthRedirectURL,
		MicrosoftRedirectURL:    cfg.MicrosoftOAuthRedirectURL,
		StateSecret:             cfg.IntegrationTokenSecret,
		RequireSignedState:      cfg.IsProduction(),
	})
	journeyRunner := &journeyrunner.Runner{
		Repo: repo, Events: eventBus, Tick: cfg.JourneyRunnerInterval,
	}
	go journeyRunner.Run(ctx)
	slog.Info("journey runner started", "interval", cfg.JourneyRunnerInterval)

	if cfg.ConsumerEnabled && len(cfg.KafkaBrokers) > 0 {
		consumer, closeDLQ, err := crmconsumer.New(crmconsumer.Options{
			Brokers:  cfg.KafkaBrokers,
			GroupID:  cfg.ConsumerGroupID,
			DLQTopic: cfg.ConsumerDLQTopic,
			Pool:     pool,
			Handler:  &crmconsumer.Handler{Repo: repo},
		})
		if err != nil {
			slog.Error("consumer init", "err", err)
			os.Exit(1)
		}
		defer closeDLQ()
		defer func() { _ = consumer.Close() }()
		go func() {
			if err := crmconsumer.Run(ctx, consumer); err != nil {
				slog.Warn("consumer stopped", "err", err)
			}
		}()
		slog.Info("commercial consumer enabled", "group", cfg.ConsumerGroupID)
	}

	api := &handlers.API{
		Repo: repo, Cfg: cfg, Events: eventBus,
		Finance: financeClient, Users: usersClient,
		DMS: dmsClient, Contracts: contractsClient, AI: aiClient, Bridge: bridgeSvc,
		Integrations: integrationsSvc,
	}
	engine := router.New(router.Options{
		Cfg:          cfg,
		PlatformAuth: platformAuth,
		Repo:         repo,
		API:          api,
		IndexHTML:    indexHTML,
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       120 * time.Second,
	}

	listenErr := make(chan error, 1)
	go func() {
		slog.Info("CRM API listening",
			"addr", cfg.Addr,
			"audience", cfg.Audience,
			"gatewayPrefix", cfg.GatewayAPIPrefix,
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-stop:
		slog.Info("shutdown", "signal", sig.String())
	case err := <-listenErr:
		slog.Error("listener died", "err", err)
		os.Exit(1)
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()
	_ = srv.Shutdown(shutdownCtx)
	cancelApp()
}

type outboxDispatcher struct {
	bus *events.Bus
}

func (d outboxDispatcher) DispatchOutbox(ctx context.Context, row outbox.Row) error {
	if d.bus == nil {
		return nil
	}
	return d.bus.DispatchOutbox(ctx, row.EventType, row.EventKey, row.Payload)
}

func configureLogger() {
	level := slog.LevelInfo
	if strings.EqualFold(os.Getenv("LOG_LEVEL"), "debug") {
		level = slog.LevelDebug
	}
	var handler slog.Handler
	if strings.ToLower(os.Getenv("LOG_FORMAT")) == "json" {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	}
	slog.SetDefault(slog.New(handler))
}

func autoMigrate(parent context.Context, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	applied, err := migrate.Up(ctx, pool, crmDB.Migrations())
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if len(applied) == 0 {
		slog.Info("schema already up to date")
	} else {
		slog.Info("migrations applied", "versions", applied)
	}
	return nil
}

func registerPermissionsLoop(ctx context.Context, cfg config.Config) {
	saClient := platformserviceauth.NewClient(platformserviceauth.Options{
		TokenURL:     cfg.AuthTokenURL,
		ClientID:     cfg.ServiceClientID,
		ClientSecret: cfg.ServiceClientSecret,
		Audience:     "iag.authentication",
	})
	descriptors := models.PermissionDescriptors()
	perms := make([]platformserviceauth.Permission, 0, len(descriptors))
	for _, d := range descriptors {
		perms = append(perms, platformserviceauth.Permission{Name: d.Name, Description: d.Description})
	}
	backoff := time.Second
	for {
		regCtx, c := context.WithTimeout(ctx, 10*time.Second)
		err := platformserviceauth.RegisterPermissions(regCtx, saClient, cfg.JWTIssuer, "crm", perms)
		c()
		if err == nil {
			slog.Info("permissions registered", "count", len(perms))
			return
		}
		slog.Warn("permissions register failed", "err", err, "retry_in", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 5*time.Minute {
			backoff *= 2
		}
	}
}
