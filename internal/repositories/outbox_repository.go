package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
)

type OutboxRepository interface {
	Enqueue(ctx context.Context, email *domain.EmailOutbox) error
	FetchPending(ctx context.Context, limit int) ([]domain.EmailOutbox, error)
	MarkSent(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, nextAttemptAt time.Time) error
}

// Table name: email_delivery_requests (see database/schema/schema.sql),
// primary key column: email_request_id.
type outboxRepository struct {
	db dbtx
}

func NewOutboxRepository(db dbtx) OutboxRepository {
	return &outboxRepository{db: db}
}

func (r *outboxRepository) Enqueue(ctx context.Context, email *domain.EmailOutbox) error {
	email.Status = domain.OutboxPending
	if email.NextAttemptAt.IsZero() {
		email.NextAttemptAt = time.Now()
	}

	return r.db.QueryRow(ctx, `
		INSERT INTO email_delivery_requests (recipient, subject, body_html, template_key, template_data, status, attempts, next_attempt_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING email_request_id, created_at, updated_at`,
		email.Recipient, email.Subject, email.Body, email.TemplateKey, email.TemplateData, email.Status,
		email.Attempts, email.NextAttemptAt,
	).Scan(&email.ID, &email.CreatedAt, &email.UpdatedAt)
}

func (r *outboxRepository) FetchPending(ctx context.Context, limit int) ([]domain.EmailOutbox, error) {
	rows, err := r.db.Query(ctx, `
		SELECT email_request_id, recipient, subject, body_html, template_key, template_data, status, attempts, next_attempt_at, created_at, updated_at
		FROM email_delivery_requests
		WHERE status = $1 AND next_attempt_at <= $2
		ORDER BY next_attempt_at ASC
		LIMIT $3`, domain.OutboxPending, time.Now(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	emails := []domain.EmailOutbox{}
	for rows.Next() {
		var e domain.EmailOutbox
		if err := rows.Scan(&e.ID, &e.Recipient, &e.Subject, &e.Body, &e.TemplateKey, &e.TemplateData, &e.Status, &e.Attempts, &e.NextAttemptAt, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

func (r *outboxRepository) MarkSent(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE email_delivery_requests SET status = $1 WHERE email_request_id = $2`, domain.OutboxSent, id)
	return err
}

func (r *outboxRepository) MarkFailed(ctx context.Context, id uuid.UUID, nextAttemptAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE email_delivery_requests SET attempts = attempts + 1, next_attempt_at = $1
		WHERE email_request_id = $2`, nextAttemptAt, id)
	return err
}
