package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sli/backend/internal/config"
	"github.com/sli/backend/internal/handlers"
	internalHttp "github.com/sli/backend/internal/http"
	"github.com/sli/backend/internal/jobs"
	"github.com/sli/backend/internal/platform/auth"
	"github.com/sli/backend/internal/platform/db"
	"github.com/sli/backend/internal/platform/logger"
	"github.com/sli/backend/internal/platform/mailer"
	"github.com/sli/backend/internal/platform/pdf"
	"github.com/sli/backend/internal/platform/storage"
	"github.com/sli/backend/internal/repositories"
	"github.com/sli/backend/internal/services"
)

func main() {
	// 1. Setup structured logger
	logger.Setup()
	slog.Info("Starting SLI Backend Server")

	// 2. Load configuration
	cfg := config.Load()
	slog.Info("Configuration loaded", "port", cfg.Port)

	// 3. Initialize Database
	database, err := db.Connect(cfg.DBDSN, cfg.IsProduction())
	if err != nil {
		slog.Error("Failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	// Apply database/schema/schema.sql out of band; this only seeds the
	// fixed roles table (see internal/platform/db/migrate.go).
	if err := db.Migrate(database); err != nil {
		slog.Error("Failed to seed roles", "err", err)
		os.Exit(1)
	}

	// Initialize repositories
	userRepo := repositories.NewUserRepository(database)
	revokedTokenRepo := repositories.NewRevokedTokenRepository(database)
	internshipRepo := repositories.NewInternshipRepository(database)
	assignmentRepo := repositories.NewMentorAssignmentRepository(database)
	reportRepo := repositories.NewReportRepository(database)
	feedbackRepo := repositories.NewFeedbackRepository(database)
	tokenRepo := repositories.NewTokenRepository(database)
	outboxRepo := repositories.NewOutboxRepository(database)
	evalRepo := repositories.NewEvaluationRepository(database)
	marksheetRepo := repositories.NewMarksheetRepository(database)
	auditRepo := repositories.NewAuditRepository(database)
	magicLinkRepo := repositories.NewMagicLinkRepository(database)

	// Initialize platform utilities
	oauthService := auth.NewOAuthService(cfg)
	jwtService := auth.NewJWTService(cfg)
	pdfGen := pdf.NewPDFGenerator()
	store := storage.NewLocalStorage(cfg)
	smtpMailer := mailer.NewSMTPMailer(cfg)

	// Initialize business services
	notificationService := services.NewNotificationService(outboxRepo, cfg.FrontendURL, cfg.APIBaseURL)
	authService := services.NewAuthService(
		oauthService, jwtService, userRepo, revokedTokenRepo, auditRepo,
		magicLinkRepo, notificationService, cfg.AllowedDomain, cfg.MagicLinkExpiryMin,
	)
	internshipService := services.NewInternshipService(internshipRepo, assignmentRepo, userRepo)
	tokenService := services.NewTokenService(tokenRepo)
	reportService := services.NewReportService(reportRepo, internshipRepo, assignmentRepo, userRepo, tokenService, notificationService, cfg.ReportEditWindowHours)
	feedbackService := services.NewFeedbackService(feedbackRepo, reportRepo, assignmentRepo)
	marksheetService := services.NewMarksheetService(marksheetRepo, evalRepo, internshipRepo, userRepo, pdfGen, store)
	evalService := services.NewEvaluationService(evalRepo, internshipRepo, assignmentRepo, auditRepo, marksheetService)

	// Start Background Jobs
	ctx := context.Background()

	outboxDispatcher := jobs.NewOutboxDispatcher(outboxRepo, smtpMailer)
	go outboxDispatcher.Start(ctx)

	tokenCleanupWorker := jobs.NewTokenCleanupWorker(tokenService, magicLinkRepo)
	go tokenCleanupWorker.Start(ctx)

	reminderWorker := jobs.NewReminderWorker(notificationService, internshipRepo, reportRepo)
	go reminderWorker.Start(ctx)

	// 4. Initialize Handlers
	healthHandler := handlers.NewHealthHandler()
	authHandler := handlers.NewAuthHandler(oauthService, jwtService, authService, cfg.FrontendURL, string(cfg.CSRFSecret()))
	coordinatorHandler := handlers.NewCoordinatorHandler(internshipService, userRepo, reportService)
	facultyHandler := handlers.NewFacultyHandler(internshipService, feedbackService, reportService)
	studentHandler := handlers.NewStudentHandler(reportService, internshipService)
	industryHandler := handlers.NewIndustryHandler(tokenService, feedbackRepo, internshipRepo)
	evaluationHandler := handlers.NewEvaluationHandler(evalService, marksheetService)
	hodHandler := handlers.NewHODHandler(internshipRepo)
	adminHandler := handlers.NewAdminHandler(auditRepo, userRepo)

	// 5. Setup Router
	r := internalHttp.NewRouter(
		cfg,
		healthHandler,
		authHandler,
		coordinatorHandler,
		facultyHandler,
		studentHandler,
		industryHandler,
		evaluationHandler,
		hodHandler,
		adminHandler,
		jwtService,
		revokedTokenRepo,
	)

	// 6. Start Server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		slog.Info("Server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed", "err", err)
			os.Exit(1)
		}
	}()

	// 7. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "err", err)
		os.Exit(1)
	}

	slog.Info("Server exited properly")
}
