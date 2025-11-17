package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the control plane
type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	Billing    BillingConfig
	Security   SecurityConfig
	Monitoring MonitoringConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
}

// BillingConfig holds billing configuration
type BillingConfig struct {
	StripeSecretKey      string
	StripeWebhookSecret  string
	AggregationInterval  time.Duration
	ExportInterval       time.Duration
}

// SecurityConfig holds security configuration
type SecurityConfig struct {
	APIKeyHashRounds int
	JWTSecret        string
	TLSEnabled       bool
	TLSCertPath      string
	TLSKeyPath       string
}

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	Enabled           bool
	PrometheusPort    int
	MetricsPath       string
	LogLevel          string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         getEnvAsInt("SERVER_PORT", 8080),
			ReadTimeout:  getEnvAsDuration("SERVER_READ_TIMEOUT", "30s"),
			WriteTimeout: getEnvAsDuration("SERVER_WRITE_TIMEOUT", "30s"),
			IdleTimeout:  getEnvAsDuration("SERVER_IDLE_TIMEOUT", "120s"),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvAsInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "crosslogic"),
			Password:        getEnv("DB_PASSWORD", ""),
			Database:        getEnv("DB_NAME", "crosslogic_iaas"),
			SSLMode:         getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvAsDuration("DB_CONN_MAX_LIFETIME", "5m"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvAsInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
			PoolSize: getEnvAsInt("REDIS_POOL_SIZE", 10),
		},
		Billing: BillingConfig{
			StripeSecretKey:      getEnv("STRIPE_SECRET_KEY", ""),
			StripeWebhookSecret:  getEnv("STRIPE_WEBHOOK_SECRET", ""),
			AggregationInterval:  getEnvAsDuration("BILLING_AGGREGATION_INTERVAL", "1h"),
			ExportInterval:       getEnvAsDuration("BILLING_EXPORT_INTERVAL", "5m"),
		},
		Security: SecurityConfig{
			APIKeyHashRounds: getEnvAsInt("API_KEY_HASH_ROUNDS", 12),
			JWTSecret:        getEnv("JWT_SECRET", ""),
			TLSEnabled:       getEnvAsBool("TLS_ENABLED", false),
			TLSCertPath:      getEnv("TLS_CERT_PATH", ""),
			TLSKeyPath:       getEnv("TLS_KEY_PATH", ""),
		},
		Monitoring: MonitoringConfig{
			Enabled:        getEnvAsBool("MONITORING_ENABLED", true),
			PrometheusPort: getEnvAsInt("PROMETHEUS_PORT", 9090),
			MetricsPath:    getEnv("METRICS_PATH", "/metrics"),
			LogLevel:       getEnv("LOG_LEVEL", "info"),
		},
	}

	// Validate required fields
	if cfg.Database.Password == "" {
		return nil, fmt.Errorf("DB_PASSWORD is required")
	}

	if cfg.Billing.StripeSecretKey == "" {
		return nil, fmt.Errorf("STRIPE_SECRET_KEY is required")
	}

	return cfg, nil
}

// Helper functions for environment variable parsing
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsDuration(key string, defaultValue string) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		valueStr = defaultValue
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		duration, _ := time.ParseDuration(defaultValue)
		return duration
	}
	return value
}
