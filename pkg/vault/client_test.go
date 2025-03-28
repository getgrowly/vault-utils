package vault

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockTransport implements http.RoundTripper for testing
type mockTransport struct {
	responses []*http.Response
	current   int
}

func (t *mockTransport) RoundTrip(*http.Request) (*http.Response, error) {
	if t.current >= len(t.responses) {
		return nil, fmt.Errorf("no more responses available")
	}
	response := t.responses[t.current]
	t.current++
	return response, nil
}

func TestCheckStatus(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedError  bool
		expectedStatus *Status
	}{
		{
			name:       "success - initialized and unsealed",
			statusCode: http.StatusOK,
			responseBody: `{
				"initialized": true,
				"sealed": false
			}`,
			expectedError: false,
			expectedStatus: &Status{
				Initialized: true,
				Sealed:      false,
			},
		},
		{
			name:          "error - API not found",
			statusCode:    http.StatusNotFound,
			responseBody:  "",
			expectedError: true,
		},
		{
			name:       "error - invalid JSON response",
			statusCode: http.StatusOK,
			responseBody: `{
				"initialized": true,
				"sealed": invalid
			}`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/sys/seal-status" {
					t.Errorf("Expected to request '/v1/sys/seal-status', got: %s", r.URL.Path)
				}
				if r.Method != http.MethodGet {
					t.Errorf("Expected GET request, got: %s", r.Method)
				}
				w.WriteHeader(tt.statusCode)
				fmt.Fprintln(w, tt.responseBody)
			}))
			defer server.Close()

			client := NewClient(server.URL)
			status, err := client.CheckStatus()

			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectedStatus != nil {
				if status.Initialized != tt.expectedStatus.Initialized {
					t.Errorf("Expected initialized=%v, got %v", tt.expectedStatus.Initialized, status.Initialized)
				}
				if status.Sealed != tt.expectedStatus.Sealed {
					t.Errorf("Expected sealed=%v, got %v", tt.expectedStatus.Sealed, status.Sealed)
				}
			}
		})
	}
}

func TestInitialize(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectedError bool
		expectedResp  *InitResponse
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			responseBody: `{
				"keys": ["key1", "key2", "key3"],
				"root_token": "root-token"
			}`,
			expectedError: false,
			expectedResp: &InitResponse{
				Keys:      []string{"key1", "key2", "key3"},
				RootToken: "root-token",
			},
		},
		{
			name:          "error - server error",
			statusCode:    http.StatusInternalServerError,
			responseBody:  "",
			expectedError: true,
		},
		{
			name:       "error - invalid JSON response",
			statusCode: http.StatusOK,
			responseBody: `{
				"keys": ["key1", "key2", "key3"],
				"root_token": invalid
			}`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/sys/init" {
					t.Errorf("Expected to request '/v1/sys/init', got: %s", r.URL.Path)
				}
				if r.Method != http.MethodPut {
					t.Errorf("Expected PUT request, got: %s", r.Method)
				}

				var req InitRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Errorf("Error decoding request body: %v", err)
				}

				if req.SecretShares != defaultSecretShares {
					t.Errorf("Expected secret_shares=%d, got %d", defaultSecretShares, req.SecretShares)
				}
				if req.SecretThreshold != defaultSecretThreshold {
					t.Errorf("Expected secret_threshold=%d, got %d", defaultSecretThreshold, req.SecretThreshold)
				}

				w.WriteHeader(tt.statusCode)
				fmt.Fprintln(w, tt.responseBody)
			}))
			defer server.Close()

			client := NewClient(server.URL)
			resp, err := client.Initialize()

			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectedResp != nil {
				if len(resp.Keys) != len(tt.expectedResp.Keys) {
					t.Errorf("Expected %d keys, got %d", len(tt.expectedResp.Keys), len(resp.Keys))
				}
				for i := range resp.Keys {
					if resp.Keys[i] != tt.expectedResp.Keys[i] {
						t.Errorf("Expected key[%d]=%s, got %s", i, tt.expectedResp.Keys[i], resp.Keys[i])
					}
				}
				if resp.RootToken != tt.expectedResp.RootToken {
					t.Errorf("Expected root_token=%s, got %s", tt.expectedResp.RootToken, resp.RootToken)
				}
			}
		})
	}
}

func TestUnsealWithKey(t *testing.T) {
	tests := []struct {
		name            string
		serverResponses []*http.Response
		expectError     bool
	}{
		{
			name: "success",
			serverResponses: []*http.Response{
				{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"sealed": false}`)),
				},
			},
			expectError: false,
		},
		{
			name: "error - server error",
			serverResponses: []*http.Response{
				{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader("")),
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				baseURL: "http://test:8200",
				httpClient: &http.Client{
					Transport: &mockTransport{
						responses: tt.serverResponses,
					},
				},
			}

			err := client.UnsealWithKey("test-key")
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestUnsealWithKeysFromDir(t *testing.T) {
	tests := []struct {
		name            string
		serverResponses []*http.Response
		expectError     bool
	}{
		{
			name: "success",
			serverResponses: []*http.Response{
				{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"sealed": true}`)),
				},
				{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"sealed": true}`)),
				},
				{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"sealed": false}`)),
				},
			},
			expectError: false,
		},
		{
			name: "error - server error",
			serverResponses: []*http.Response{
				{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader("")),
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				baseURL: "http://test:8200",
				httpClient: &http.Client{
					Transport: &mockTransport{
						responses: tt.serverResponses,
					},
				},
			}

			err := client.UnsealWithKeysFromDir([]string{"key1", "key2", "key3"})
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}
