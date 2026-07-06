package auth

import (
	"testing"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/config"
	"github.com/sli/backend/internal/domain"
)

func TestJWTGenerateAndValidateTokenPair(t *testing.T) {
	userID := uuid.New()
	svc := NewJWTService(&config.Config{
		JWTSecret:            "test-secret",
		JWTExpiryHours:       1,
		JWTRefreshExpiryDays: 7,
	})

	user := &domain.User{
		ID:    userID,
		Email: "student@somaiya.edu",
		Role:  domain.Role{Name: domain.RoleStudent},
	}

	access, refresh, err := svc.GenerateTokenPair(user)
	if err != nil {
		t.Fatalf("GenerateTokenPair returned error: %v", err)
	}
	if access == "" || refresh == "" {
		t.Fatal("expected non-empty access and refresh tokens")
	}

	claims, err := svc.ValidateToken(access)
	if err != nil {
		t.Fatalf("ValidateToken returned error: %v", err)
	}
	if claims.UserID != userID || claims.Email != user.Email {
		t.Fatalf("claims mismatch: got user=%s email=%s", claims.UserID, claims.Email)
	}
	if claims.Role != domain.RoleStudent {
		t.Fatalf("role mismatch: %#v", claims.Role)
	}
	if claims.ID == "" {
		t.Fatal("expected token jti to be set")
	}
}

func TestJWTRejectsInvalidToken(t *testing.T) {
	svc := NewJWTService(&config.Config{JWTSecret: "test-secret", JWTExpiryHours: 1, JWTRefreshExpiryDays: 7})
	if _, err := svc.ValidateToken("not-a-jwt"); err == nil {
		t.Fatal("expected invalid token error")
	}
}
