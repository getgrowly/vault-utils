package vault

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckStatus(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   VaultStatus
		expectError    bool
	}{
		{
			name:           "vault is sealed",
			responseStatus: http.StatusOK,
			responseBody:   VaultStatus{Sealed: true, Initialized: true},
			expectError:    false,
		},
		{
			name:           "vault is unsealed",
			responseStatus: http.StatusOK,
			responseBody:   VaultStatus{Sealed: false, Initialized: true},
			expectError:    false,
		},
		{
			name:           "vault is not initialized",
			responseStatus: http.StatusOK,
			responseBody:   VaultStatus{Sealed: true, Initialized: false},
			expectError:    false,
		},
		{
			name:           "server error",
			responseStatus: http.StatusInternalServerError,
			responseBody:   VaultStatus{},
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
				if tt.responseStatus == http.StatusOK {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			client := NewClient(server.URL)
			status, err := client.CheckStatus()
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if status.Sealed != tt.responseBody.Sealed {
				t.Errorf("expected sealed=%v, got sealed=%v", tt.responseBody.Sealed, status.Sealed)
			}

			if status.Initialized != tt.responseBody.Initialized {
				t.Errorf("expected initialized=%v, got initialized=%v", tt.responseBody.Initialized, status.Initialized)
			}
		})
	}
}

func TestUnsealWithKeysFromDir(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   UnsealResponse
		expectError    bool
	}{
		{
			name:           "successful unseal",
			responseStatus: http.StatusOK,
			responseBody:   UnsealResponse{Sealed: false},
			expectError:    false,
		},
		{
			name:           "server error",
			responseStatus: http.StatusInternalServerError,
			responseBody:   UnsealResponse{},
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test keys
			tempDir, err := os.MkdirTemp("", "TestUnsealVault")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			// Create unseal keys directory
			keysDir := filepath.Join(tempDir, "unseal-keys")
			if err := os.MkdirAll(keysDir, 0755); err != nil {
				t.Fatalf("failed to create keys dir: %v", err)
			}

			// Create test key files
			for i := 1; i <= 3; i++ {
				keyPath := filepath.Join(keysDir, fmt.Sprintf("key%d", i))
				if err := os.WriteFile(keyPath, []byte(fmt.Sprintf("test-key-%d", i)), 0644); err != nil {
					t.Fatalf("failed to write key file: %v", err)
				}
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
				if tt.responseStatus == http.StatusOK {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			os.Setenv("VAULT_UNSEAL_KEYS_DIR", keysDir)
			defer os.Unsetenv("VAULT_UNSEAL_KEYS_DIR")

			client := NewClient(server.URL)
			err = client.UnsealWithKeysFromDir(keysDir)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
} 