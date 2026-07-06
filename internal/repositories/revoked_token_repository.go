package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type RevokedTokenRepository interface {
	Revoke(ctx context.Context, jti string, userID uuid.UUID, expiresAt time.Time) error
	IsRevoked(ctx context.Context, jti string) bool
	CleanupExpired(ctx context.Context) error
}

type revokedTokenRepository struct {
	db dbtx
}

func NewRevokedTokenRepository(db dbtx) RevokedTokenRepository {
	return &revokedTokenRepository{db: db}
}

func (r *revokedTokenRepository) Revoke(ctx context.Context, jti string, userID uuid.UUID, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO revoked_tokens (jti, user_id, expires_at, revoked_at)
		VALUES ($1, $2, $3, $4)`,
		jti, userID, expiresAt, time.Now(),
	)
	return err
}

func (r *revokedTokenRepository) IsRevoked(ctx context.Context, jti string) bool {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM revoked_tokens WHERE jti = $1`, jti).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

func (r *revokedTokenRepository) CleanupExpired(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `DELETE FROM revoked_tokens WHERE expires_at < $1`, time.Now())
	return err
}
