package domain

import (
	"time"

	"github.com/google/uuid"
)

// RevokedToken is used to blacklist JWTs that have been logged out before their natural expiry.
type RevokedToken struct {
	ID        uuid.UUID
	JTI       string
	UserID    uuid.UUID
	ExpiresAt time.Time
	RevokedAt time.Time
}
