package http

import (
	"github.com/go-chi/chi/v5"
	"github.com/sli/backend/internal/config"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/handlers"
	customMiddleware "github.com/sli/backend/internal/http/middleware"
	"github.com/sli/backend/internal/platform/auth"
	"github.com/sli/backend/internal/repositories"
)

// NewRouter initializes the Chi router with all standard middlewares
func NewRouter(
	cfg *config.Config,
	healthHandler *handlers.HealthHandler,
	authHandler *handlers.AuthHandler,
	coordinatorHandler *handlers.CoordinatorHandler,
	facultyHandler *handlers.FacultyHandler,
	studentHandler *handlers.StudentHandler,
	industryHandler *handlers.IndustryHandler,
	evaluationHandler *handlers.EvaluationHandler,
	hodHandler *handlers.HODHandler,
	adminHandler *handlers.AdminHandler,
	jwtService auth.JWTService,
	revokedRepo repositories.RevokedTokenRepository,
) *chi.Mux {
	r := chi.NewRouter()

	// 1. Standard Middlewares
	r.Use(customMiddleware.RequestID)
	r.Use(customMiddleware.Recoverer)
	r.Use(customMiddleware.Logger)
	// cfg.AllowedDomain restricts which Google Workspace email domains may log
	// in - it is not a URL and must not be used here. CORS is restricted to the
	// actual frontend origin(s) that are allowed to make credentialed requests.
	r.Use(customMiddleware.CORS(cfg.CORSAllowedOrigins()...))

	// Public routes
	r.Group(func(r chi.Router) {
		r.Use(customMiddleware.RateLimit(cfg.RateLimitPublic))

		r.Get("/healthz", healthHandler.Healthz)
		r.Get("/readyz", healthHandler.Readyz)

		r.Get("/api/auth/google/login", authHandler.GoogleLogin)
		r.Get("/api/auth/google/callback", authHandler.GoogleCallback)
		r.Post("/api/auth/magic-link/request", authHandler.RequestMagicLink)
		r.Get("/api/auth/magic-link/verify", authHandler.VerifyMagicLink)
	})

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(customMiddleware.RateLimit(cfg.RateLimitAuth))
		r.Use(customMiddleware.Auth(jwtService, revokedRepo))
		r.Use(customMiddleware.NoCache)
		r.Use(customMiddleware.CSRF(string(cfg.CSRFSecret())))

		r.Route("/api", func(r chi.Router) {
			r.Get("/auth/csrf", authHandler.GetCSRF)
			r.Post("/auth/logout", authHandler.Logout)
			r.Get("/auth/me", authHandler.GetMe)

			// Shared internship detail - readable by anyone who might
			// legitimately need to see one internship's record (not
			// coordinator-specific, hence its own group rather than living
			// under /coordinator).
			r.Group(func(r chi.Router) {
				r.Use(customMiddleware.RBAC(domain.RoleFaculty, domain.RoleCoordinator, domain.RoleAdmin))
				r.Get("/internships/{id}", coordinatorHandler.GetInternship)
			})

			// Report feedback - readable by students (their own report) and
			// faculty (their assigned students' reports).
			r.Group(func(r chi.Router) {
				r.Use(customMiddleware.RBAC(domain.RoleStudent, domain.RoleFaculty, domain.RoleAdmin))
				r.Get("/reports/{reportID}/feedback", facultyHandler.GetReportFeedback)
			})

			// Coordinator routes
			r.Group(func(r chi.Router) {
				r.Use(customMiddleware.RBAC(domain.RoleCoordinator, domain.RoleAdmin))
				r.Post("/coordinator/internships", coordinatorHandler.EnrollStudent)
				r.Post("/coordinator/internships/assign", coordinatorHandler.AssignFaculty)
				r.Get("/coordinator/internships", coordinatorHandler.ListInternships)
				r.Get("/coordinator/faculty", coordinatorHandler.ListFaculty)
				r.Get("/coordinator/reports", coordinatorHandler.ListReports)
			})

			// Student routes
			r.Group(func(r chi.Router) {
				r.Use(customMiddleware.RBAC(domain.RoleStudent, domain.RoleAdmin))
				r.Post("/student/reports/{type}/{period}", studentHandler.SubmitReport)
				r.Put("/student/reports/{type}/{period}", studentHandler.EditReport)
				r.Get("/student/reports", studentHandler.GetReports)
				r.Get("/student/internship", studentHandler.GetInternship)
			})

			// Faculty routes
			r.Group(func(r chi.Router) {
				r.Use(customMiddleware.RBAC(domain.RoleFaculty, domain.RoleAdmin))
				r.Get("/faculty/students", facultyHandler.ListStudents)
				r.Get("/faculty/students/{internshipID}/reports", facultyHandler.GetStudentReports)
				r.Post("/faculty/students/approve", facultyHandler.ApproveStudent)
				r.Post("/faculty/reports/{reportID}/feedback", facultyHandler.SubmitReportFeedback)
				r.Get("/faculty/evaluations/{internshipID}", evaluationHandler.GetEvaluation)
				r.Post("/faculty/evaluations/{internshipID}/schedule", evaluationHandler.SetSchedule)
				r.Post("/faculty/evaluations/{internshipID}/submit", evaluationHandler.SubmitScores)
				r.Get("/faculty/evaluations/{internshipID}/marksheet/download", evaluationHandler.DownloadMarksheet)
			})

			// HOD routes. There is no separate HOD role in the database (see
			// database/schema/schema.sql) - these are gated as admin-only.
			r.Group(func(r chi.Router) {
				r.Use(customMiddleware.RBAC(domain.RoleAdmin))
				r.Get("/hod/statistics", hodHandler.GetStatistics)
				r.Get("/hod/overview", hodHandler.GetOverview)
			})

			// Admin routes
			r.Group(func(r chi.Router) {
				r.Use(customMiddleware.RBAC(domain.RoleAdmin))
				r.Get("/admin/audit-logs", adminHandler.ListAuditLogs)
				r.Get("/admin/users", adminHandler.ListUsers)
				r.Patch("/admin/users/{userID}", adminHandler.UpdateUserProfile)
				r.Post("/admin/evaluations/{internshipID}/correct", evaluationHandler.AdminCorrectMarks)
			})
		})
	})

	// Industry routes (Public but secured by token)
	r.Group(func(r chi.Router) {
		r.Use(customMiddleware.RateLimit(cfg.RateLimitPublic))
		r.Use(customMiddleware.NoCache)
		r.Get("/api/industry/reports/{token}", industryHandler.ViewReport)
		r.Post("/api/industry/reports/{token}/feedback", industryHandler.SubmitFeedback)
		r.Get("/industry/reports/{token}", industryHandler.ViewReport)
		r.Post("/industry/reports/{token}/feedback", industryHandler.SubmitFeedback)
	})

	return r
}
