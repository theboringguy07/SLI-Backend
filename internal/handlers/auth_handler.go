package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/http/middleware"
	"github.com/sli/backend/internal/http/response"
	"github.com/sli/backend/internal/platform/auth"
	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/services"
)

type AuthHandler struct {
	oauthService auth.OAuthService
	jwtService   auth.JWTService
	authService  services.AuthService
	frontendURL  string
	csrfSecret   []byte
}

func NewAuthHandler(oauthService auth.OAuthService, jwtService auth.JWTService, authService services.AuthService, frontendURL string, csrfSecret string) *AuthHandler {
	return &AuthHandler{
		oauthService: oauthService,
		jwtService:   jwtService,
		authService:  authService,
		frontendURL:  frontendURL,
		csrfSecret:   []byte(csrfSecret),
	}
}

func (h *AuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := secureState()
	if err != nil {
		appErr := errors.NewWithErr(errors.CodeInternalServer, "Failed to start OAuth login", err)
		response.Error(w, r, appErr)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/api/auth/google/callback",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300,
	})
	url := h.oauthService.GetLoginURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func secureState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || state == "" || stateCookie.Value != state {
		appErr := errors.New(errors.CodeUnauthorized, "Invalid OAuth state")
		response.Error(w, r, appErr)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/api/auth/google/callback",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		appErr := errors.New(errors.CodeBadRequest, "Missing auth code")
		response.Error(w, r, appErr)
		return
	}

	user, accessToken, refreshToken, err := h.authService.HandleOAuthCallback(r.Context(), code)
	if err != nil {
		if err == auth.ErrInvalidDomain {
			appErr := errors.New(errors.CodeInvalidDomain, "Email domain is not allowed")
			response.Error(w, r, appErr)
			return
		}
		appErr := errors.NewWithErr(errors.CodeInternalServer, "Authentication failed", err)
		response.Error(w, r, appErr)
		return
	}

	h.startSession(w, user.ID, accessToken, refreshToken)

	// Redirect to frontend dashboard
	http.Redirect(w, r, h.frontendURL+"/dashboard", http.StatusTemporaryRedirect)
}

// startSession sets the access/refresh cookies and issues a CSRF token for a
// newly authenticated user. Shared by GoogleCallback and VerifyMagicLink -
// both end in an identical "issue a session" step regardless of which login
// method produced the user/tokens.
func (h *AuthHandler) startSession(w http.ResponseWriter, userID uuid.UUID, accessToken, refreshToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   3600, // 1 hour
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   7 * 24 * 3600, // 7 days
	})

	middleware.IssueCSRFTokenForUser(w, userID, h.csrfSecret)
}

// RequestMagicLink is POST /api/auth/magic-link/request {"email": "..."}. It
// always returns 200 regardless of whether the email already has an account
// (a brand new one is provisioned on first successful verify, same as first
// Google sign-in) - this avoids leaking which emails are registered.
func (h *AuthHandler) RequestMagicLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid JSON payload"))
		return
	}

	if err := h.authService.RequestMagicLink(r.Context(), req.Email); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"message": "If that email is allowed, a sign-in link has been sent"})
}

// VerifyMagicLink is GET /api/auth/magic-link/verify?token=... - the link
// clicked from the emailed message. Mirrors GoogleCallback: consume the
// token, start a session, redirect to the frontend dashboard.
func (h *AuthHandler) VerifyMagicLink(w http.ResponseWriter, r *http.Request) {
	rawToken := r.URL.Query().Get("token")
	if rawToken == "" {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Missing token"))
		return
	}

	user, accessToken, refreshToken, err := h.authService.VerifyMagicLink(r.Context(), rawToken)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	h.startSession(w, user.ID, accessToken, refreshToken)

	http.Redirect(w, r, h.frontendURL+"/dashboard", http.StatusTemporaryRedirect)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)
	if !ok {
		appErr := errors.New(errors.CodeUnauthorized, "Not authenticated")
		response.Error(w, r, appErr)
		return
	}

	refreshJTI := ""
	refreshExp := time.Now()
	if refreshCookie, err := r.Cookie("refresh_token"); err == nil {
		if refreshClaims, err := h.jwtService.ValidateToken(refreshCookie.Value); err == nil && refreshClaims.UserID == claims.UserID {
			refreshJTI = refreshClaims.ID
			refreshExp = refreshClaims.ExpiresAt.Time
		}
	}

	err := h.authService.Logout(r.Context(), claims.UserID, claims.ID, refreshJTI, claims.ExpiresAt.Time, refreshExp, claims.Email)
	if err != nil {
		appErr := errors.NewWithErr(errors.CodeInternalServer, "Failed to logout", err)
		response.Error(w, r, appErr)
		return
	}

	// Clear cookies
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	middleware.ClearCSRFToken(w)

	response.JSON(w, http.StatusOK, map[string]string{"message": "Successfully logged out"})
}

func (h *AuthHandler) GetCSRF(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)
	if !ok {
		response.Error(w, r, errors.New(errors.CodeUnauthorized, "Not authenticated"))
		return
	}

	token := middleware.IssueCSRFTokenForUser(w, claims.UserID, h.csrfSecret)
	response.JSON(w, http.StatusOK, map[string]string{
		"csrf_token": token,
		"header":     "X-CSRF-Token",
	})
}

func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)
	if !ok {
		appErr := errors.New(errors.CodeUnauthorized, "Not authenticated")
		response.Error(w, r, appErr)
		return
	}

	user, err := h.authService.GetCurrentUser(r.Context(), claims.UserID)
	if err != nil {
		appErr := errors.NewWithErr(errors.CodeNotFound, "User not found", err)
		response.Error(w, r, appErr)
		return
	}

	response.JSON(w, http.StatusOK, user)
}
