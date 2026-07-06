package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/platform/pdf"
	"github.com/sli/backend/internal/platform/storage"
	"github.com/sli/backend/internal/repositories"
)

type MarksheetService interface {
	GenerateMarksheet(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) error
	GetMarksheetContent(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) ([]byte, error)
}

type marksheetService struct {
	marksheetRepo  repositories.MarksheetRepository
	evalRepo       repositories.EvaluationRepository
	internshipRepo repositories.InternshipRepository
	userRepo       repositories.UserRepository
	pdfGen         pdf.Generator
	store          storage.Storage
}

func NewMarksheetService(
	marksheetRepo repositories.MarksheetRepository,
	evalRepo repositories.EvaluationRepository,
	internshipRepo repositories.InternshipRepository,
	userRepo repositories.UserRepository,
	pdfGen pdf.Generator,
	store storage.Storage,
) MarksheetService {
	return &marksheetService{
		marksheetRepo:  marksheetRepo,
		evalRepo:       evalRepo,
		internshipRepo: internshipRepo,
		userRepo:       userRepo,
		pdfGen:         pdfGen,
		store:          store,
	}
}

func (s *marksheetService) GenerateMarksheet(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) error {
	internship, err := s.internshipRepo.FindByID(ctx, internshipID)
	if err != nil { return err }

	schedule, err := s.evalRepo.GetSchedule(ctx, internshipID, examType)
	if err != nil { return err }

	score, err := s.evalRepo.GetScore(ctx, internshipID, examType)
	if err != nil { return err }

	faculty, err := s.userRepo.FindByID(ctx, score.SubmittedBy)
	if err != nil { return err }

	pdfBytes, err := s.pdfGen.GenerateMarksheet(ctx, internship, schedule, score, faculty)
	if err != nil { return err }

	fileKey := s.store.GenerateFileKey(internshipID, examType)
	err = s.store.Save(ctx, fileKey, pdfBytes)
	if err != nil { return err }

	// Ensure we only have one marksheet record
	existing, _ := s.marksheetRepo.FindByEvaluationScoreID(ctx, score.ID)
	if existing == nil {
		marksheet := &domain.Marksheet{
			EvaluationScoreID: score.ID,
			FileKey:           fileKey,
			GeneratedAt:       time.Now(),
			// marksheets.generated_by is NOT NULL (references users.id): use
			// the faculty member whose submission triggered this generation.
			GeneratedBy: score.SubmittedBy,
		}
		return s.marksheetRepo.Create(ctx, marksheet)
	}

	return nil
}

func (s *marksheetService) GetMarksheetContent(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) ([]byte, error) {
	score, err := s.evalRepo.GetScore(ctx, internshipID, examType)
	if err != nil { return nil, err }

	marksheet, err := s.marksheetRepo.FindByEvaluationScoreID(ctx, score.ID)
	if err != nil { return nil, err }

	return s.store.Get(ctx, marksheet.FileKey)
}
