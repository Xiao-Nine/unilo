package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Storage  StorageConfig
	Agent    AgentConfig
}

type AppConfig struct {
	Env             string
	Name            string
	Version         string
	HTTPAddr        string
	ServerPublicURL string
	ServiceSecret   string
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

type StorageConfig struct {
	Endpoint       string
	AccessKey      string
	SecretKey      string
	Bucket         string
	Region         string
	UseSSL         bool
	MaxUploadBytes int64
}

type AgentConfig struct {
	Enabled            bool
	BaseURL            string
	APIKey             string
	Model              string
	Timeout            time.Duration
	MaxHistoryMessages int
	MaxContextResults  int
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		App: AppConfig{
			Env:             getEnv("APP_ENV", "development"),
			Name:            getEnv("APP_NAME", "Unilo"),
			Version:         getEnv("APP_VERSION", "0.1.0"),
			HTTPAddr:        getEnv("HTTP_ADDR", getEnv("APP_PORT", ":8080")),
			ServerPublicURL: getEnv("SERVER_PUBLIC_URL", getEnv("VITE_DEFAULT_SERVER_URL", "http://localhost:8080")),
			ServiceSecret:   getEnv("SERVICE_SECRET_KEY", getEnv("APP_SERVICE_SECRET", devDefault("change-me"))),
		},
		Database: DatabaseConfig{URL: databaseURL()},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "127.0.0.1:8002"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			AccessSecret:  getEnv("JWT_ACCESS_SECRET", devDefault("change-me-access-secret")),
			RefreshSecret: getEnv("JWT_REFRESH_SECRET", devDefault("change-me-refresh-secret")),
			AccessTTL:     getEnvDuration("JWT_ACCESS_TTL", 2*time.Hour),
			RefreshTTL:    getEnvDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		},
		Storage: StorageConfig{
			Endpoint:       getEnv("STORAGE_ENDPOINT", getEnv("S3_ENDPOINT", "127.0.0.1:8003")),
			AccessKey:      getEnv("STORAGE_ACCESS_KEY", getEnv("S3_ACCESS_KEY_ID", getEnv("MINIO_ROOT_USER", "unilo"))),
			SecretKey:      getEnv("STORAGE_SECRET_KEY", getEnv("S3_SECRET_ACCESS_KEY", getEnv("MINIO_ROOT_PASSWORD", "change-me-minio"))),
			Bucket:         getEnv("STORAGE_BUCKET", getEnv("S3_BUCKET", "unilo")),
			Region:         getEnv("STORAGE_REGION", getEnv("S3_REGION", "us-east-1")),
			UseSSL:         getEnvBool("STORAGE_USE_SSL", getEnvBool("S3_USE_SSL", false)),
			MaxUploadBytes: getEnvInt64("MAX_UPLOAD_BYTES", 100*1024*1024),
		},
		Agent: AgentConfig{
			Enabled:            getEnvBool("AGENT_ENABLED", true),
			BaseURL:            getEnv("OPENAI_BASE_URL", getEnv("AI_BASE_URL", "https://api.openai.com/v1")),
			APIKey:             getEnv("OPENAI_API_KEY", getEnv("AI_API_KEY", "")),
			Model:              getEnv("OPENAI_MODEL", getEnv("AI_MODEL", "gpt-4o-mini")),
			Timeout:            getEnvDuration("AGENT_TIMEOUT", 30*time.Second),
			MaxHistoryMessages: getEnvInt("AGENT_MAX_HISTORY_MESSAGES", 20),
			MaxContextResults:  getEnvInt("AGENT_MAX_CONTEXT_RESULTS", 8),
		},
	}

	return cfg, cfg.validate()
}

func (c Config) APIBaseURL() string {
	return c.App.ServerPublicURL + "/api/v1"
}

func (c Config) validate() error {
	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL or POSTGRES_* config is required")
	}
	if c.App.Env == "production" {
		if c.App.ServiceSecret == "" || c.App.ServiceSecret == "change-me" {
			return fmt.Errorf("SERVICE_SECRET_KEY must be set in production")
		}
		if c.JWT.AccessSecret == "" || c.JWT.AccessSecret == "change-me-access-secret" {
			return fmt.Errorf("JWT_ACCESS_SECRET must be set in production")
		}
		if c.JWT.RefreshSecret == "" || c.JWT.RefreshSecret == "change-me-refresh-secret" {
			return fmt.Errorf("JWT_REFRESH_SECRET must be set in production")
		}
		if c.Agent.Enabled {
			if c.Agent.BaseURL == "" {
				return fmt.Errorf("OPENAI_BASE_URL must be set when agent is enabled in production")
			}
			if c.Agent.APIKey == "" {
				return fmt.Errorf("OPENAI_API_KEY must be set when agent is enabled in production")
			}
			if c.Agent.Model == "" {
				return fmt.Errorf("OPENAI_MODEL must be set when agent is enabled in production")
			}
		}
	}
	return nil
}

func databaseURL() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}

	user := getEnv("POSTGRES_USER", "unilo")
	password := getEnv("POSTGRES_PASSWORD", "unilo")
	db := getEnv("POSTGRES_DB", "unilo")
	host := getEnv("POSTGRES_HOST", "127.0.0.1")
	port := getEnv("POSTGRES_PORT", "8001")

	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   host + ":" + port,
		Path:   db,
	}
	q := u.Query()
	q.Set("sslmode", getEnv("POSTGRES_SSLMODE", "disable"))
	u.RawQuery = q.Encode()
	return u.String()
}

func getEnv(key string, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvInt64(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func devDefault(value string) string {
	if os.Getenv("APP_ENV") == "production" {
		return ""
	}
	return value
}
