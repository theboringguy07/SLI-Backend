package config

import (
	"crypto/hmac"
	"crypto/sha256"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const insecureJWTSecretDefault = "super_secret_key_change_me_in_production"

type Config struct {
	AppEnv                string
	Port                  string
	DBDSN                 string
	JWTSecret             string
	JWTExpiryHours        int
	JWTRefreshExpiryDays  int
	OAuthClientID         string
	OAuthClientSecret     string
	OAuthRedirectURL      string
	AllowedDomain         string
	FrontendURL           string
	// APIBaseURL is this server's own public base URL (no trailing slash),
	// used to build the clickable link inside a magic-link email - the link
	// must hit this backend's /api/auth/magic-link/verify endpoint directly
	// (same idea as OAuthRedirectURL for Google's callback), not the
	// frontend.
	APIBaseURL string
	CORSOrigins           []string
	PDFStoragePath        string
	ReportEditWindowHours int
	RateLimitAuth         int
	RateLimitPublic       int
	RateLimitDefault      int
	SMTPHost              string
	SMTPPort              int
	SMTPUsername          string
	SMTPPassword          string
	SMTPFromAddress       string
	SMTPFromName          string
	MagicLinkExpiryMin    int
}

// IsProduction reports whether APP_ENV is set to "production".
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

// CORSAllowedOrigins returns the origins permitted to make credentialed
// cross-origin requests.
func (c *Config) CORSAllowedOrigins() []string {
	return c.CORSOrigins
}

// CSRFSecret derives a key for CSRF token signing that is distinct from the
// JWT signing key, for key separation: a compromise of one signing use
// (e.g. a bug that leaks a MAC) shouldn't automatically compromise the other.
func (c *Config) CSRFSecret() []byte {
	mac := hmac.New(sha256.New, []byte(c.JWTSecret))
	mac.Write([]byte("csrf-token-v1"))
	return mac.Sum(nil)
}

func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	frontendURL := getEnv("FRONTEND_URL", "http://localhost:3000")

	cfg := &Config{
		AppEnv:                getEnv("APP_ENV", "development"),
		Port:                  getEnv("PORT", "8080"),
		DBDSN:                 getEnv("DB_DSN", "postgres://user:password@localhost:5432/sli_db?sslmode=disable"),
		JWTSecret:             getEnv("JWT_SECRET", insecureJWTSecretDefault),
		JWTExpiryHours:        getEnvAsInt("JWT_EXPIRY_HOURS", 1),
		JWTRefreshExpiryDays:  getEnvAsInt("JWT_REFRESH_EXPIRY_DAYS", 7),
		OAuthClientID:         getEnv("OAUTH_CLIENT_ID", ""),
		OAuthClientSecret:     getEnv("OAUTH_CLIENT_SECRET", ""),
		OAuthRedirectURL:      getEnv("OAUTH_REDIRECT_URL", ""),
		AllowedDomain:         getEnv("ALLOWED_DOMAIN", "somaiya.edu"),
		FrontendURL:           frontendURL,
		APIBaseURL:            getEnv("API_BASE_URL", "http://localhost:8080"),
		CORSOrigins:           splitAndTrim(getEnv("CORS_ALLOWED_ORIGINS", frontendURL)),
		PDFStoragePath:        getEnv("PDF_STORAGE_PATH", "./data/marksheets"),
		ReportEditWindowHours: getEnvAsInt("REPORT_EDIT_WINDOW_HOURS", 24),
		RateLimitAuth:         getEnvAsInt("RATE_LIMIT_AUTH", 100),
		RateLimitPublic:       getEnvAsInt("RATE_LIMIT_PUBLIC", 20),
		RateLimitDefault:      getEnvAsInt("RATE_LIMIT_DEFAULT", 100),
		SMTPHost:              getEnv("SMTP_HOST", ""),
		SMTPPort:              getEnvAsInt("SMTP_PORT", 587),
		SMTPUsername:          getEnv("SMTP_USERNAME", ""),
		SMTPPassword:          getEnv("SMTP_PASSWORD", ""),
		SMTPFromAddress:       getEnv("SMTP_FROM_ADDRESS", "no-reply@"+getEnv("ALLOWED_DOMAIN", "somaiya.edu")),
		SMTPFromName:          getEnv("SMTP_FROM_NAME", "SLI Internship Portal"),
		MagicLinkExpiryMin:    getEnvAsInt("MAGIC_LINK_EXPIRY_MINUTES", 15),
	}

	if cfg.IsProduction() {
		if problems := cfg.validateProduction(); len(problems) > 0 {
			log.Fatalf("refusing to start with APP_ENV=production: %s", strings.Join(problems, "; "))
		}
	}

	return cfg
}

// validateProduction returns a list of configuration problems that must be
// fixed before this app is safe to run with real user data. Insecure
// defaults are fine for local development but must never reach production.
func (c *Config) validateProduction() []string {
	var problems []string

	if len(c.JWTSecret) < 32 || c.JWTSecret == insecureJWTSecretDefault {
		problems = append(problems, "JWT_SECRET must be set to a random value of at least 32 characters")
	}
	if strings.Contains(c.DBDSN, "user:password@") {
		problems = append(problems, "DB_DSN is still set to its placeholder value")
	}
	if c.OAuthClientID == "" || c.OAuthClientSecret == "" {
		problems = append(problems, "OAUTH_CLIENT_ID and OAUTH_CLIENT_SECRET must be set")
	}
	if !strings.HasPrefix(c.OAuthRedirectURL, "https://") {
		problems = append(problems, "OAUTH_REDIRECT_URL must be an https:// URL")
	}
	if !strings.HasPrefix(c.FrontendURL, "https://") {
		problems = append(problems, "FRONTEND_URL must be an https:// URL")
	}
	if !strings.HasPrefix(c.APIBaseURL, "https://") {
		problems = append(problems, "API_BASE_URL must be an https:// URL")
	}
	if len(c.CORSOrigins) == 0 {
		problems = append(problems, "CORS_ALLOWED_ORIGINS (or FRONTEND_URL) must be set")
	}
	if c.SMTPHost == "" || c.SMTPUsername == "" || c.SMTPPassword == "" {
		problems = append(problems, "SMTP_HOST, SMTP_USERNAME and SMTP_PASSWORD must be set (required to send magic-link emails)")
	}

	return problems
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsInt(name string, defaultVal int) int {
	valueStr := getEnv(name, "")
	if valueStr == "" {
		return defaultVal
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("Warning: Environment variable %s is not a valid integer. Using default value %d", name, defaultVal)
		return defaultVal
	}
	return value
}

func splitAndTrim(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
