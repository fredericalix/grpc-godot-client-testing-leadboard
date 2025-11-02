package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration
type Config struct {
	// Database connection URL
	DatabaseURL string

	// gRPC server port
	GRPCPort string

	// REST API port
	RESTPort string

	// Log level (debug, info, warn, error)
	LogLevel string

	// Default limit for leaderboard queries
	DefaultLimit int32

	// Maximum limit for leaderboard queries
	MaxLimit int32
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://leaderboard:leaderboard@localhost:5432/leaderboard?sslmode=disable"),
		GRPCPort:     getEnv("GRPC_PORT", "50051"),
		RESTPort:     getEnv("REST_PORT", "8080"),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
		DefaultLimit: getEnvInt32("DEFAULT_LIMIT", 10),
		MaxLimit:     getEnvInt32("MAX_LIMIT", 100),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.GRPCPort == "" {
		return fmt.Errorf("GRPC_PORT is required")
	}
	if c.RESTPort == "" {
		return fmt.Errorf("REST_PORT is required")
	}
	if c.DefaultLimit <= 0 {
		return fmt.Errorf("DEFAULT_LIMIT must be positive")
	}
	if c.MaxLimit <= 0 || c.MaxLimit < c.DefaultLimit {
		return fmt.Errorf("MAX_LIMIT must be positive and >= DEFAULT_LIMIT")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt32(key string, defaultValue int32) int32 {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.ParseInt(value, 10, 32); err == nil {
			return int32(i)
		}
	}
	return defaultValue
}
