package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/sli/backend/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	ErrInvalidDomain = errors.New("email domain not allowed")
)

type GoogleUser struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	HostedDomain  string `json:"hd"`
}

type OAuthService interface {
	GetLoginURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*GoogleUser, error)
}

type oauthService struct {
	config *oauth2.Config
	cfg    *config.Config
}

func NewOAuthService(cfg *config.Config) OAuthService {
	return &oauthService{
		cfg: cfg,
		config: &oauth2.Config{
			ClientID:     cfg.OAuthClientID,
			ClientSecret: cfg.OAuthClientSecret,
			RedirectURL:  cfg.OAuthRedirectURL,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
	}
}

func (s *oauthService) GetLoginURL(state string) string {
	return s.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (s *oauthService) ExchangeCode(ctx context.Context, code string) (*GoogleUser, error) {
	token, err := s.config.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}

	client := s.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to get user info from Google")
	}

	var user GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	if !user.EmailVerified {
		return nil, errors.New("google email is not verified")
	}

	// For Google Workspace domains, 'hd' contains the domain.
	// We can also extract it manually from the email if 'hd' is missing but needed.
	domain := user.HostedDomain
	if domain == "" {
		// fallback to extract from email
		// e.g. foo@somaiya.edu -> somaiya.edu
		for i := len(user.Email) - 1; i >= 0; i-- {
			if user.Email[i] == '@' {
				domain = user.Email[i+1:]
				break
			}
		}
	}

	if domain != s.cfg.AllowedDomain {
		return nil, ErrInvalidDomain
	}

	user.HostedDomain = domain
	return &user, nil
}
