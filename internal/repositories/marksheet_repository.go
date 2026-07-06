package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sli/backend/internal/domain"
)

var (
	ErrMarksheetNotFound = errors.New("marksheet not found")
)

type MarksheetRepository interface {
	Create(ctx context.Context, marksheet *domain.Marksheet) error
	FindByEvaluationScoreID(ctx context.Context, scoreID uuid.UUID) (*domain.Marksheet, error)
}

type marksheetRepository struct {
	db dbtx
}

func NewMarksheetRepository(db dbtx) MarksheetRepository {
	return &marksheetRepository{db: db}
}

func (r *marksheetRepository) Create(ctx context.Context, marksheet *domain.Marksheet) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO marksheets (evaluation_score_id, file_key, generated_at, generated_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		marksheet.EvaluationScoreID, marksheet.FileKey, marksheet.GeneratedAt, marksheet.GeneratedBy,
	).Scan(&marksheet.ID)
}

func (r *marksheetRepository) FindByEvaluationScoreID(ctx context.Context, scoreID uuid.UUID) (*domain.Marksheet, error) {
	var m domain.Marksheet
	err := r.db.QueryRow(ctx, `
		SELECT id, evaluation_score_id, file_key, generated_at, generated_by
		FROM marksheets WHERE evaluation_score_id = $1`, scoreID,
	).Scan(&m.ID, &m.EvaluationScoreID, &m.FileKey, &m.GeneratedAt, &m.GeneratedBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMarksheetNotFound
		}
		return nil, err
	}
	return &m, nil
}
