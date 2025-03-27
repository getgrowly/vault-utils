package config

import (
	"os"
	"strconv"
	"time"
)

// Config represents the application configuration
type Config struct {
	// VaultNamespace is the Kubernetes namespace where Vault is running
	VaultNamespace string
	// VaultPort is the port number where Vault is listening
	VaultPort string
	// CheckInterval is the interval between Vault status checks
	CheckInterval time.Duration
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	cfg := &Config{
		VaultNamespace: getEnvOrDefault("VAULT_NAMESPACE", "vault"),
		VaultPort:     getEnvOrDefault("VAULT_PORT", "8200"),
		CheckInterval: time.Duration(getEnvAsIntOrDefault("CHECK_INTERVAL", 10)) * time.Second,
	}

	return cfg
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsIntOrDefault returns the value of an environment variable as an integer or a default value
func getEnvAsIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
} 