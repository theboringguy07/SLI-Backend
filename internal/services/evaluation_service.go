package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/repositories"
)

type EvaluationService interface {
	SetSchedule(ctx context.Context, facultyID uuid.UUID, schedule *domain.EvaluationSchedule) error
	SubmitMarks(ctx context.Context, facultyID uuid.UUID, score *domain.EvaluationScore) error
	GetEvaluationForInternship(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) (*domain.EvaluationScore, error)
	GetEvaluationDetail(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) (*EvaluationDetail, error)
	CorrectMarks(ctx context.Context, adminID uuid.UUID, adminName string, internshipID uuid.UUID, examType domain.ExamType, newScore *domain.EvaluationScore, reason string) error
}

// EvaluationDetail is the read-model for viewing an internship's evaluation
// state (as opposed to SetSchedule/SubmitMarks, which mutate it). Either
// field may be nil - a schedule can exist before any score is submitted, and
// there's no schedule/score at all before faculty ever set one.
type EvaluationDetail struct {
	Schedule *domain.EvaluationSchedule `json:"schedule"`
	Score    *domain.EvaluationScore    `json:"score"`
}

type evaluationService struct {
	evalRepo         repositories.EvaluationRepository
	internshipRepo   repositories.InternshipRepository
	assignmentRepo   repositories.MentorAssignmentRepository
	auditRepo        repositories.AuditRepository
	marksheetService MarksheetService
}

func NewEvaluationService(
	evalRepo repositories.EvaluationRepository,
	internshipRepo repositories.InternshipRepository,
	assignmentRepo repositories.MentorAssignmentRepository,
	auditRepo repositories.AuditRepository,
	marksheetService MarksheetService,
) EvaluationService {
	return &evaluationService{
		evalRepo:         evalRepo,
		internshipRepo:   internshipRepo,
		assignmentRepo:   assignmentRepo,
		auditRepo:        auditRepo,
		marksheetService: marksheetService,
	}
}

func (s *evaluationService) SetSchedule(ctx context.Context, facultyID uuid.UUID, schedule *domain.EvaluationSchedule) error {
	assignment, err := s.assignmentRepo.FindByInternshipID(ctx, schedule.InternshipID)
	if err != nil {
		return errors.New(errors.CodeNotFound, "internship assignment not found")
	}

	if assignment.FacultyMentorID != facultyID {
		return errors.New(errors.CodeForbidden, "you are not the assigned mentor")
	}

	schedule.SetBy = facultyID
	if err := s.evalRepo.SetSchedule(ctx, schedule); err != nil {
		return errors.NewWithErr(errors.CodeInternalServer, "failed to set schedule", err)
	}

	return nil
}

func (s *evaluationService) SubmitMarks(ctx context.Context, facultyID uuid.UUID, score *domain.EvaluationScore) error {
	assignment, err := s.assignmentRepo.FindByInternshipID(ctx, score.InternshipID)
	if err != nil {
		return errors.New(errors.CodeNotFound, "internship assignment not found")
	}

	if assignment.FacultyMentorID != facultyID {
		return errors.New(errors.CodeForbidden, "you are not the assigned mentor")
	}

	// Check if already submitted
	existing, err := s.evalRepo.GetScore(ctx, score.InternshipID, score.ExamType)
	if err == nil && existing != nil {
		return errors.New(errors.CodeReportAlreadyLocked, "evaluation marks already submitted and locked")
	}

	// Lock the marks
	now := time.Now()
	score.LockedAt = &now
	score.SubmittedBy = facultyID
	score.SubmittedAt = now

	err = s.evalRepo.RunInTransaction(ctx, func(txRepo repositories.EvaluationRepository) error {
		if err := txRepo.SubmitScores(ctx, score); err != nil {
			return err
		}

		// Audit Log
		meta, _ := json.Marshal(map[string]interface{}{"internship_id": score.InternshipID})
		auditLog := &domain.AuditLog{
			ActorUserID:  facultyID,
			ActorName:    "Faculty", // Should fetch actual name ideally
			Action:       "submit_evaluation",
			ResourceType: "evaluation_scores",
			ResourceID:   score.ID,
			MetadataJSON: string(meta),
			CreatedAt:    time.Now(),
		}

		// In a real implementation we would inject auditRepo into the transaction context.
		// For simplicity, we just use the global auditRepo, which is fine if we use the same DB.
		_ = s.auditRepo.Create(ctx, auditLog)

		return nil
	})
	if err != nil {
		return err
	}

	if s.marksheetService != nil {
		if err := s.marksheetService.GenerateMarksheet(ctx, score.InternshipID, score.ExamType); err != nil {
			return errors.NewWithErr(errors.CodeInternalServer, "failed to generate marksheet", err)
		}
	}

	return nil
}

