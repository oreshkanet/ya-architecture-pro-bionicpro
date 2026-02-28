package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port string

	KeycloakURL       string
	KeycloakPublicURL string
	KeycloakRealm     string
	KeycloakClientID  string
	KeycloakSecret    string

	FrontendURL     string
	ReportsAPIURL   string
	AuthCallbackURL string

	SessionCookieName string
	SessionMaxAge     int
	SecureCookie      bool

	EncryptionKey string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:              getEnv("PORT", "8001"),
		KeycloakURL:       getEnv("KEYCLOAK_URL", "http://localhost:8080"),
		KeycloakPublicURL: getEnv("KEYCLOAK_PUBLIC_URL", getEnv("KEYCLOAK_URL", "http://localhost:8080")),
		KeycloakRealm:     getEnv("KEYCLOAK_REALM", "reports-realm"),
		KeycloakClientID:  getEnv("KEYCLOAK_CLIENT_ID", "bionicpro-auth"),
		KeycloakSecret:    getEnv("KEYCLOAK_CLIENT_SECRET", "auth-secret-change-in-prod"),
		FrontendURL:       getEnv("FRONTEND_URL", "http://localhost:3000"),
		ReportsAPIURL:     getEnv("REPORTS_API_URL", "http://localhost:8000"),
		AuthCallbackURL:   getEnv("AUTH_CALLBACK_URL", "http://localhost:8001/auth/callback"),
		SessionCookieName: getEnv("SESSION_COOKIE_NAME", "bionicpro_session"),
		SessionMaxAge:     getEnvInt("SESSION_MAX_AGE", 1800),
		SecureCookie:      getEnv("SECURE_COOKIE", "false") == "true",
		EncryptionKey:     getEnv("ENCRYPTION_KEY", "32-byte-key-for-aes-256-encryption!!"),
	}
	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}
