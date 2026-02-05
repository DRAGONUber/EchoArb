// internal/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config holds all application configuration
type Config struct {
	// Service config
	Environment string `json:"environment"`
	MetricsPort int    `json:"metrics_port"`
	LogLevel    string `json:"log_level"`

	// Redis config
	Redis RedisConfig `json:"redis"`

	// API endpoints
	KalshiWSURL    string `json:"kalshi_ws_url"`
	PolyWSURL      string `json:"poly_ws_url"`

	// Kalshi authentication
	KalshiAPIKey        string `json:"kalshi_api_key"`
	KalshiPrivateKeyPEM string `json:"kalshi_private_key_pem"` // Path to PEM file

	// Market subscriptions
	Subscriptions []MarketSubscription `json:"subscriptions"`

	// Connection settings
	Reconnect ReconnectConfig `json:"reconnect"`
}

type RedisConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Password     string `json:"password"`
	DB           int    `json:"db"`
	PoolSize     int    `json:"pool_size"`
	MinIdleConns int    `json:"min_idle_conns"`
}

type ReconnectConfig struct {
	InitialInterval time.Duration `json:"initial_interval"`
	MaxInterval     time.Duration `json:"max_interval"`
	MaxRetries      int           `json:"max_retries"` // 0 = infinite
}

type MarketSubscription struct {
	ID          string           `json:"id"`
	Description string           `json:"description"`
	Kalshi      *KalshiMarket    `json:"kalshi,omitempty"`
	Polymarket  *PolymarketMarket `json:"polymarket,omitempty"`
}

type KalshiMarket struct {
	Ticker string `json:"ticker"`
}

type PolymarketMarket struct {
	TokenID string `json:"token_id"`
}

// Load reads configuration from file and environment variables
func Load() (*Config, error) {
	// Default configuration
	cfg := &Config{
		Environment: getEnv("ENVIRONMENT", "development"),
		MetricsPort: getEnvInt("METRICS_PORT", 9090),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		Redis: RedisConfig{
			Host:         getEnv("REDIS_HOST", "localhost"),
			Port:         getEnvInt("REDIS_PORT", 6379),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           getEnvInt("REDIS_DB", 0),
			PoolSize:     getEnvInt("REDIS_POOL_SIZE", 10),
			MinIdleConns: getEnvInt("REDIS_MIN_IDLE_CONNS", 5),
		},
		KalshiWSURL:    getEnv("KALSHI_WS_URL", "wss://api.elections.kalshi.com/trade-api/ws/v2"),
		PolyWSURL:      getEnv("POLY_WS_URL", "wss://ws-subscriptions-clob.polymarket.com/ws"),
		
		// Kalshi auth
		KalshiAPIKey:        getEnv("KALSHI_API_KEY", ""),
		KalshiPrivateKeyPEM: getEnv("KALSHI_PRIVATE_KEY_PATH", "./keys/kalshi_private_key.pem"),
		
		Reconnect: ReconnectConfig{
			InitialInterval: 5 * time.Second,
			MaxInterval:     5 * time.Minute,
			MaxRetries:      0, // Infinite retries
		},
	}

	// Load market subscriptions from JSON file if provided
	configPath := getEnv("CONFIG_PATH", "./config/market_pairs.json")
	if err := loadSubscriptions(configPath, cfg); err != nil {
		return nil, fmt.Errorf("failed to load market subscriptions: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

func loadSubscriptions(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		// Config file is optional in development
		if os.IsNotExist(err) && cfg.Environment == "development" {
			cfg.Subscriptions = []MarketSubscription{} // Empty subscriptions for testing
			return nil
		}
		return err
	}

	var subscriptionsConfig struct {
		Subscriptions []MarketSubscription `json:"subscriptions"`
	}

	if err := json.Unmarshal(data, &subscriptionsConfig); err != nil {
		return err
	}

	cfg.Subscriptions = subscriptionsConfig.Subscriptions
	return nil
}

// Validate checks if configuration is valid
func (c *Config) Validate() error {
	if c.Redis.Host == "" {
		return fmt.Errorf("redis host is required")
	}
	
	if c.KalshiAPIKey == "" {
		return fmt.Errorf("KALSHI_API_KEY environment variable is required")
	}

	if c.KalshiPrivateKeyPEM == "" {
		return fmt.Errorf("KALSHI_PRIVATE_KEY_PATH environment variable is required")
	}

	// Check if private key file exists
	if _, err := os.Stat(c.KalshiPrivateKeyPEM); os.IsNotExist(err) {
		return fmt.Errorf("Kalshi private key file not found: %s", c.KalshiPrivateKeyPEM)
	}

	return nil
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}
