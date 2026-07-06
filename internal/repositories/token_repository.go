package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sli/backend/internal/domain"
)

var (
	ErrTokenNotFound = errors.New("access token not found")
)

type TokenRepository interface {
	Create(ctx context.Context, token *domain.IndustryAccessToken) error
	FindByHash(ctx context.Context, hash string) (*domain.IndustryAccessToken, error)
	MarkUsed(ctx context.Context, id uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

type tokenRepository struct {
	db dbtx
}

func NewTokenRepository(db dbtx) TokenRepository {
	return &tokenRepository{db: db}
}

func (r *tokenRepository) Create(ctx context.Context, token *domain.IndustryAccessToken) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO industry_access_tokens (report_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		token.ReportID, token.TokenHash, token.ExpiresAt, time.Now(),
	).Scan(&token.ID)
}

// FindByHash also joins in the report it's for (matching the previous
// Preload("Report")) - callers read token.Report directly.
func (r *tokenRepository) FindByHash(ctx context.Context, hash string) (*domain.IndustryAccessToken, error) {
	row := r.db.QueryRow(ctx, `
		SELECT
			t.id, t.report_id, t.token_hash, t.expires_at, t.used_at, t.created_at,
			`+reportSelectCols+`
		FROM industry_access_tokens t
		JOIN reports rp ON rp.id = t.report_id
		WHERE t.token_hash = $1`, hash)

	var tok domain.IndustryAccessToken
	var rpt domain.WeeklyReport
	err := row.Scan(
		&tok.ID, &tok.ReportID, &tok.TokenHash, &tok.ExpiresAt, &tok.UsedAt, &tok.CreatedAt,
		&rpt.ID, &rpt.InternshipID, &rpt.ReportType, &rpt.WeekNumber, &rpt.Content, &rpt.Status,
		&rpt.SubmittedAt, &rpt.EditedAt, &rpt.ApprovedAt, &rpt.ApprovedBy, &rpt.ReminderSentAt,
		&rpt.CreatedBy, &rpt.UpdatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}
	tok.Report = &rpt
	return &tok, nil
}

func (r *tokenRepository) MarkUsed(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE industry_access_tokens SET used_at = $1 WHERE id = $2`, time.Now(), id)
	return err
}

func (r *tokenRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `DELETE FROM industry_access_tokens WHERE expires_at < $1`, time.Now())
	return err
}
