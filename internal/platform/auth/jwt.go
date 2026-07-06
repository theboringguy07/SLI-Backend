package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/sli/backend/internal/config"
	"github.com/sli/backend/internal/domain"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
)

type JWTClaims struct {
	UserID uuid.UUID       `json:"user_id"`
	Email  string          `json:"email"`
	Role   domain.RoleName `json:"role"`
	jwt.RegisteredClaims
}

type JWTService interface {
	GenerateTokenPair(user *domain.User) (accessToken, refreshToken string, err error)
	ValidateToken(tokenStr string) (*JWTClaims, error)
}

type jwtService struct {
	secretKey     []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewJWTService(cfg *config.Config) JWTService {
	return &jwtService{
		secretKey:     []byte(cfg.JWTSecret),
		accessExpiry:  time.Duration(cfg.JWTExpiryHours) * time.Hour,
		refreshExpiry: time.Duration(cfg.JWTRefreshExpiryDays) * 24 * time.Hour,
	}
}

func (s *jwtService) GenerateTokenPair(user *domain.User) (string, string, error) {
	role := user.Role.Name

	now := time.Now()

	// Access Token
	accessJTI := uuid.New().String()
	accessClaims := JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        accessJTI,
			Subject:   user.ID.String(),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessString, err := accessToken.SignedString(s.secretKey)
	if err != nil {
		return "", "", err
	}

	// Refresh Token
	refreshJTI := uuid.New().String()
	refreshClaims := JWTClaims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.refreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        refreshJTI,
			Subject:   user.ID.String(),
		},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshString, err := refreshToken.SignedString(s.secretKey)
	if err != nil {
		return "", "", err
	}

	return accessString, refreshString, nil
}

func (s *jwtService) ValidateToken(tokenStr string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secretKey, nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
