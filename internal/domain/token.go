package domain

import (
	"time"

	"github.com/google/uuid"
)

type IndustryAccessToken struct {
	ID        uuid.UUID  `json:"id"`
	ReportID  uuid.UUID  `json:"report_id"`
	TokenHash string     `json:"-"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at"`
	CreatedAt time.Time  `json:"created_at"`

	Report *WeeklyReport `json:"report,omitempty"`
}