func (s *evaluationService) GetEvaluationForInternship(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) (*domain.EvaluationScore, error) {
	score, err := s.evalRepo.GetScore(ctx, internshipID, examType)
	if err != nil {
		if err == repositories.ErrEvaluationNotFound {
			return nil, errors.New(errors.CodeNotFound, "evaluation not found")
		}
		return nil, errors.NewWithErr(errors.CodeInternalServer, "database error", err)
	}
	return score, nil
}

func (s *evaluationService) GetEvaluationDetail(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) (*EvaluationDetail, error) {
	detail := &EvaluationDetail{}

	schedule, err := s.evalRepo.GetSchedule(ctx, internshipID, examType)
	if err != nil && err != repositories.ErrEvaluationNotFound {
		return nil, errors.NewWithErr(errors.CodeInternalServer, "database error", err)
	}
	detail.Schedule = schedule

	score, err := s.evalRepo.GetScore(ctx, internshipID, examType)
	if err != nil && err != repositories.ErrEvaluationNotFound {
		return nil, errors.NewWithErr(errors.CodeInternalServer, "database error", err)
	}
	detail.Score = score

	if detail.Schedule == nil && detail.Score == nil {
		return nil, errors.New(errors.CodeNotFound, "no schedule or evaluation set for this exam type yet")
	}

	return detail, nil
}

func (s *evaluationService) CorrectMarks(ctx context.Context, adminID uuid.UUID, adminName string, internshipID uuid.UUID, examType domain.ExamType, newScore *domain.EvaluationScore, reason string) error {
	score, err := s.evalRepo.GetScore(ctx, internshipID, examType)
	if err != nil {
		return errors.New(errors.CodeNotFound, "evaluation not found")
	}

	oldScoresJSON, _ := json.Marshal(score)

	correctedScore := *score
	correctedScore.ReportQuality = newScore.ReportQuality
	correctedScore.OralPresentation = newScore.OralPresentation
	correctedScore.WorkQuality = newScore.WorkQuality
	correctedScore.Understanding = newScore.Understanding
	correctedScore.PeriodicInteraction = newScore.PeriodicInteraction
	correctedScore.Remarks = newScore.Remarks

	newScoresJSON, _ := json.Marshal(correctedScore)

	correction := &domain.EvaluationCorrection{
		EvaluationScoreID: score.ID,
		OldScoresJSON:     string(oldScoresJSON),
		NewScoresJSON:     string(newScoresJSON),
		Reason:            reason,
		CorrectedBy:       adminID,
		CorrectedAt:       time.Now(),
	}

	err = s.evalRepo.CreateCorrection(ctx, correction)
	if err != nil {
		return errors.NewWithErr(errors.CodeInternalServer, "failed to save correction", err)
	}

	meta, _ := json.Marshal(map[string]interface{}{
		"internship_id": score.InternshipID,
		"reason":        reason,
	})
	auditLog := &domain.AuditLog{
		ActorUserID:  adminID,
		ActorName:    adminName,
		Action:       "correct_evaluation",
		ResourceType: "evaluation_scores",
		ResourceID:   score.ID,
		MetadataJSON: string(meta),
		CreatedAt:    time.Now(),
	}
	_ = s.auditRepo.Create(ctx, auditLog)

	return nil
}
