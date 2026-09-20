package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"

	"whisperledger-backend/internal/config"
	"whisperledger-backend/internal/domain"
	v1 "whisperledger-backend/internal/handler/v1"
	"whisperledger-backend/internal/middleware"
	"whisperledger-backend/internal/pkg/response"
)

type RouterConfig struct {
	Config           *config.Config
	Logger           *zap.Logger
	AuthMiddleware   *middleware.AuthMiddleware
	AuthHandler      *v1.AuthHandler
	ExpenseHandler   *v1.ExpenseHandler
	HouseholdHandler *v1.HouseholdHandler
	RecoveryHandler  *v1.RecoveryHandler
	DetectiveHandler *v1.DetectiveHandler
	AdminHandler     *v1.AdminHandler
}

func NewRouter(rc RouterConfig) http.Handler {
	r := chi.NewRouter()

	// Base middlewares
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestLogger(rc.Logger))
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   rc.Config.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		response.Success(w, http.StatusOK, "WhisperLedger API is operational", map[string]interface{}{
			"service":   "whisperledger-backend",
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
		})
	}

	// Health Checks (supporting standard /health and Render /healthz)
	r.Get("/health", healthHandler)
	r.Get("/healthz", healthHandler)

	// API v1
	r.Route("/api/v1", func(api chi.Router) {
		// Public Auth routes
		api.Route("/auth", func(auth chi.Router) {
			auth.Post("/register", rc.AuthHandler.Register)
			auth.Post("/login", rc.AuthHandler.Login)
			auth.Post("/refresh", rc.AuthHandler.Refresh)

			// Protected auth route
			auth.Group(func(protected chi.Router) {
				protected.Use(rc.AuthMiddleware.Authenticate)
				protected.Get("/me", rc.AuthHandler.Me)
			})
		})

		// Protected user routes
		api.Group(func(protected chi.Router) {
			protected.Use(rc.AuthMiddleware.Authenticate)

			// Expenses & 3-Way Outflow Ledger
			protected.Route("/expenses", func(exp chi.Router) {
				exp.Post("/", rc.ExpenseHandler.Create)
				exp.Get("/", rc.ExpenseHandler.List)
				exp.Get("/summary", rc.ExpenseHandler.Summary)
				exp.Get("/{id}", rc.ExpenseHandler.GetByID)
				exp.Delete("/{id}", rc.ExpenseHandler.Delete)
			})

			// Households & Shared living
			protected.Route("/households", func(hh chi.Router) {
				hh.Post("/", rc.HouseholdHandler.Create)
				hh.Post("/join", rc.HouseholdHandler.Join)
				hh.Get("/me", rc.HouseholdHandler.GetMe)
				hh.Get("/{id}/balances", rc.HouseholdHandler.GetBalances)
				hh.Get("/{id}/settlements", rc.HouseholdHandler.GetOptimalSettlements)
				hh.Post("/{id}/settle", rc.HouseholdHandler.Settle)
			})

			// Money Recovery Engine
			protected.Route("/recovery", func(rec chi.Router) {
				rec.Post("/", rc.RecoveryHandler.Create)
				rec.Get("/", rc.RecoveryHandler.List)
				rec.Get("/summary", rc.RecoveryHandler.Summary)
				rec.Post("/{id}/settle", rc.RecoveryHandler.Settle)
			})

			// Money Leak Detective & Autopilot
			protected.Route("/detective", func(det chi.Router) {
				det.Get("/leaks", rc.DetectiveHandler.DetectLeaks)
				det.Get("/safe-to-spend", rc.DetectiveHandler.SafeToSpend)
				det.Get("/household-audit", rc.DetectiveHandler.HouseholdAudit)
			})

			// Admin Portal (Guarded by PlatformAdmin / HouseholdAdmin)
			protected.Route("/admin", func(admin chi.Router) {
				admin.Use(rc.AuthMiddleware.RequireRole(domain.RolePlatformAdmin, domain.RoleHouseholdAdmin))
				admin.Get("/metrics", rc.AdminHandler.Metrics)
				admin.Get("/users", rc.AdminHandler.Users)
			})
		})
	})

	return r
}
