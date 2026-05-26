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

	platformserviceauth "github.com/alvor-technologies/iag-platform-go/serviceauth"
	"github.com/jackc/pgx/v5/pgxpool"

	crmDB "github.com/iag/crm/backend/db"
	"github.com/iag/crm/backend/internal/config"
	"github.com/iag/crm/backend/internal/db"
	"github.com/iag/crm/backend/internal/events"
	"github.com/iag/crm/backend/internal/handlers"
	"github.com/iag/crm/backend/internal/migrate"
	"github.com/iag/crm/backend/internal/middleware"
	"github.com/iag/crm/backend/internal/models"
	"github.com/iag/crm/backend/internal/platformauth"
	"github.com/iag/crm/backend/internal/router"
	"github.com/iag/crm/backend/internal/seed"
	"github.com/iag/crm/backend/internal/store"
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

	if cfg.SeedOnEmpty {
		if err := seed.Run(context.Background(), pool); err != nil {
			slog.Error("seed", "err", err)
			os.Exit(1)
		}
	}

	ctx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()

	var verifier *platformauth.Verifier
	if cfg.AuthMode == "jwt" {
		verifier = platformauth.NewVerifier(cfg.JWKSURL, cfg.JWTIssuer, cfg.Audience)
		jwksCtx, jwksCancel := context.WithTimeout(ctx, 10*time.Second)
		if err := verifier.Refresh(jwksCtx); err != nil {
			jwksCancel()
			slog.Error("jwks refresh", "err", err)
			os.Exit(1)
		}
		jwksCancel()
		verifier.StartRefreshLoop(ctx, 15*time.Minute)
	} else {
		slog.Warn("AUTH_MODE=none — open API for local development only")
	}

	if cfg.ServiceClientSecret != "" {
		go registerPermissionsLoop(ctx, cfg)
	}

	platformAuth := middleware.NewPlatformAuth(cfg.AuthMode, verifier)
	repo := store.New(pool)
	eventBus := events.NewFromEnv()
	defer eventBus.Close()

	api := &handlers.API{Repo: repo, Cfg: cfg, Events: eventBus}
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
			"authMode", cfg.AuthMode,
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
