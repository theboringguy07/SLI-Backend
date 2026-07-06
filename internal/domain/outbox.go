package domain

import (
	"time"

	"github.com/google/uuid"
)

// EmailOutbox maps to the "email_delivery_requests" table (primary key
// column email_request_id) - see database/schema/schema.sql. Rows are
// created by NotificationService and drained by the outbox dispatcher job
// (internal/jobs/outbox_dispatcher.go), which sends them via SMTP.
type EmailOutbox struct {
	ID            uuid.UUID    `json:"id"`
	Recipient     string       `json:"recipient"`
	Subject       string       `json:"subject"`
	Body          string       `json:"body"`
	TemplateKey   string       `json:"template_key"`
	TemplateData  string       `json:"template_data"`
	Status        OutboxStatus `json:"status"`
	Attempts      int          `json:"attempts"`
	NextAttemptAt time.Time    `json:"next_attempt_at"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}
