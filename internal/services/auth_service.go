package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/platform/auth"
	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/platform/logger"
	"github.com/sli/backend/internal/repositories"
)

type AuthService interface {
	HandleOAuthCallback(ctx context.Context, code string) (*domain.User, string, string, error)
	// RequestMagicLink emails a one-click sign-in link to email, if the
	// domain is allowed. It never returns an error for "email not found" -
	// a first-time email gets a fresh account provisioned the same way a
	// first-time Google sign-in does (see VerifyMagicLink) - so the response
	// to the client is identical either way and doesn't leak which emails
	// already have accounts.
	RequestMagicLink(ctx context.Context, email string) error
	// VerifyMagicLink consumes a single-use magic-link token (see
	// RequestMagicLink) and returns the signed-in user plus a fresh JWT pair,
	// mirroring HandleOAuthCallback's return shape.
	VerifyMagicLink(ctx context.Context, rawToken string) (*domain.User, string, string, error)
	Logout(ctx context.Context, userID uuid.UUID, accessJTI, refreshJTI string, accessExp, refreshExp time.Time, actorName string) error
	GetCurrentUser(ctx context.Context, userID uuid.UUID) (*domain.User, error)
}

type authService struct {
	oauthService        auth.OAuthService
	jwtService          auth.JWTService
	userRepo            repositories.UserRepository
	revokedRepo         repositories.RevokedTokenRepository
	auditRepo           repositories.AuditRepository
	magicLinkRepo       repositories.MagicLinkRepository
	notificationService NotificationService
	allowedDomain       string
	magicLinkExpiryMin  int
}

func NewAuthService(
	oauthService auth.OAuthService,
	jwtService auth.JWTService,
	userRepo repositories.UserRepository,
	revokedRepo repositories.RevokedTokenRepository,
	auditRepo repositories.AuditRepository,
	magicLinkRepo repositories.MagicLinkRepository,
	notificationService NotificationService,
	allowedDomain string,
	magicLinkExpiryMin int,
) AuthService {
	return &authService{
		oauthService:        oauthService,
		jwtService:          jwtService,
		userRepo:            userRepo,
		revokedRepo:         revokedRepo,
		auditRepo:           auditRepo,
		magicLinkRepo:       magicLinkRepo,
		notificationService: notificationService,
		allowedDomain:       allowedDomain,
		magicLinkExpiryMin:  magicLinkExpiryMin,
	}
}

func (s *authService) HandleOAuthCallback(ctx context.Context, code string) (*domain.User, string, string, error) {
	googleUser, err := s.oauthService.ExchangeCode(ctx, code)
	if err != nil {
		return nil, "", "", err
	}

	user, err := s.userRepo.FindByGoogleSub(ctx, googleUser.Sub)
	if err != nil {
		if err == repositories.ErrUserNotFound {
			// No user with this Google sub yet. They may already exist via
			// magic-link signup with the same email (email is UNIQUE, so we
			// must link rather than insert a second row) - check before
			// creating a brand new user.
			existing, findErr := s.userRepo.FindByEmail(ctx, googleUser.Email)
			if findErr == nil {
				if err := s.userRepo.LinkGoogleSub(ctx, existing.ID, googleUser.Sub); err != nil {
					return nil, "", "", err
				}
				user, err = s.userRepo.FindByID(ctx, existing.ID)
				if err != nil {
					return nil, "", "", err
				}
			} else if findErr != repositories.ErrUserNotFound {
				return nil, "", "", findErr
			} else {
				sub := googleUser.Sub
				user, err = s.provisionUser(ctx, googleUser.Email, googleUser.Name, &sub)
				if err != nil {
					return nil, "", "", err
				}
			}
		} else {
			return nil, "", "", err
		}
	} else {
		// Update details if changed
		user.Name = googleUser.Name
		user.Email = googleUser.Email
		now := time.Now()
		user.LastLoginAt = &now
		if err := s.userRepo.UpdateUser(ctx, user); err != nil {
			return nil, "", "", err
		}
	}

	accessToken, refreshToken, err := s.jwtService.GenerateTokenPair(user)
	if err != nil {
		return nil, "", "", err
	}

	l := logger.WithContext(ctx)
	l.Info("User logged in successfully", "user_id", user.ID, "email", user.Email, "method", "google_oauth")

	return user, accessToken, refreshToken, nil
}

// provisionUser creates a brand-new user with the default Student role -
// Coordinators/faculty/admins are provisioned by an admin via SetRole after
// the fact - role_id is NOT NULL in the schema, so we must resolve it before
// creating the row. Shared by both first-time Google sign-in and first-time
// magic-link sign-in.
func (s *authService) provisionUser(ctx context.Context, email, name string, googleSub *string) (*domain.User, error) {
	defaultRole, err := s.userRepo.FindRoleByName(ctx, domain.RoleStudent)
	if err != nil {
		return nil, err
	}

	if name == "" {
		name = email
	}

	user := &domain.User{
		GoogleSub: googleSub,
		Email:     email,
		Name:      name,
		RoleID:    defaultRole.ID,
	}
	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	// Reload so the Role association is populated.
	return s.userRepo.FindByID(ctx, user.ID)
}

