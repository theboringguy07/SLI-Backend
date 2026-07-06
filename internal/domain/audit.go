package domain

import (
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID           uuid.UUID `json:"id"`
	ActorUserID  uuid.UUID `json:"actor_user_id"`
	ActorName    string    `json:"actor_name"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   uuid.UUID `json:"resource_id"`
	MetadataJSON string    `json:"metadata_json"`
	CreatedAt    time.Time `json:"created_at"`
}
