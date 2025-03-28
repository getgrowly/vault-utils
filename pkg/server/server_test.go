package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getgrowly/vault-utils/pkg/kubernetes"
	"github.com/getgrowly/vault-utils/pkg/vault"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestHealthCheckEndpoints(t *testing.T) {
	// Create a fake Kubernetes clientset
	clientset := fake.NewSimpleClientset()

	// Create test pods
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vault-0",
			Namespace: "vault",
			Labels: map[string]string{
				"app.kubernetes.io/name": "vault",
				"component":              "server",
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
				"component":              "server",
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

	// Create a test HTTP server to mock Vault responses
	vaultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sys/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// Return a sealed status
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(vault.VaultStatus{
			Sealed:      true,
			Initialized: true,
		})
	}))
	defer vaultServer.Close()

	// Create Kubernetes client
	k8sClient := kubernetes.NewClientWithInterface(clientset)
	srv := NewServer(k8sClient, "8080")

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
			expectCode: http.StatusServiceUnavailable, // Vault pods exist but are sealed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.endpoint, nil)
			w := httptest.NewRecorder()

			switch tt.endpoint {
			case "/health":
				srv.handleHealth(w, req)
			case "/ready":
				srv.handleReady(w, req)
			}

			if w.Code != tt.expectCode {
				t.Errorf("expected status code %d, got %d", tt.expectCode, w.Code)
			}
		})
	}
}
