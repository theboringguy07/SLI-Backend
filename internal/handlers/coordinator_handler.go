package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/http/middleware"
	"github.com/sli/backend/internal/http/response"
	"github.com/sli/backend/internal/platform/auth"
	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/repositories"
	"github.com/sli/backend/internal/services"
)

type CoordinatorHandler struct {
	internshipService services.InternshipService
	userRepo          repositories.UserRepository
	reportService     services.ReportService
}

func NewCoordinatorHandler(internshipService services.InternshipService, userRepo repositories.UserRepository, reportService services.ReportService) *CoordinatorHandler {
	return &CoordinatorHandler{
		internshipService: internshipService,
		userRepo:          userRepo,
		reportService:     reportService,
	}
}

// ListReports is GET /api/coordinator/reports - every report across every
// internship, for the coordinator's tracking view.
func (h *CoordinatorHandler) ListReports(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 100
	}

	reports, count, err := h.reportService.ListAllReports(r.Context(), offset, limit)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"data":  reports,
		"total": count,
	})
}

// facultyOption is a deliberately narrow projection of domain.User - a
// coordinator picking a mentor needs a name/email/department, not the full
// admin user record (which includes google_sub and other fields that
// shouldn't leak to a non-admin role).
type facultyOption struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Department string    `json:"department,omitempty"`
}

// ListFaculty is GET /api/coordinator/faculty - every FACULTY user, for the
// mentor-mapping UI's assignment dropdown. COORDINATOR has no access to
// GET /api/admin/users (ADMIN-only), so this exists as a narrower
// COORDINATOR-accessible alternative rather than widening that endpoint's
// RBAC (which would also expose google_sub and every other role's users).
func (h *CoordinatorHandler) ListFaculty(w http.ResponseWriter, r *http.Request) {
	users, err := h.userRepo.ListByRole(r.Context(), domain.RoleFaculty)
	if err != nil {
		response.Error(w, r, errors.NewWithErr(errors.CodeInternalServer, "failed to list faculty", err))
		return
	}

	options := make([]facultyOption, 0, len(users))
	for _, u := range users {
		options = append(options, facultyOption{ID: u.ID, Name: u.Name, Email: u.Email, Department: u.Department})
	}

	response.JSON(w, http.StatusOK, options)
}

func (h *CoordinatorHandler) EnrollStudent(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)

	var req services.EnrollStudentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appErr := errors.New(errors.CodeBadRequest, "Invalid JSON payload")
		response.Error(w, r, appErr)
		return
	}

	internship, err := h.internshipService.EnrollStudent(r.Context(), claims.UserID, req)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, internship)
}

func (h *CoordinatorHandler) AssignFaculty(w http.ResponseWriter, r *http.Request) {
	// Not implementing the full chi param extraction here for brevity,
	// assuming it's available via chi.URLParam or similar.
	// For example:
	// studentIDStr := chi.URLParam(r, "studentID")
	// For this code snippet, let's extract it from query or assume the service handles it.

	// Real implementation using chi:
	// studentID, err := uuid.Parse(chi.URLParam(r, "studentID"))

	var req struct {
		InternshipID    uuid.UUID `json:"internship_id"`
		FacultyMentorID uuid.UUID `json:"faculty_mentor_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appErr := errors.New(errors.CodeBadRequest, "Invalid JSON payload")
		response.Error(w, r, appErr)
		return
	}

	assignment, err := h.internshipService.AssignFacultyMentor(r.Context(), req.InternshipID, req.FacultyMentorID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, assignment)
}

// GetInternship is GET /api/internships/{id} - a single internship's detail
// (company, mentor, dates, status), shared across FACULTY/COORDINATOR/ADMIN
// (see internal/http/router.go). Not coordinator-specific despite living on
// this handler; it just reuses the same internshipService dependency.
func (h *CoordinatorHandler) GetInternship(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid internship ID"))
		return
	}

	internship, err := h.internshipService.GetInternshipByID(r.Context(), id)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, internship)
}

func (h *CoordinatorHandler) ListInternships(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}

	internships, count, err := h.internshipService.ListAllInternships(r.Context(), offset, limit)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"data":  internships,
		"total": count,
	})
}
