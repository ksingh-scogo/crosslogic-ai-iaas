package notifications

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the configuration for the notification service
type Config struct {
	// Discord configuration
	DiscordEnabled    bool
	DiscordWebhookURL string

	// Slack configuration
	SlackEnabled    bool
	SlackWebhookURL string
	SlackChannel    string

	// Email configuration (using Resend)
	EmailEnabled  bool
	EmailProvider string // Currently only "resend"
	EmailFrom     string
	EmailTo       []string
	ResendAPIKey  string

	// Generic webhook configuration
	WebhookEnabled bool
	WebhookURL     string
	WebhookSecret  string
	WebhookMethod  string
	WebhookHeaders map[string]string

	// Retry configuration
	MaxRetries       int
	RetryBackoffBase time.Duration
	RetryQueueSize   int
	RetryWorkers     int

	// Event routing: map event types to channels
	// e.g., {"payment.succeeded": ["discord", "slack", "email"]}
	EventRouting map[string][]string

	// General settings
	Enabled           bool
	AsyncDelivery     bool
	DeliveryTimeout   time.Duration
	MaxConcurrent     int
}

// LoadConfig loads the notification configuration from environment variables
func LoadConfig() (*Config, error) {
	cfg := &Config{
		// Discord
		DiscordEnabled:    getEnvBool("NOTIFICATIONS_DISCORD_ENABLED", false),
		DiscordWebhookURL: os.Getenv("NOTIFICATIONS_DISCORD_WEBHOOK_URL"),

		// Slack
		SlackEnabled:    getEnvBool("NOTIFICATIONS_SLACK_ENABLED", false),
		SlackWebhookURL: os.Getenv("NOTIFICATIONS_SLACK_WEBHOOK_URL"),
		SlackChannel:    getEnv("NOTIFICATIONS_SLACK_CHANNEL", "#notifications"),

		// Email (Resend)
		EmailEnabled:  getEnvBool("NOTIFICATIONS_EMAIL_ENABLED", false),
		EmailProvider: getEnv("NOTIFICATIONS_EMAIL_PROVIDER", "resend"),
		EmailFrom:     getEnv("NOTIFICATIONS_EMAIL_FROM", "noreply@crosslogic.ai"),
		EmailTo:       getEnvStringSlice("NOTIFICATIONS_EMAIL_TO", []string{"ops@crosslogic.ai"}),
		ResendAPIKey:  os.Getenv("NOTIFICATIONS_RESEND_API_KEY"),

		// Generic Webhook
		WebhookEnabled: getEnvBool("NOTIFICATIONS_WEBHOOK_ENABLED", false),
		WebhookURL:     os.Getenv("NOTIFICATIONS_WEBHOOK_URL"),
		WebhookSecret:  os.Getenv("NOTIFICATIONS_WEBHOOK_SECRET"),
		WebhookMethod:  getEnv("NOTIFICATIONS_WEBHOOK_METHOD", "POST"),
		WebhookHeaders: getEnvJSONMap("NOTIFICATIONS_WEBHOOK_HEADERS"),

		// Retry configuration
		MaxRetries:       getEnvInt("NOTIFICATIONS_MAX_RETRIES", 3),
		RetryBackoffBase: getEnvDuration("NOTIFICATIONS_RETRY_BACKOFF_BASE", 5*time.Second),
		RetryQueueSize:   getEnvInt("NOTIFICATIONS_RETRY_QUEUE_SIZE", 1000),
		RetryWorkers:     getEnvInt("NOTIFICATIONS_RETRY_WORKERS", 5),

		// Event routing
		EventRouting: getEnvEventRouting("NOTIFICATIONS_EVENT_ROUTING"),

		// General settings
		Enabled:         getEnvBool("NOTIFICATIONS_ENABLED", true),
		AsyncDelivery:   getEnvBool("NOTIFICATIONS_ASYNC_DELIVERY", true),
		DeliveryTimeout: getEnvDuration("NOTIFICATIONS_DELIVERY_TIMEOUT", 30*time.Second),
		MaxConcurrent:   getEnvInt("NOTIFICATIONS_MAX_CONCURRENT", 10),
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid notification config: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	// Check if at least one channel is enabled
	if !c.DiscordEnabled && !c.SlackEnabled && !c.EmailEnabled && !c.WebhookEnabled {
		return fmt.Errorf("no notification channels enabled")
	}

	// Validate Discord
	if c.DiscordEnabled && c.DiscordWebhookURL == "" {
		return fmt.Errorf("discord enabled but webhook URL not provided")
	}

	// Validate Slack
	if c.SlackEnabled && c.SlackWebhookURL == "" {
		return fmt.Errorf("slack enabled but webhook URL not provided")
	}

	// Validate Email
	if c.EmailEnabled {
		if c.ResendAPIKey == "" {
			return fmt.Errorf("email enabled but Resend API key not provided")
		}
		if c.EmailFrom == "" {
			return fmt.Errorf("email enabled but 'from' address not provided")
		}
		if len(c.EmailTo) == 0 {
			return fmt.Errorf("email enabled but no recipients specified")
		}
	}

	// Validate Generic Webhook
	if c.WebhookEnabled {
		if c.WebhookURL == "" {
			return fmt.Errorf("webhook enabled but URL not provided")
		}
		if c.WebhookMethod != "POST" && c.WebhookMethod != "PUT" {
			return fmt.Errorf("webhook method must be POST or PUT")
		}
	}

	// Validate retry settings
	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}
	if c.RetryBackoffBase <= 0 {
		return fmt.Errorf("retry backoff base must be positive")
	}
	if c.RetryQueueSize <= 0 {
		return fmt.Errorf("retry queue size must be positive")
	}

	return nil
}

// GetChannelsForEvent returns the list of notification channels for a given event type
func (c *Config) GetChannelsForEvent(eventType string) []string {
	if channels, ok := c.EventRouting[eventType]; ok {
		return channels
	}

	// Default: send to all enabled channels
	var channels []string
	if c.DiscordEnabled {
		channels = append(channels, "discord")
	}
	if c.SlackEnabled {
		channels = append(channels, "slack")
	}
	if c.EmailEnabled {
		channels = append(channels, "email")
	}
	if c.WebhookEnabled {
		channels = append(channels, "webhook")
	}

	return channels
}

// Helper functions for environment variables

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		b, err := strconv.ParseBool(value)
		if err != nil {
			return defaultValue
		}
		return b
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		i, err := strconv.Atoi(value)
		if err != nil {
			return defaultValue
		}
		return i
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		d, err := time.ParseDuration(value)
		if err != nil {
			return defaultValue
		}
		return d
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		var slice []string
		if err := json.Unmarshal([]byte(value), &slice); err != nil {
			return defaultValue
		}
		return slice
	}
	return defaultValue
}

func getEnvJSONMap(key string) map[string]string {
	if value := os.Getenv(key); value != "" {
		var m map[string]string
		if err := json.Unmarshal([]byte(value), &m); err != nil {
			return make(map[string]string)
		}
		return m
	}
	return make(map[string]string)
}

func getEnvEventRouting(key string) map[string][]string {
	if value := os.Getenv(key); value != "" {
		var routing map[string][]string
		if err := json.Unmarshal([]byte(value), &routing); err != nil {
			return make(map[string][]string)
		}
		return routing
	}
	return make(map[string][]string)
}
