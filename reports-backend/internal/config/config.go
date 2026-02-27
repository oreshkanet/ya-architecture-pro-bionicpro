package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port               string
	OLAPDSN            string
	S3Endpoint         string
	S3Bucket           string
	S3AccessKey        string
	S3SecretKey        string
	ReportServeBaseURL string
	ReportTokenSecret  string
	ReportTokenTTL     int
	S3ReportTTLDays    int
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:               getEnv("PORT", "8000"),
		OLAPDSN:            getEnv("OLAP_DSN", "postgres://olap_user:olap_password@olap_db:5432/olap_db?sslmode=disable"),
		S3Endpoint:         getEnv("S3_ENDPOINT", "http://minio:9000"),
		S3Bucket:           getEnv("S3_BUCKET", "reports"),
		S3AccessKey:        getEnv("S3_ACCESS_KEY", "minio_user"),
		S3SecretKey:        getEnv("S3_SECRET_KEY", "minio_password"),
		ReportServeBaseURL: getEnv("REPORT_SERVE_BASE_URL", "http://localhost:8001/api/reports/serve"),
		ReportTokenSecret:  getEnv("REPORT_TOKEN_SECRET", "change-me-report-token-secret"),
		ReportTokenTTL:     getEnvInt("REPORT_TOKEN_TTL_MIN", 60),
		S3ReportTTLDays:    getEnvInt("S3_REPORT_TTL_DAYS", 7),
	}
	return cfg, nil
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
