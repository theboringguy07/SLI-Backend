package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/http/response"
	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/repositories"
	"github.com/sli/backend/internal/services"
)

type IndustryHandler struct {
	tokenService   services.TokenService
	feedbackRepo   repositories.FeedbackRepository
	internshipRepo repositories.InternshipRepository
}

func NewIndustryHandler(
	tokenService services.TokenService,
	feedbackRepo repositories.FeedbackRepository,
	internshipRepo repositories.InternshipRepository,
) *IndustryHandler {
	return &IndustryHandler{
		tokenService:   tokenService,
		feedbackRepo:   feedbackRepo,
		internshipRepo: internshipRepo,
	}
}

func (h *IndustryHandler) ViewReport(w http.ResponseWriter, r *http.Request) {
	rawToken := chi.URLParam(r, "token")
	if rawToken == "" {
		appErr := errors.New(errors.CodeBadRequest, "Missing token")
		response.Error(w, r, appErr)
		return
	}

	token, err := h.tokenService.ValidateToken(r.Context(), rawToken)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	// We return the report associated with the token.
	// Since FindByHash preloads the Report, we can just return it.
	response.JSON(w, http.StatusOK, token.Report)
}

func (h *IndustryHandler) SubmitFeedback(w http.ResponseWriter, r *http.Request) {
	rawToken := chi.URLParam(r, "token")
	if rawToken == "" {
		appErr := errors.New(errors.CodeBadRequest, "Missing token")
		response.Error(w, r, appErr)
		return
	}

	token, err := h.tokenService.ValidateToken(r.Context(), rawToken)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var req struct {
		FeedbackText string `json:"feedback_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appErr := errors.New(errors.CodeBadRequest, "Invalid JSON payload")
		response.Error(w, r, appErr)
		return
	}

	// We need the industry mentor's email to attach it to the feedback.
	// We can get it from the internship associated with the report.
	internship, err := h.internshipRepo.FindByID(r.Context(), token.Report.InternshipID)
	if err != nil {
		response.Error(w, r, errors.NewWithErr(errors.CodeInternalServer, "failed to find internship", err))
		return
	}

	feedback := &domain.ReportFeedback{
		ReportID:      token.ReportID,
		Source:        domain.FeedbackSourceIndustry,
		IndustryEmail: internship.IndustryMentorEmail,
		Comments:      req.FeedbackText,
		SubmittedAt:   time.Now(),
		CreatedAt:     time.Now(),
	}

	if err := h.feedbackRepo.Create(r.Context(), feedback); err != nil {
		response.Error(w, r, errors.NewWithErr(errors.CodeInternalServer, "failed to submit feedback", err))
		return
	}

	// Mark token as used
	_ = h.tokenService.MarkTokenUsed(r.Context(), token.ID)

	response.JSON(w, http.StatusCreated, feedback)
}
