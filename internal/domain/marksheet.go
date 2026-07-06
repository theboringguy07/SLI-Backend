package domain

import (
	"time"

	"github.com/google/uuid"
)

type Marksheet struct {
	ID                uuid.UUID `json:"id"`
	EvaluationScoreID uuid.UUID `json:"evaluation_score_id"`
	FileKey           string    `json:"file_key"`
	GeneratedAt       time.Time `json:"generated_at"`
	GeneratedBy       uuid.UUID `json:"generated_by"`

	EvaluationScore *EvaluationScore `json:"evaluation_score,omitempty"`
}
