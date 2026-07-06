package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/sli/backend/internal/http/response"
	"github.com/sli/backend/internal/platform/auth"
	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/platform/logger"
	"github.com/sli/backend/internal/repositories"
)

type contextKey string

const (
	UserClaimsKey contextKey = "user_claims"
)

// Auth middleware validates the JWT and checks if it has been revoked.
func Auth(jwtService auth.JWTService, revokedRepo repositories.RevokedTokenRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractToken(r)
			if tokenStr == "" {
				appErr := errors.New(errors.CodeUnauthorized, "Missing authentication token")
				response.Error(w, r, appErr)
				return
			}

			claims, err := jwtService.ValidateToken(tokenStr)
			if err != nil {
				appErr := errors.New(errors.CodeUnauthorized, "Invalid or expired token")
				response.Error(w, r, appErr)
				return
			}

			// Check if token was revoked (logout)
			if revokedRepo.IsRevoked(r.Context(), claims.ID) {
				appErr := errors.New(errors.CodeUnauthorized, "Token has been revoked")
				response.Error(w, r, appErr)
				return
			}

			// Add claims to context
			ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
			
			// Also add actor_id and actor_name to logger context
			ctx = context.WithValue(ctx, logger.ActorIDKey, claims.UserID.String())
			ctx = context.WithValue(ctx, logger.ActorNameKey, claims.Email) // Email or name

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractToken(r *http.Request) string {
	// Check header first
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	// Fallback to cookie
	cookie, err := r.Cookie("access_token")
	if err == nil {
		return cookie.Value
	}

	return ""
}
