package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/sli/backend/internal/platform/logger"
)

// RequestID generates a unique request ID and attaches it to the request context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := generateRequestID()
		ctx := context.WithValue(r.Context(), logger.RequestIDKey, reqID)
		
		// Also set it in the response header for the client
		w.Header().Set("X-Request-ID", reqID)
		
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func generateRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}
