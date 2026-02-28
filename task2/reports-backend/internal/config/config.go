package config

import (
	"os"
)

type Config struct {
	Port    string
	OLAPDSN string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:    getEnv("PORT", "8000"),
		OLAPDSN: getEnv("OLAP_DSN", "postgres://olap_user:olap_password@olap_db:5432/olap_db?sslmode=disable"),
	}
	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
