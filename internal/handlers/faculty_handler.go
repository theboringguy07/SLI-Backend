package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sli/backend/internal/http/middleware"
	"github.com/sli/backend/internal/http/response"
	"github.com/sli/backend/internal/platform/auth"
	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/services"
)

type FacultyHandler struct {
	internshipService services.InternshipService
	feedbackService   services.FeedbackService
	reportService     services.ReportService
}

func NewFacultyHandler(internshipService services.InternshipService, feedbackService services.FeedbackService, reportService services.ReportService) *FacultyHandler {
	return &FacultyHandler{
		internshipService: internshipService,
		feedbackService:   feedbackService,
		reportService:     reportService,
	}
}

// GetStudentReports is GET /api/faculty/students/{internshipID}/reports -
// every report (any type) a specific assigned student has submitted, for
// the mentor to review and leave feedback on.
func (h *FacultyHandler) GetStudentReports(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)

	internshipID, err := uuid.Parse(chi.URLParam(r, "internshipID"))
	if err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid internship ID"))
		return
	}

	reports, err := h.reportService.GetReportsForInternship(r.Context(), claims.UserID, internshipID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, reports)
}

func (h *FacultyHandler) ListStudents(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)

	assignments, err := h.internshipService.ListStudentsForFaculty(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, assignments)
}

func (h *FacultyHandler) ApproveStudent(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)

	var req struct {
		AssignmentID uuid.UUID `json:"assignment_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// As an alternative to body, maybe assignmentID is in URL: chi.URLParam(r, "assignmentID")
		// Let's support URL param instead as the plan says POST /api/faculty/students/{studentID}/approve
		// Wait, the plan says POST /api/faculty/students/{studentID}/approve but actually we approve the assignment.
		// Let's stick to the JSON payload for this snippet.
		appErr := errors.New(errors.CodeBadRequest, "Invalid JSON payload")
		response.Error(w, r, appErr)
		return
	}

	err := h.internshipService.ApproveStudent(r.Context(), req.AssignmentID, claims.UserID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Student approved successfully"})
}

// GetReportFeedback is GET /api/reports/{reportID}/feedback - shared by
// student (own report) and faculty (assigned student's report) views; see
// the route registration in internal/http/router.go for the exact RBAC.
func (h *FacultyHandler) GetReportFeedback(w http.ResponseWriter, r *http.Request) {
	reportID, err := uuid.Parse(chi.URLParam(r, "reportID"))
	if err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid report ID"))
		return
	}

	feedback, err := h.feedbackService.ListFeedbackForReport(r.Context(), reportID)
	if err != nil {
		response.Error(w, r, errors.NewWithErr(errors.CodeInternalServer, "failed to load feedback", err))
		return
	}

	response.JSON(w, http.StatusOK, feedback)
}

func (h *FacultyHandler) SubmitReportFeedback(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)

	reportID, err := uuid.Parse(chi.URLParam(r, "reportID"))
	if err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid report ID"))
		return
	}

	var req struct {
		FeedbackText string `json:"feedback_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid JSON payload"))
		return
	}

	feedback, err := h.feedbackService.SubmitFacultyFeedback(r.Context(), claims.UserID, reportID, req.FeedbackText)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, feedback)
}
