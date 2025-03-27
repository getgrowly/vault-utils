package kubernetes

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetVaultPods(t *testing.T) {
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

	// Create client with fake clientset
	client := NewClientWithInterface(clientset)

	// Test GetVaultPods function
	pods, err := client.GetVaultPods("vault")
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

func TestCreateAndGetSecret(t *testing.T) {
	// Create a fake Kubernetes clientset
	clientset := fake.NewSimpleClientset()
	client := NewClientWithInterface(clientset)

	// Test creating unseal key secret
	keys := []string{"key1", "key2", "key3"}
	err := client.CreateUnsealKeySecret("vault", keys)
	if err != nil {
		t.Fatalf("failed to create unseal key secret: %v", err)
	}

	// Test getting the created secret
	secret, err := client.GetSecret("vault", "vault-unseal-keys")
	if err != nil {
		t.Fatalf("failed to get secret: %v", err)
	}

	// Verify secret data
	for i, key := range keys {
		secretKey := string(secret.Data[string(key)])
		if secretKey != key {
			t.Errorf("expected key %d to be %s, got %s", i+1, key, secretKey)
		}
	}

	// Test creating root token secret
	rootToken := "test-root-token"
	err = client.CreateRootTokenSecret("vault", rootToken)
	if err != nil {
		t.Fatalf("failed to create root token secret: %v", err)
	}

	// Test getting the created root token secret
	secret, err = client.GetSecret("vault", "vault-root-token")
	if err != nil {
		t.Fatalf("failed to get root token secret: %v", err)
	}

	// Verify root token
	if string(secret.Data["token"]) != rootToken {
		t.Errorf("expected root token to be %s, got %s", rootToken, string(secret.Data["token"]))
	}
}
