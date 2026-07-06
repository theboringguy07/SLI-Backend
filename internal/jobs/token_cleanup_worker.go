package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/sli/backend/internal/repositories"
	"github.com/sli/backend/internal/services"
)

// TokenCleanupWorker periodically deletes expired single-use tokens:
// industry-mentor review tokens (via TokenService) and magic-link sign-in
// tokens (via MagicLinkRepository directly - there's no service layer for
// magic link cleanup since AuthService only needs Create/FindByHash/MarkUsed).
type TokenCleanupWorker struct {
	tokenService  services.TokenService
	magicLinkRepo repositories.MagicLinkRepository
}

func NewTokenCleanupWorker(tokenService services.TokenService, magicLinkRepo repositories.MagicLinkRepository) *TokenCleanupWorker {
	return &TokenCleanupWorker{
		tokenService:  tokenService,
		magicLinkRepo: magicLinkRepo,
	}
}

func (w *TokenCleanupWorker) Start(ctx context.Context) {
	slog.Info("Starting Token Cleanup Worker")
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping Token Cleanup Worker")
			return
		case <-ticker.C:
			if err := w.tokenService.CleanupExpiredTokens(ctx); err != nil {
				slog.Error("Failed to cleanup expired industry access tokens", "err", err)
			}
			if err := w.magicLinkRepo.DeleteExpired(ctx); err != nil {
				slog.Error("Failed to cleanup expired magic link tokens", "err", err)
			}
		}
	}
}
