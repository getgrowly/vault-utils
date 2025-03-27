package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	// Test default values
	cfg := LoadConfig()
	if cfg.VaultNamespace != "vault" {
		t.Errorf("expected default namespace 'vault', got '%s'", cfg.VaultNamespace)
	}
	if cfg.VaultPort != "8200" {
		t.Errorf("expected default port '8200', got '%s'", cfg.VaultPort)
	}
	if cfg.CheckInterval != 10*time.Second {
		t.Errorf("expected default check interval 10s, got %v", cfg.CheckInterval)
	}

	// Test custom values
	os.Setenv("VAULT_NAMESPACE", "custom-namespace")
	os.Setenv("VAULT_PORT", "8201")
	os.Setenv("CHECK_INTERVAL", "20")
	defer func() {
		os.Unsetenv("VAULT_NAMESPACE")
		os.Unsetenv("VAULT_PORT")
		os.Unsetenv("CHECK_INTERVAL")
	}()

	cfg = LoadConfig()
	if cfg.VaultNamespace != "custom-namespace" {
		t.Errorf("expected namespace 'custom-namespace', got '%s'", cfg.VaultNamespace)
	}
	if cfg.VaultPort != "8201" {
		t.Errorf("expected port '8201', got '%s'", cfg.VaultPort)
	}
	if cfg.CheckInterval != 20*time.Second {
		t.Errorf("expected check interval 20s, got %v", cfg.CheckInterval)
	}

	// Test invalid check interval
	os.Setenv("CHECK_INTERVAL", "invalid")
	cfg = LoadConfig()
	if cfg.CheckInterval != 10*time.Second {
		t.Errorf("expected default check interval 10s for invalid input, got %v", cfg.CheckInterval)
	}
}
