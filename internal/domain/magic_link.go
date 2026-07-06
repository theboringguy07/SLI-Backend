package domain

import (
	"time"

	"github.com/google/uuid"
)

// MagicLinkToken is a single-use, short-lived passwordless login token e-mailed
// to a user who requested one via POST /api/auth/magic-link/request. Mirrors
// the same hashed-token pattern as IndustryAccessToken (internal/domain/token.go):
// only the SHA-256 hash is stored, the raw token is only ever held in memory
// long enough to email it.
type MagicLinkToken struct {
	ID        uuid.UUID  `json:"id"`
	Email     string     `json:"email"`
	TokenHash string     `json:"-"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at"`
	CreatedAt time.Time  `json:"created_at"`
}
