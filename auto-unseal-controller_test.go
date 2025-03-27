package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCheckVaultStatus(t *testing.T) {
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

			status, err := checkVaultStatus(server.URL)
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

func TestUnsealVault(t *testing.T) {
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

			err = unsealVault(server.URL)
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

func TestHealthCheckEndpoints(t *testing.T) {
	tests := []struct {
		name       string
		endpoint   string
		expectCode int
	}{
		{
			name:       "health endpoint",
			endpoint:   "/health",
			expectCode: http.StatusOK,
		},
		{
			name:       "ready endpoint",
			endpoint:   "/ready",
			expectCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.endpoint, nil)
			w := httptest.NewRecorder()

			switch tt.endpoint {
			case "/health":
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}).ServeHTTP(w, req)
			case "/ready":
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}).ServeHTTP(w, req)
			}

			if w.Code != tt.expectCode {
				t.Errorf("expected status code %d, got %d", tt.expectCode, w.Code)
			}
		})
	}
}

func TestMainLoop(t *testing.T) {
	// Create a fake Kubernetes clientset
	clientset := fake.NewSimpleClientset()

	// Create test pods
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vault-0",
			Namespace: "vault",
			Labels: map[string]string{
				"app.kubernetes.io/name": "vault",
			},
		},
		Status: corev1.PodStatus{
			PodIP: "10.0.0.1",
		},
	}

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vault-1",
			Namespace: "vault",
			Labels: map[string]string{
				"app.kubernetes.io/name": "vault",
			},
		},
		Status: corev1.PodStatus{
			PodIP: "10.0.0.2",
		},
	}

	// Add pods to the fake clientset
	_, err := clientset.CoreV1().Pods("vault").Create(context.Background(), pod1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test pod: %v", err)
	}

	_, err = clientset.CoreV1().Pods("vault").Create(context.Background(), pod2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create test pod: %v", err)
	}

	// Test getVaultPods function
	pods, err := getVaultPods(clientset, "vault")
	if err != nil {
		t.Fatalf("failed to get vault pods: %v", err)
	}

	if len(pods) != 2 {
		t.Errorf("expected 2 pods, got %d", len(pods))
	}

	expectedIPs := map[string]bool{
		"10.0.0.1": true,
		"10.0.0.2": true,
	}

	for _, podIP := range pods {
		if !expectedIPs[podIP] {
			t.Errorf("unexpected pod IP: %s", podIP)
		}
	}
}

// mockKubernetesClient creates a mock Kubernetes client for testing
func mockKubernetesClient() kubernetes.Interface {
	return fake.NewSimpleClientset()
} 