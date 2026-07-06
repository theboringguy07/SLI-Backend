package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/http/response"
	"github.com/sli/backend/internal/platform/auth"
	"github.com/sli/backend/internal/platform/errors"
)

const csrfCookieName = "csrf_token"
const csrfHeaderName = "X-CSRF-Token"

func CSRF(secret string) func(http.Handler) http.Handler {
	secretBytes := []byte(secret)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, _ := r.Context().Value(UserClaimsKey).(*auth.JWTClaims)
			if claims == nil {
				next.ServeHTTP(w, r)
				return
			}

			if isSafeMethod(r.Method) {
				IssueCSRFTokenForUser(w, claims.UserID, secretBytes)
				next.ServeHTTP(w, r)
				return
			}

			if !usesCookieAuth(r) {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie(csrfCookieName)
			if err != nil || cookie.Value == "" || r.Header.Get(csrfHeaderName) == "" {
				response.Error(w, r, errors.New(errors.CodeForbidden, "Missing CSRF token"))
				return
			}

			if cookie.Value != r.Header.Get(csrfHeaderName) || !ValidCSRFToken(cookie.Value, claims.UserID, secretBytes) {
				response.Error(w, r, errors.New(errors.CodeForbidden, "Invalid CSRF token"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func IssueCSRFTokenForUser(w http.ResponseWriter, userID uuid.UUID, secret []byte) string {
	token := newCSRFToken(userID, secret)
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   7 * 24 * 3600,
	})
	return token
}

func ClearCSRFToken(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func ValidCSRFToken(token string, userID uuid.UUID, secret []byte) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	expected := csrfSignature(parts[0], userID, secret)
	return hmac.Equal([]byte(parts[1]), []byte(expected))
}

func newCSRFToken(userID uuid.UUID, secret []byte) string {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		randomBytes = []byte(uuid.NewString())
	}
	randomPart := base64.RawURLEncoding.EncodeToString(randomBytes)
	return randomPart + "." + csrfSignature(randomPart, userID, secret)
}

func csrfSignature(randomPart string, userID uuid.UUID, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(userID.String()))
	mac.Write([]byte(":"))
	mac.Write([]byte(randomPart))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func usesCookieAuth(r *http.Request) bool {
	if hasBearerToken(r) {
		return false
	}
	_, err := r.Cookie("access_token")
	return err == nil
}

func hasBearerToken(r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	parts := strings.Split(authHeader, " ")
	return len(parts) == 2 && strings.EqualFold(parts[0], "bearer") && parts[1] != ""
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions || method == http.MethodTrace
}
