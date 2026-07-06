package response

import (
	"encoding/json"
	"net/http"

	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/platform/logger"
)

// JSON sends a standard JSON response.
func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			// We can't do much here if encoding fails, maybe log it.
			http.Error(w, `{"error":{"code":"INTERNAL_SERVER_ERROR","message":"Failed to encode response"}}`, http.StatusInternalServerError)
		}
	}
}

// Error sends a standard JSON error response. The client only ever sees the
// AppError's public Code/Message (deliberately - internal error text like DB
// or OAuth failures shouldn't leak), so anything at 500-level is logged
// server-side here with the full wrapped error chain via err.Error() -
// otherwise a 500 is completely unactionable from the server logs alone.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	requestID := ""
	if reqID, ok := r.Context().Value(logger.RequestIDKey).(string); ok {
		requestID = reqID
	}

	errResp, status := errors.ToErrorResponse(err, requestID)

	if status >= 500 {
		logger.WithContext(r.Context()).Error("request failed",
			"request_id", requestID,
			"path", r.URL.Path,
			"status", status,
			"error", err.Error(),
		)
	}

	JSON(w, status, errResp)
}
