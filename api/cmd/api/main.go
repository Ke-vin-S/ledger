package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/audit"
	"github.com/Ke-vin-S/ledger/api/internal/config"
	"github.com/Ke-vin-S/ledger/api/internal/db"
	"github.com/Ke-vin-S/ledger/api/internal/domain/team"
	"github.com/Ke-vin-S/ledger/api/internal/domain/user"
	authhandler "github.com/Ke-vin-S/ledger/api/internal/handler/auth"
	teamhandler "github.com/Ke-vin-S/ledger/api/internal/handler/team"
	userhandler "github.com/Ke-vin-S/ledger/api/internal/handler/user"
	"github.com/Ke-vin-S/ledger/api/internal/middleware"
	"github.com/Ke-vin-S/ledger/api/internal/repository"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("startup: %v", err)
	}
}

func run() error {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()
	log.Println("connected to database")

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("parse redis url: %w", err)
	}
	rdb := redis.NewClient(redisOpts)
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	log.Println("connected to redis")

	// Core infrastructure
	auditor := audit.NewLogger(pool)
	jwtSvc, err := jwtauth.NewJWTService(cfg.JWTPrivateKey, cfg.JWTPublicKey)
	if err != nil {
		return fmt.Errorf("init jwt: %w", err)
	}
	tokenStore := jwtauth.NewTokenStore(rdb)
	resetStore := jwtauth.NewResetStore(rdb)
	authMW := jwtauth.RequireAuth(jwtSvc, tokenStore)

	// Repositories
	userRepo := repository.NewUserRepo(pool)
	teamRepo := repository.NewTeamRepo(pool)

	// Domain services
	userSvc := user.NewService(userRepo, auditor)
	teamSvc := team.NewService(teamRepo, userRepo, auditor)

	// Handlers
	authH := authhandler.New(userSvc, jwtSvc, tokenStore, resetStore, cfg.IsLocal(), cfg.GoogleClientID)
	userH := userhandler.New(userSvc, cfg.FrontendURL)
	teamH := teamhandler.New(teamSvc, cfg.FrontendURL)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.CORS)
	r.Use(middleware.RequestID)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Mount("/v1/auth", authH.Routes(authMW))
	r.Mount("/v1/users", userH.Routes(authMW))
	r.Mount("/v1/teams", teamH.Routes(authMW))
	r.With(authMW).Post("/v1/invite/{token}", teamH.JoinViaInviteLink)

	// Additional feature routes will be mounted here as each phase is built.

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("server listening on :%s (env=%s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-quit
	log.Println("shutting down...")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	log.Println("server stopped")
	return nil
}
