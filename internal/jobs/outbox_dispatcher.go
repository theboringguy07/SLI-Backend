package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/sli/backend/internal/platform/mailer"
	"github.com/sli/backend/internal/repositories"
)

// OutboxDispatcher periodically drains email_delivery_requests (populated by
// NotificationService) and sends each one via SMTP (internal/platform/mailer).
// Failed sends are retried with simple linear backoff up to maxAttempts,
// after which they're left in the table (still "pending" but far in the
// future) for manual inspection rather than silently dropped.
type OutboxDispatcher struct {
	outboxRepo  repositories.OutboxRepository
	mailer      mailer.Mailer
	pollEvery   time.Duration
	batchSize   int
	maxAttempts int
}

func NewOutboxDispatcher(outboxRepo repositories.OutboxRepository, m mailer.Mailer) *OutboxDispatcher {
	return &OutboxDispatcher{
		outboxRepo:  outboxRepo,
		mailer:      m,
		pollEvery:   15 * time.Second,
		batchSize:   20,
		maxAttempts: 5,
	}
}

func (d *OutboxDispatcher) Start(ctx context.Context) {
	slog.Info("Starting Outbox Dispatcher")
	ticker := time.NewTicker(d.pollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping Outbox Dispatcher")
			return
		case <-ticker.C:
			d.dispatchPending(ctx)
		}
	}
}

func (d *OutboxDispatcher) dispatchPending(ctx context.Context) {
	emails, err := d.outboxRepo.FetchPending(ctx, d.batchSize)
	if err != nil {
		slog.Error("Failed to fetch pending outbox emails", "err", err)
		return
	}

	for _, email := range emails {
		err := d.mailer.Send([]string{email.Recipient}, email.Subject, email.Body)
		if err != nil {
			nextAttempt := time.Now().Add(time.Duration(email.Attempts+1) * time.Minute)
			if email.Attempts+1 >= d.maxAttempts {
				// Push far into the future rather than looping forever;
				// still visible via FetchPending for manual retry/inspection.
				nextAttempt = time.Now().Add(24 * time.Hour)
				slog.Error("Email delivery permanently failed after max attempts", "id", email.ID, "recipient", email.Recipient, "attempts", email.Attempts+1, "err", err)
			} else {
				slog.Warn("Email delivery failed, will retry", "id", email.ID, "recipient", email.Recipient, "attempt", email.Attempts+1, "err", err)
			}
			if markErr := d.outboxRepo.MarkFailed(ctx, email.ID, nextAttempt); markErr != nil {
				slog.Error("Failed to record outbox failure", "id", email.ID, "err", markErr)
			}
			continue
		}

		if err := d.outboxRepo.MarkSent(ctx, email.ID); err != nil {
			slog.Error("Failed to mark outbox email as sent", "id", email.ID, "err", err)
		}
	}
}
