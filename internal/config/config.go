package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const minJWTSecretLength = 32

type Config struct {
	// Server
	Port           string
	Env            string
	MaxRequestSize int64 // bytes
	AllowedOrigins []string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// JWT
	JWTSecret         string
	JWTAccessTTLHours int

	// MinIO
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
	MinIOUseSSL    bool
	MinIOMaxSizeMB int64  // max upload size in MB
	MinIOPublicURL string // public base URL to rewrite presigned URL host (e.g. https://minio.example.com)
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	jwtSecret := getEnv("JWT_SECRET", "")
	if len(jwtSecret) < minJWTSecretLength {
		return nil, fmt.Errorf(
			"JWT_SECRET must be at least %d characters (got %d)",
			minJWTSecretLength, len(jwtSecret),
		)
	}

	accessTTL, _ := strconv.Atoi(getEnv("JWT_ACCESS_TTL_HOURS", "24"))
	if accessTTL <= 0 {
		accessTTL = 24
	}

	maxSize, _ := strconv.ParseInt(getEnv("MAX_REQUEST_SIZE_MB", "10"), 10, 64)
	if maxSize <= 0 {
		maxSize = 10
	}

	minioMaxMB, _ := strconv.ParseInt(getEnv("MINIO_MAX_UPLOAD_MB", "5"), 10, 64)
	if minioMaxMB <= 0 {
		minioMaxMB = 5
	}

	originsRaw := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	origins := []string{}
	for _, o := range strings.Split(originsRaw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}

	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		Env:            getEnv("ENV", "development"),
		MaxRequestSize: maxSize * 1024 * 1024,
		AllowedOrigins: origins,

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "inventory"),
		DBPassword: getEnv("DB_PASSWORD", "inventory"),
		DBName:     getEnv("DB_NAME", "inventory"),
		DBSSLMode:  getEnv("DB_SSL_MODE", "disable"),

		JWTSecret:         jwtSecret,
		JWTAccessTTLHours: accessTTL,

		MinIOEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinIOBucket:    getEnv("MINIO_BUCKET", "inventory"),
		MinIOUseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",
		MinIOMaxSizeMB: minioMaxMB,
		MinIOPublicURL: getEnv("MINIO_PUBLIC_URL", ""),
	}

	return cfg, nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		c.DBHost, c.DBUser, c.DBPassword, c.DBName, c.DBPort, c.DBSSLMode,
	)
}

func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

func getEnv(key, defaultValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultValue
}
