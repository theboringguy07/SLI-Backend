package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/platform/auth"
)

func TestRBACAllowsMatchingRole(t *testing.T) {
	called := false
	handler := RBAC(domain.RoleFaculty)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserClaimsKey, &auth.JWTClaims{
		Role: domain.RoleFaculty,
	}))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected handler to be called")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
}

func TestRBACRejectsWrongRole(t *testing.T) {
	handler := RBAC(domain.RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserClaimsKey, &auth.JWTClaims{
		Role: domain.RoleStudent,
	}))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestCSRFRequiresHeaderForUnsafeCookieAuth(t *testing.T) {
	userID := uuid.New()
	handler := CSRF("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called without CSRF header")
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/student/reports/1", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "cookie-token"})
	req = req.WithContext(context.WithValue(req.Context(), UserClaimsKey, &auth.JWTClaims{UserID: userID}))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestCSRFAllowsValidDoubleSubmitToken(t *testing.T) {
	userID := uuid.New()
	secret := []byte("secret")
	token := IssueCSRFTokenForUser(httptest.NewRecorder(), userID, secret)
	called := false
	handler := CSRF(string(secret))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/student/reports/1", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "cookie-token"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	req.Header.Set(csrfHeaderName, token)
	req = req.WithContext(context.WithValue(req.Context(), UserClaimsKey, &auth.JWTClaims{UserID: userID}))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected handler to be called")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
}

func TestCSRFSkipsBearerRequests(t *testing.T) {
	called := false
	handler := CSRF("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/student/reports/1", nil)
	req.Header.Set("Authorization", "Bearer token")
	req = req.WithContext(context.WithValue(req.Context(), UserClaimsKey, &auth.JWTClaims{UserID: uuid.New()}))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected bearer request to bypass CSRF")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
}

func TestNoCacheHeaders(t *testing.T) {
	handler := NoCache(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))

	if got := rr.Header().Get("Cache-Control"); got == "" {
		t.Fatal("expected Cache-Control header")
	}
	if got := rr.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("expected Pragma no-cache, got %q", got)
	}
}
