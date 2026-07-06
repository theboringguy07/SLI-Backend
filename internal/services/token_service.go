package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/repositories"
)

type TokenService interface {
	GenerateToken(ctx context.Context, reportID uuid.UUID) (string, error)
	ValidateToken(ctx context.Context, rawToken string) (*domain.IndustryAccessToken, error)
	MarkTokenUsed(ctx context.Context, id uuid.UUID) error
	CleanupExpiredTokens(ctx context.Context) error
}

type tokenService struct {
	tokenRepo repositories.TokenRepository
}

func NewTokenService(tokenRepo repositories.TokenRepository) TokenService {
	return &tokenService{
		tokenRepo: tokenRepo,
	}
}

func (s *tokenService) GenerateToken(ctx context.Context, reportID uuid.UUID) (string, error) {
	// Generate random 32-byte token
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.NewWithErr(errors.CodeInternalServer, "failed to generate secure token", err)
	}
	rawToken := hex.EncodeToString(b)

	// Hash the token
	hash := sha256.Sum256([]byte(rawToken))
	hashedToken := hex.EncodeToString(hash[:])

	token := &domain.IndustryAccessToken{
		ReportID:  reportID,
		TokenHash: hashedToken,
		ExpiresAt: time.Now().Add(24 * time.Hour), // 24-hour expiry as per requirements
	}

	if err := s.tokenRepo.Create(ctx, token); err != nil {
		return "", errors.NewWithErr(errors.CodeInternalServer, "failed to store token", err)
	}

	// Return the raw token so it can be emailed
	return rawToken, nil
}

func (s *tokenService) ValidateToken(ctx context.Context, rawToken string) (*domain.IndustryAccessToken, error) {
	hash := sha256.Sum256([]byte(rawToken))
	hashedToken := hex.EncodeToString(hash[:])

	token, err := s.tokenRepo.FindByHash(ctx, hashedToken)
	if err != nil {
		if err == repositories.ErrTokenNotFound {
			return nil, errors.New(errors.CodeTokenExpired, "invalid or expired token")
		}
		return nil, errors.NewWithErr(errors.CodeInternalServer, "database error checking token", err)
	}

	if time.Now().After(token.ExpiresAt) {
		return nil, errors.New(errors.CodeTokenExpired, "token has expired")
	}

	if token.UsedAt != nil {
		return nil, errors.New(errors.CodeTokenExpired, "token has already been used")
	}

	return token, nil
}

func (s *tokenService) MarkTokenUsed(ctx context.Context, id uuid.UUID) error {
	return s.tokenRepo.MarkUsed(ctx, id)
}

func (s *tokenService) CleanupExpiredTokens(ctx context.Context) error {
	return s.tokenRepo.DeleteExpired(ctx)
}
