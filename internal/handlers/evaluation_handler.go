package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/http/middleware"
	"github.com/sli/backend/internal/http/response"
	"github.com/sli/backend/internal/platform/auth"
	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/services"
)

type EvaluationHandler struct {
	evalService      services.EvaluationService
	marksheetService services.MarksheetService
}

func NewEvaluationHandler(evalService services.EvaluationService, marksheetService services.MarksheetService) *EvaluationHandler {
	return &EvaluationHandler{
		evalService:      evalService,
		marksheetService: marksheetService,
	}
}

func (h *EvaluationHandler) SetSchedule(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)

	internshipIDStr := chi.URLParam(r, "internshipID")
	internshipID, err := uuid.Parse(internshipIDStr)
	if err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid internship ID"))
		return
	}

	var req struct {
		ExamType      string `json:"exam_type"`
		InSemesterAt  string `json:"in_semester_at"`
		EndSemesterAt string `json:"end_semester_at"`
		Venue         string `json:"venue"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid JSON payload"))
		return
	}

	examType, err := parseExamType(req.ExamType)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	layout := "2006-01-02"
	inSem, err1 := time.Parse(layout, req.InSemesterAt)
	endSem, err2 := time.Parse(layout, req.EndSemesterAt)
	if err1 != nil || err2 != nil {
		response.Error(w, r, errors.New(errors.CodeValidationFailed, "Invalid dates. Format should be YYYY-MM-DD"))
		return
	}

	schedule := &domain.EvaluationSchedule{
		InternshipID:  internshipID,
		ExamType:      examType,
		InSemesterAt:  inSem,
		EndSemesterAt: endSem,
		Venue:         req.Venue,
	}

	if err := h.evalService.SetSchedule(r.Context(), claims.UserID, schedule); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, schedule)
}

func (h *EvaluationHandler) SubmitScores(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)

	internshipIDStr := chi.URLParam(r, "internshipID")
	internshipID, err := uuid.Parse(internshipIDStr)
	if err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid internship ID"))
		return
	}

	var req struct {
		ExamType            string `json:"exam_type"`
		ReportQuality       int    `json:"report_quality"`
		OralPresentation    int    `json:"oral_presentation"`
		WorkQuality         int    `json:"work_quality"`
		Understanding       int    `json:"understanding"`
		PeriodicInteraction int    `json:"periodic_interaction"`
		Remarks             string `json:"remarks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid JSON payload"))
		return
	}

	examType, err := parseExamType(req.ExamType)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	score := &domain.EvaluationScore{
		InternshipID:        internshipID,
		ExamType:            examType,
		ReportQuality:       req.ReportQuality,
		OralPresentation:    req.OralPresentation,
		WorkQuality:         req.WorkQuality,
		Understanding:       req.Understanding,
		PeriodicInteraction: req.PeriodicInteraction,
		Remarks:             req.Remarks,
	}

	if err := h.evalService.SubmitMarks(r.Context(), claims.UserID, score); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, score)
}

// GetEvaluation is GET /api/faculty/evaluations/{internshipID}?exam_type=ISE|ESE
// - a read-only view of the schedule and (if submitted) the locked score,
// for the evaluation review UI. Reuses evalService.GetEvaluationDetail.
func (h *EvaluationHandler) GetEvaluation(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)

	internshipIDStr := chi.URLParam(r, "internshipID")
	internshipID, err := uuid.Parse(internshipIDStr)
	if err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid internship ID"))
		return
	}

	examType, err := parseExamType(r.URL.Query().Get("exam_type"))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.evalService.EnsureCanViewInternship(r.Context(), claims.UserID, claims.Role, internshipID); err != nil {
		response.Error(w, r, err)
		return
	}

	detail, err := h.evalService.GetEvaluationDetail(r.Context(), internshipID, examType)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, detail)
}

func (h *EvaluationHandler) DownloadMarksheet(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)

	internshipIDStr := chi.URLParam(r, "internshipID")
	internshipID, err := uuid.Parse(internshipIDStr)
	if err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid internship ID"))
		return
	}

	examType, err := parseExamType(r.URL.Query().Get("exam_type"))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.evalService.EnsureCanViewInternship(r.Context(), claims.UserID, claims.Role, internshipID); err != nil {
		response.Error(w, r, err)
		return
	}

	content, err := h.marksheetService.GetMarksheetContent(r.Context(), internshipID, examType)
	if err != nil {
		response.Error(w, r, errors.NewWithErr(errors.CodeNotFound, "marksheet not found or not generated", err))
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=marksheet.pdf")
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func (h *EvaluationHandler) AdminCorrectMarks(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)

	internshipIDStr := chi.URLParam(r, "internshipID")
	internshipID, err := uuid.Parse(internshipIDStr)
	if err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid internship ID"))
		return
	}

	var req struct {
		ExamType            string `json:"exam_type"`
		ReportQuality       int    `json:"report_quality"`
		OralPresentation    int    `json:"oral_presentation"`
		WorkQuality         int    `json:"work_quality"`
		Understanding       int    `json:"understanding"`
		PeriodicInteraction int    `json:"periodic_interaction"`
		Remarks             string `json:"remarks"`
		Reason              string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid JSON payload"))
		return
	}

	examType, err := parseExamType(req.ExamType)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	newScore := &domain.EvaluationScore{
		ReportQuality:       req.ReportQuality,
		OralPresentation:    req.OralPresentation,
		WorkQuality:         req.WorkQuality,
		Understanding:       req.Understanding,
		PeriodicInteraction: req.PeriodicInteraction,
		Remarks:             req.Remarks,
	}

	if err := h.evalService.CorrectMarks(r.Context(), claims.UserID, claims.Email, internshipID, examType, newScore, req.Reason); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "Marks corrected and audited"})
}

// parseExamType validates and converts a raw exam_type string (from a JSON
// body or query param) into a domain.ExamType, matching domain.AllExamTypes.
func parseExamType(raw string) (domain.ExamType, error) {
	candidate := domain.ExamType(raw)
	for _, valid := range domain.AllExamTypes {
		if candidate == valid {
			return candidate, nil
		}
	}
	return "", errors.New(errors.CodeValidationFailed, "exam_type must be ISE or ESE")
}
