package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/http/middleware"
	"github.com/sli/backend/internal/http/response"
	"github.com/sli/backend/internal/platform/auth"
	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/services"
)

type StudentHandler struct {
	reportService     services.ReportService
	internshipService services.InternshipService
}

func NewStudentHandler(reportService services.ReportService, internshipService services.InternshipService) *StudentHandler {
	return &StudentHandler{
		reportService:     reportService,
		internshipService: internshipService,
	}
}

// GetInternship is GET /api/student/internship - the student's own internship
// record (company, mentor, dates, status). Reuses
// InternshipService.GetStudentInternship, which already existed for internal
// use by ReportService but had no HTTP route pointing at it.
func (h *StudentHandler) GetInternship(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)

	internship, err := h.internshipService.GetStudentInternship(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, internship)
}

func (h *StudentHandler) SubmitReport(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)

	reportType := domain.ReportType(chi.URLParam(r, "type"))

	periodStr := chi.URLParam(r, "period")
	period, err := strconv.Atoi(periodStr)
	if err != nil {
		appErr := errors.New(errors.CodeBadRequest, "Invalid period number")
		response.Error(w, r, appErr)
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appErr := errors.New(errors.CodeBadRequest, "Invalid JSON payload")
		response.Error(w, r, appErr)
		return
	}

	report, err := h.reportService.SubmitReport(r.Context(), claims.UserID, reportType, period, req.Content)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, report)
}

func (h *StudentHandler) EditReport(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)

	reportType := domain.ReportType(chi.URLParam(r, "type"))

	periodStr := chi.URLParam(r, "period")
	period, err := strconv.Atoi(periodStr)
	if err != nil {
		appErr := errors.New(errors.CodeBadRequest, "Invalid period number")
		response.Error(w, r, appErr)
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appErr := errors.New(errors.CodeBadRequest, "Invalid JSON payload")
		response.Error(w, r, appErr)
		return
	}

	report, err := h.reportService.EditReport(r.Context(), claims.UserID, reportType, period, req.Content)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, report)
}

func (h *StudentHandler) GetReports(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)

	reports, err := h.reportService.GetReports(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, reports)
}
