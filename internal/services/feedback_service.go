package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/repositories"
)

type FeedbackService interface {
	SubmitFacultyFeedback(ctx context.Context, facultyID uuid.UUID, reportID uuid.UUID, feedbackText string) (*domain.ReportFeedback, error)
	ListFeedbackForReport(ctx context.Context, reportID uuid.UUID) ([]domain.ReportFeedback, error)
	// Industry feedback will go here later
}

type feedbackService struct {
	feedbackRepo   repositories.FeedbackRepository
	reportRepo     repositories.ReportRepository
	assignmentRepo repositories.MentorAssignmentRepository
}

func NewFeedbackService(
	feedbackRepo repositories.FeedbackRepository,
	reportRepo repositories.ReportRepository,
	assignmentRepo repositories.MentorAssignmentRepository,
) FeedbackService {
	return &feedbackService{
		feedbackRepo:   feedbackRepo,
		reportRepo:     reportRepo,
		assignmentRepo: assignmentRepo,
	}
}

func (s *feedbackService) SubmitFacultyFeedback(ctx context.Context, facultyID uuid.UUID, reportID uuid.UUID, feedbackText string) (*domain.ReportFeedback, error) {
	report, err := s.reportRepo.FindByID(ctx, reportID)
	if err != nil {
		return nil, errors.New(errors.CodeNotFound, "report not found")
	}

	// Verify faculty owns this student's internship
	assignment, err := s.assignmentRepo.FindByInternshipID(ctx, report.InternshipID)
	if err != nil {
		return nil, errors.New(errors.CodeForbidden, "mentor assignment not found")
	}

	if assignment.FacultyMentorID != facultyID {
		return nil, errors.New(errors.CodeForbidden, "you are not the assigned mentor for this report")
	}

	feedback := &domain.ReportFeedback{
		ReportID:    report.ID,
		Source:      domain.FeedbackSourceFaculty,
		GivenBy:     &facultyID,
		Comments:    feedbackText,
		SubmittedAt: time.Now(),
		CreatedAt:   time.Now(),
	}

	if err := s.feedbackRepo.Create(ctx, feedback); err != nil {
		return nil, errors.NewWithErr(errors.CodeInternalServer, "failed to submit feedback", err)
	}

	return feedback, nil
}

func (s *feedbackService) ListFeedbackForReport(ctx context.Context, reportID uuid.UUID) ([]domain.ReportFeedback, error) {
	return s.feedbackRepo.ListByReport(ctx, reportID)
}
