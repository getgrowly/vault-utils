package vault

import (
	"encoding/json"
	"fmt"
	"io"
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
			responseStatus: http.StatusNotImplemented,
			responseBody:   VaultStatus{Sealed: true, Initialized: false},
			expectError:    false,
		},
		{
			name:           "vault is sealed with 503",
			responseStatus: http.StatusServiceUnavailable,
			responseBody:   VaultStatus{Sealed: true, Initialized: true},
			expectError:    false,
		},
		{
			name:           "vault is in standby",
			responseStatus: http.StatusTooManyRequests,
			responseBody:   VaultStatus{Sealed: false, Initialized: true},
			expectError:    false,
		},
		{
			name:           "vault is DR secondary",
			responseStatus: 472,
			responseBody:   VaultStatus{Sealed: false, Initialized: true},
			expectError:    false,
		},
		{
			name:           "vault is performance standby",
			responseStatus: 473,
			responseBody:   VaultStatus{Sealed: false, Initialized: true},
			expectError:    false,
		},
		{
			name:           "invalid status code",
			responseStatus: http.StatusBadRequest,
			responseBody:   VaultStatus{},
			expectError:    true,
		},
		{
			name:           "invalid response body",
			responseStatus: http.StatusOK,
			responseBody:   VaultStatus{},
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/sys/health" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.responseStatus)
				if tt.responseStatus != http.StatusBadRequest {
					if err := json.NewEncoder(w).Encode(tt.responseBody); err != nil {
						t.Errorf("failed to encode response: %v", err)
					}
				} else {
					// Write invalid JSON for the error case
					w.Write([]byte("{invalid json"))
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

func TestInitialize(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   *InitResponse
		expectError    bool
	}{
		{
			name:           "successful initialization",
			responseStatus: http.StatusOK,
			responseBody: &InitResponse{
				Keys:       []string{"key1", "key2", "key3", "key4", "key5"},
				RootToken:  "root-token",
				KeysBase64: []string{"key1-base64", "key2-base64", "key3-base64", "key4-base64", "key5-base64"},
			},
			expectError: false,
		},
		{
			name:           "already initialized",
			responseStatus: http.StatusBadRequest,
			responseBody:   nil,
			expectError:    true,
		},
		{
			name:           "server error",
			responseStatus: http.StatusInternalServerError,
			responseBody:   nil,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("expected PUT request, got %s", r.Method)
				}
				if r.URL.Path != "/v1/sys/init" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}

				// Verify request body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("failed to read request body: %v", err)
				}
				var req InitRequest
				if err := json.Unmarshal(body, &req); err != nil {
					t.Errorf("failed to parse request body: %v", err)
				}
				if req.SecretShares != 5 || req.SecretThreshold != 3 {
					t.Errorf("unexpected request body: %+v", req)
				}

				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != nil {
					if err := json.NewEncoder(w).Encode(tt.responseBody); err != nil {
						t.Errorf("failed to encode response: %v", err)
					}
				}
			}))
			defer server.Close()

			client := NewClient(server.URL)
			resp, err := client.Initialize()
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

			if len(resp.Keys) != 5 {
				t.Errorf("expected 5 keys, got %d", len(resp.Keys))
			}
			if resp.RootToken != tt.responseBody.RootToken {
				t.Errorf("expected root token %s, got %s", tt.responseBody.RootToken, resp.RootToken)
			}
		})
	}
}

func TestUnsealWithKey(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   *UnsealResponse
		expectError    bool
	}{
		{
			name:           "successful unseal",
			responseStatus: http.StatusOK,
			responseBody:   &UnsealResponse{Sealed: false},
			expectError:    false,
		},
		{
			name:           "still sealed",
			responseStatus: http.StatusOK,
			responseBody:   &UnsealResponse{Sealed: true},
			expectError:    false,
		},
		{
			name:           "server error",
			responseStatus: http.StatusInternalServerError,
			responseBody:   nil,
			expectError:    true,
		},
		{
			name:           "invalid key",
			responseStatus: http.StatusBadRequest,
			responseBody:   nil,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST request, got %s", r.Method)
				}
				if r.URL.Path != "/v1/sys/unseal" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}

				// Verify request body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("failed to read request body: %v", err)
				}
				var req map[string]string
				if err := json.Unmarshal(body, &req); err != nil {
					t.Errorf("failed to parse request body: %v", err)
				}
				if _, ok := req["key"]; !ok {
					t.Error("key not found in request body")
				}

				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != nil {
					if err := json.NewEncoder(w).Encode(tt.responseBody); err != nil {
						t.Errorf("failed to encode response: %v", err)
					}
				}
			}))
			defer server.Close()

			client := NewClient(server.URL)
			err := client.UnsealWithKey("test-key")
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
				if err := os.WriteFile(keyPath, []byte(fmt.Sprintf("test-key-%d", i)), 0600); err != nil {
					t.Fatalf("failed to write key file: %v", err)
				}
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
				if tt.responseStatus == http.StatusOK {
					if err := json.NewEncoder(w).Encode(tt.responseBody); err != nil {
						t.Errorf("failed to encode response: %v", err)
					}
				}
			}))
			defer server.Close()

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
