package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/sli/backend/internal/http/response"
	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/platform/logger"
)

// Recoverer recovers from panics, logs the stack trace, and returns a 500 error.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				l := logger.WithContext(r.Context())
				l.Error("panic recovered", "error", err, "stack", string(debug.Stack()))

				// Return a generic 500 JSON response
				appErr := errors.New(errors.CodeInternalServer, "An unexpected error occurred")
				response.Error(w, r, appErr)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