// RequestMagicLink validates the email's domain, generates a single-use
// token (same hashed-token pattern as TokenService for industry review
// links: only the SHA-256 hash is stored, the raw token only ever exists in
// memory long enough to email it), and enqueues the email.
func (s *authService) RequestMagicLink(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return errors.New(errors.CodeValidationFailed, "a valid email address is required")
	}
	domainPart := email[strings.LastIndex(email, "@")+1:]
	if domainPart != s.allowedDomain {
		return errors.New(errors.CodeInvalidDomain, "email domain is not allowed")
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return errors.NewWithErr(errors.CodeInternalServer, "failed to generate secure token", err)
	}
	rawToken := hex.EncodeToString(b)
	hash := sha256.Sum256([]byte(rawToken))
	hashedToken := hex.EncodeToString(hash[:])

	token := &domain.MagicLinkToken{
		Email:     email,
		TokenHash: hashedToken,
		ExpiresAt: time.Now().Add(time.Duration(s.magicLinkExpiryMin) * time.Minute),
	}
	if err := s.magicLinkRepo.Create(ctx, token); err != nil {
		return errors.NewWithErr(errors.CodeInternalServer, "failed to store magic link token", err)
	}

	if err := s.notificationService.NotifyMagicLink(ctx, email, rawToken, s.magicLinkExpiryMin); err != nil {
		return errors.NewWithErr(errors.CodeInternalServer, "failed to send magic link email", err)
	}

	return nil
}

func (s *authService) VerifyMagicLink(ctx context.Context, rawToken string) (*domain.User, string, string, error) {
	hash := sha256.Sum256([]byte(rawToken))
	hashedToken := hex.EncodeToString(hash[:])

	token, err := s.magicLinkRepo.FindByHash(ctx, hashedToken)
	if err != nil {
		if err == repositories.ErrMagicLinkNotFound {
			return nil, "", "", errors.New(errors.CodeTokenExpired, "invalid or expired sign-in link")
		}
		return nil, "", "", errors.NewWithErr(errors.CodeInternalServer, "database error checking magic link", err)
	}
	if time.Now().After(token.ExpiresAt) {
		return nil, "", "", errors.New(errors.CodeTokenExpired, "sign-in link has expired")
	}
	if token.UsedAt != nil {
		return nil, "", "", errors.New(errors.CodeTokenExpired, "sign-in link has already been used")
	}

	if err := s.magicLinkRepo.MarkUsed(ctx, token.ID); err != nil {
		return nil, "", "", errors.NewWithErr(errors.CodeInternalServer, "failed to consume magic link token", err)
	}

	user, err := s.userRepo.FindByEmail(ctx, token.Email)
	if err != nil {
		if err != repositories.ErrUserNotFound {
			return nil, "", "", errors.NewWithErr(errors.CodeInternalServer, "database error checking user", err)
		}
		user, err = s.provisionUser(ctx, token.Email, "", nil)
		if err != nil {
			return nil, "", "", errors.NewWithErr(errors.CodeInternalServer, "failed to provision user", err)
		}
	} else {
		now := time.Now()
		user.LastLoginAt = &now
		if err := s.userRepo.UpdateUser(ctx, user); err != nil {
			return nil, "", "", errors.NewWithErr(errors.CodeInternalServer, "failed to update user", err)
		}
	}

	accessToken, refreshToken, err := s.jwtService.GenerateTokenPair(user)
	if err != nil {
		return nil, "", "", errors.NewWithErr(errors.CodeInternalServer, "failed to generate session", err)
	}

	l := logger.WithContext(ctx)
	l.Info("User logged in successfully", "user_id", user.ID, "email", user.Email, "method", "magic_link")

	return user, accessToken, refreshToken, nil
}

func (s *authService) Logout(ctx context.Context, userID uuid.UUID, accessJTI, refreshJTI string, accessExp, refreshExp time.Time, actorName string) error {
	// Revoke Access Token
	if accessJTI != "" {
		if err := s.revokedRepo.Revoke(ctx, accessJTI, userID, accessExp); err != nil {
			slog.Error("Failed to revoke access token", "err", err)
		}
	}
	// Revoke Refresh Token
	if refreshJTI != "" {
		if err := s.revokedRepo.Revoke(ctx, refreshJTI, userID, refreshExp); err != nil {
			slog.Error("Failed to revoke refresh token", "err", err)
		}
	}

	l := logger.WithContext(ctx)
	l.Info("User logged out", "action", "logout", "user_id", userID, "actor_name", actorName)

	if s.auditRepo != nil {
		meta, _ := json.Marshal(map[string]interface{}{
			"access_jti":  accessJTI,
			"refresh_jti": refreshJTI,
		})
		_ = s.auditRepo.Create(ctx, &domain.AuditLog{
			ActorUserID:  userID,
			ActorName:    actorName,
			Action:       "logout",
			ResourceType: "users",
			ResourceID:   userID,
			MetadataJSON: string(meta),
			CreatedAt:    time.Now(),
		})
	}

	return nil
}

func (s *authService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.userRepo.FindByID(ctx, userID)
}
