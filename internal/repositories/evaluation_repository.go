package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sli/backend/internal/domain"
)

var (
	ErrEvaluationNotFound = errors.New("evaluation not found")
)

type EvaluationRepository interface {
	SetSchedule(ctx context.Context, schedule *domain.EvaluationSchedule) error
	GetSchedule(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) (*domain.EvaluationSchedule, error)
	SubmitScores(ctx context.Context, score *domain.EvaluationScore) error
	GetScore(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) (*domain.EvaluationScore, error)
	CreateCorrection(ctx context.Context, correction *domain.EvaluationCorrection) error
	RunInTransaction(ctx context.Context, fn func(txRepo EvaluationRepository) error) error
}

type evaluationRepository struct {
	db dbtx
}

func NewEvaluationRepository(db dbtx) EvaluationRepository {
	return &evaluationRepository{db: db}
}

func (r *evaluationRepository) SetSchedule(ctx context.Context, schedule *domain.EvaluationSchedule) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO evaluation_schedules (internship_id, exam_type, in_semester_at, end_semester_at, venue, set_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT ON CONSTRAINT uq_es_internship_examtype
		DO UPDATE SET
			in_semester_at = EXCLUDED.in_semester_at,
			end_semester_at = EXCLUDED.end_semester_at,
			venue = EXCLUDED.venue,
			set_by = EXCLUDED.set_by
		RETURNING id, created_at, updated_at`,
		schedule.InternshipID, schedule.ExamType, schedule.InSemesterAt, schedule.EndSemesterAt, schedule.Venue, schedule.SetBy,
	).Scan(&schedule.ID, &schedule.CreatedAt, &schedule.UpdatedAt)
}

func (r *evaluationRepository) GetSchedule(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) (*domain.EvaluationSchedule, error) {
	var s domain.EvaluationSchedule
	err := r.db.QueryRow(ctx, `
		SELECT id, internship_id, exam_type, in_semester_at, end_semester_at, venue, set_by, created_at, updated_at
		FROM evaluation_schedules WHERE internship_id = $1 AND exam_type = $2`, internshipID, examType,
	).Scan(&s.ID, &s.InternshipID, &s.ExamType, &s.InSemesterAt, &s.EndSemesterAt, &s.Venue, &s.SetBy, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEvaluationNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *evaluationRepository) SubmitScores(ctx context.Context, score *domain.EvaluationScore) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO evaluation_scores (
			internship_id, exam_type, report_quality, oral_presentation, work_quality, understanding,
			periodic_interaction, remarks, locked_at, submitted_by, submitted_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`,
		score.InternshipID, score.ExamType, score.ReportQuality, score.OralPresentation, score.WorkQuality,
		score.Understanding, score.PeriodicInteraction, score.Remarks, score.LockedAt, score.SubmittedBy, score.SubmittedAt,
	).Scan(&score.ID)
}

func (r *evaluationRepository) GetScore(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) (*domain.EvaluationScore, error) {
	var s domain.EvaluationScore
	err := r.db.QueryRow(ctx, `
		SELECT id, internship_id, exam_type, report_quality, oral_presentation, work_quality, understanding,
			periodic_interaction, remarks, locked_at, submitted_by, submitted_at
		FROM evaluation_scores WHERE internship_id = $1 AND exam_type = $2`, internshipID, examType,
	).Scan(&s.ID, &s.InternshipID, &s.ExamType, &s.ReportQuality, &s.OralPresentation, &s.WorkQuality, &s.Understanding,
		&s.PeriodicInteraction, &s.Remarks, &s.LockedAt, &s.SubmittedBy, &s.SubmittedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEvaluationNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *evaluationRepository) CreateCorrection(ctx context.Context, correction *domain.EvaluationCorrection) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO evaluation_corrections (evaluation_score_id, old_scores_json, new_scores_json, reason, corrected_by, corrected_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		correction.EvaluationScoreID, correction.OldScoresJSON, correction.NewScoresJSON, correction.Reason,
		correction.CorrectedBy, correction.CorrectedAt,
	).Scan(&correction.ID)
}

func (r *evaluationRepository) RunInTransaction(ctx context.Context, fn func(txRepo EvaluationRepository) error) error {
	beginner, ok := r.db.(txBeginner)
	if !ok {
		return fn(r)
	}

	tx, err := beginner.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	txRepo := NewEvaluationRepository(tx)
	if err := fn(txRepo); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
