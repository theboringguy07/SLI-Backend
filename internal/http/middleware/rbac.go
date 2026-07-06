package middleware

import (
	"net/http"

	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/http/response"
	"github.com/sli/backend/internal/platform/auth"
	"github.com/sli/backend/internal/platform/errors"
)

// RBAC middleware ensures the user has at least one of the allowed roles.
func RBAC(allowedRoles ...domain.RoleName) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(UserClaimsKey).(*auth.JWTClaims)
			if !ok {
				appErr := errors.New(errors.CodeUnauthorized, "User claims not found in context")
				response.Error(w, r, appErr)
				return
			}

			hasRole := false
			for _, allowedRole := range allowedRoles {
				if claims.Role == allowedRole {
					hasRole = true
					break
				}
			}

			if !hasRole {
				appErr := errors.New(errors.CodeForbidden, "You do not have permission to access this resource")
				response.Error(w, r, appErr)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
