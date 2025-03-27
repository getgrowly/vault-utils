package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckVaultStatus(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse VaultStatus
		serverStatus   int
		expectError    bool
	}{
		{
			name: "vault is sealed",
			serverResponse: VaultStatus{
				Sealed:      true,
				Initialized: true,
			},
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name: "vault is unsealed",
			serverResponse: VaultStatus{
				Sealed:      false,
				Initialized: true,
			},
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name: "vault is not initialized",
			serverResponse: VaultStatus{
				Sealed:      true,
				Initialized: false,
			},
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name:           "server error",
			serverResponse: VaultStatus{},
			serverStatus:   http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.serverStatus)
				if err := json.NewEncoder(w).Encode(tt.serverResponse); err != nil {
					t.Fatalf("Failed to encode response: %v", err)
				}
			}))
			defer server.Close()

			status, err := checkVaultStatus(server.URL)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if status.Sealed != tt.serverResponse.Sealed {
				t.Errorf("expected sealed=%v, got sealed=%v", tt.serverResponse.Sealed, status.Sealed)
			}
			if status.Initialized != tt.serverResponse.Initialized {
				t.Errorf("expected initialized=%v, got initialized=%v", tt.serverResponse.Initialized, status.Initialized)
			}
		})
	}
}

func TestUnsealVault(t *testing.T) {
	// Create temporary directory for test keys
	tmpDir := t.TempDir()
	keysDir := filepath.Join(tmpDir, "unseal-keys")
	if err := os.MkdirAll(keysDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create test keys
	testKeys := []string{"key1", "key2", "key3"}
	for _, key := range testKeys {
		err := os.WriteFile(filepath.Join(keysDir, key), []byte("test-key-"+key), 0644)
		if err != nil {
			t.Fatalf("failed to create test key: %v", err)
		}
	}

	tests := []struct {
		name           string
		serverResponses []UnsealResponse
		serverStatus   int
		expectError    bool
	}{
		{
			name: "successful unseal",
			serverResponses: []UnsealResponse{
				{Sealed: true},
				{Sealed: true},
				{Sealed: false},
			},
			serverStatus: http.StatusOK,
			expectError:  false,
		},
		{
			name: "server error",
			serverResponses: []UnsealResponse{},
			serverStatus:   http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyIndex := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.serverStatus)
				if tt.serverStatus == http.StatusOK && keyIndex < len(tt.serverResponses) {
					if err := json.NewEncoder(w).Encode(tt.serverResponses[keyIndex]); err != nil {
						t.Fatalf("Failed to encode response: %v", err)
					}
					keyIndex++
				}
			}))
			defer server.Close()

			// Temporarily override the keys directory for testing
			originalKeysDir := "/vault/unseal-keys"
			os.Setenv("VAULT_UNSEAL_KEYS_DIR", keysDir)
			defer os.Setenv("VAULT_UNSEAL_KEYS_DIR", originalKeysDir)

			err := unsealVault(server.URL)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestHealthCheckEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/ready":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tests := []struct {
		name       string
		endpoint   string
		wantStatus int
	}{
		{
			name:       "health endpoint",
			endpoint:   "/health",
			wantStatus: http.StatusOK,
		},
		{
			name:       "ready endpoint",
			endpoint:   "/ready",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(server.URL + tt.endpoint)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("expected status %v, got %v", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

func TestMainLoop(t *testing.T) {
	// This is a basic test to ensure the main loop doesn't crash
	// In a real environment, you'd want to test more thoroughly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(VaultStatus{
			Sealed:      false,
			Initialized: true,
		}); err != nil {
			t.Fatalf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Set environment variables for testing
	os.Setenv("VAULT_SERVICE", "localhost")
	os.Setenv("VAULT_PORT", "8200")
	os.Setenv("CHECK_INTERVAL", "1")

	// Start the main loop in a goroutine
	go main()

	// Let it run for a short time
	time.Sleep(2 * time.Second)
} 