package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sli/backend/internal/domain"
)

var ErrMagicLinkNotFound = errors.New("magic link token not found")

type MagicLinkRepository interface {
	Create(ctx context.Context, token *domain.MagicLinkToken) error
	FindByHash(ctx context.Context, hash string) (*domain.MagicLinkToken, error)
	MarkUsed(ctx context.Context, id uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

type magicLinkRepository struct {
	db dbtx
}

func NewMagicLinkRepository(db dbtx) MagicLinkRepository {
	return &magicLinkRepository{db: db}
}

func (r *magicLinkRepository) Create(ctx context.Context, token *domain.MagicLinkToken) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO magic_link_tokens (email, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		token.Email, token.TokenHash, token.ExpiresAt, time.Now(),
	).Scan(&token.ID)
}

func (r *magicLinkRepository) FindByHash(ctx context.Context, hash string) (*domain.MagicLinkToken, error) {
	var t domain.MagicLinkToken
	err := r.db.QueryRow(ctx, `
		SELECT id, email, token_hash, expires_at, used_at, created_at
		FROM magic_link_tokens WHERE token_hash = $1`, hash,
	).Scan(&t.ID, &t.Email, &t.TokenHash, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMagicLinkNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *magicLinkRepository) MarkUsed(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE magic_link_tokens SET used_at = $1 WHERE id = $2`, time.Now(), id)
	return err
}

func (r *magicLinkRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `DELETE FROM magic_link_tokens WHERE expires_at < $1`, time.Now())
	return err
}
