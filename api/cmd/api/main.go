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
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	jwtauth "github.com/Ke-vin-S/ledger/api/internal/auth"
	"github.com/Ke-vin-S/ledger/api/internal/audit"
	"github.com/Ke-vin-S/ledger/api/internal/config"
	"github.com/Ke-vin-S/ledger/api/internal/db"
	"github.com/Ke-vin-S/ledger/api/internal/domain/expense"
	"github.com/Ke-vin-S/ledger/api/internal/domain/auditlog"
	domainflag "github.com/Ke-vin-S/ledger/api/internal/domain/flag"
	"github.com/Ke-vin-S/ledger/api/internal/domain/notification"
	"github.com/Ke-vin-S/ledger/api/internal/domain/settlement"
	"github.com/Ke-vin-S/ledger/api/internal/domain/team"
	"github.com/Ke-vin-S/ledger/api/internal/domain/user"
	authhandler "github.com/Ke-vin-S/ledger/api/internal/handler/auth"
	expensehandler "github.com/Ke-vin-S/ledger/api/internal/handler/expense"
	flaghandler "github.com/Ke-vin-S/ledger/api/internal/handler/flag"
	auditloghandler "github.com/Ke-vin-S/ledger/api/internal/handler/auditlog"
	notificationhandler "github.com/Ke-vin-S/ledger/api/internal/handler/notification"
	"github.com/Ke-vin-S/ledger/api/internal/graph"
	settlementhandler "github.com/Ke-vin-S/ledger/api/internal/handler/settlement"
	teamhandler "github.com/Ke-vin-S/ledger/api/internal/handler/team"
	userhandler "github.com/Ke-vin-S/ledger/api/internal/handler/user"
	"github.com/Ke-vin-S/ledger/api/internal/middleware"
	"github.com/Ke-vin-S/ledger/api/internal/repository"
	"github.com/Ke-vin-S/ledger/api/internal/storage"
)

// teamGateway adapts team.Repository to expense.TeamGateway.
type teamGatewayAdapter struct{ repo team.Repository }

func teamGateway(repo team.Repository) expense.TeamGateway {
	return &teamGatewayAdapter{repo: repo}
}

func (a *teamGatewayAdapter) GetMembership(ctx context.Context, teamID, userID uuid.UUID) (string, string, error) {
	m, err := a.repo.GetMembership(ctx, teamID, userID)
	if err != nil {
		return "", "", err
	}
	return m.Role, m.Status, nil
}

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
	expenseRepo := repository.NewExpenseRepo(pool)
	settlementRepo := repository.NewSettlementRepo(pool)
	flagRepo := repository.NewFlagRepo(pool)
	notificationRepo := repository.NewNotificationRepo(pool)
	auditLogRepo := repository.NewAuditLogRepo(pool)
	activityStore := repository.NewActivityStore(pool)
	dashStore := repository.NewDashboardStore(pool)
	historyStore := repository.NewExpenseHistoryStore(pool)

	// S3 presigner
	presigner, err := storage.NewS3Presigner(ctx, cfg.S3Bucket, cfg.AWSRegion)
	if err != nil {
		return fmt.Errorf("init s3 presigner: %w", err)
	}

	// Domain services
	userSvc := user.NewService(userRepo, auditor)
	teamSvc := team.NewService(teamRepo, userRepo, auditor)
	expenseSvc := expense.NewService(expenseRepo, teamGateway(teamRepo), auditor, presigner)
	settlementSvc := settlement.NewService(settlementRepo, auditor)
	flagSvc := domainflag.NewService(flagRepo, auditor)
	notificationSvc := notification.NewService(notificationRepo)
	auditLogSvc := auditlog.NewService(auditLogRepo)
	gqlResolver := graph.NewResolver(activityStore, dashStore, historyStore)

	// Handlers
	authH := authhandler.New(userSvc, jwtSvc, tokenStore, resetStore, cfg.IsLocal(), cfg.GoogleClientID)
	userH := userhandler.New(userSvc, cfg.FrontendURL)
	teamH := teamhandler.New(teamSvc, cfg.FrontendURL)
	expenseH := expensehandler.New(expenseSvc, cfg.FrontendURL)
	settlementH := settlementhandler.New(settlementSvc)
	flagH := flaghandler.New(flagSvc)
	notificationH := notificationhandler.New(notificationSvc)
	auditLogH := auditloghandler.New(auditLogSvc)

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
	r.Mount("/v1/expenses", expenseH.Routes(authMW))

	// Team-scoped expense routes sit under the team router.
	r.Route("/v1/teams/{teamId}/expenses", func(r chi.Router) {
		r.Mount("/", expenseH.TeamRoutes(authMW))
	})

	// Settlement routes
	r.Route("/v1/expenses/{expenseId}/settlements", func(r chi.Router) {
		r.Mount("/", settlementH.ExpenseRoutes(authMW))
	})
	r.Mount("/v1/settlements", settlementH.SettlementRoutes(authMW))
	r.Route("/v1/teams/{teamId}/balances", func(r chi.Router) {
		r.Mount("/", settlementH.TeamBalanceRoutes(authMW))
	})
	r.Mount("/v1/balances", settlementH.MyBalancesHandler(authMW))

	// Flag routes
	r.Route("/v1/expenses/{expenseId}/flags", func(r chi.Router) {
		r.Mount("/", flagH.ExpenseRoutes(authMW))
	})
	r.Mount("/v1/flags", flagH.FlagRoutes(authMW))

	// Notification routes
	r.Mount("/v1/notifications", notificationH.Routes(authMW))

	// Audit log read routes
	r.Route("/v1/teams/{teamId}/audit", func(r chi.Router) {
		r.Mount("/", auditLogH.TeamRoutes(authMW))
	})
	r.Mount("/v1/audit", auditLogH.MyRoutes(authMW))

	// GraphQL — read-only, auth-guarded
	gqlSrv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: gqlResolver}))
	r.With(authMW).Handle("/graphql", gqlSrv)
	if cfg.IsLocal() {
		r.Handle("/playground", playground.Handler("GraphQL", "/graphql"))
	}

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
