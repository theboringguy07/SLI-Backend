package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimitRejectsRequestsAfterLimit(t *testing.T) {
	called := 0
	handler := RateLimit(1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusNoContent)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req1.RemoteAddr = "127.0.0.1:1234"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req2.RemoteAddr = "127.0.0.1:1234"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if called != 1 {
		t.Fatalf("expected downstream called once, got %d", called)
	}
	if rr1.Code != http.StatusNoContent {
		t.Fatalf("expected first request 204, got %d", rr1.Code)
	}
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("expected second request rate-limited with 400, got %d", rr2.Code)
	}
}
