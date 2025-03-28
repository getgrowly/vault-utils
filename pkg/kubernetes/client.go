package kubernetes

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client represents a Kubernetes client for managing Kubernetes operations
type Client struct {
	clientset kubernetes.Interface
}

// NewClient creates a new Kubernetes client using in-cluster configuration or local kubeconfig
func NewClient() (*Client, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			if home := os.Getenv("HOME"); home != "" {
				kubeconfig = filepath.Join(home, ".kube", "config")
			}
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to get kubeconfig: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	return &Client{clientset: clientset}, nil
}

// NewClientWithInterface creates a new Kubernetes client with a provided interface
func NewClientWithInterface(clientset kubernetes.Interface) *Client {
	return &Client{clientset: clientset}
}

// GetVaultPods returns a list of all Vault pods in the specified namespace
func (c *Client) GetVaultPods(namespace string) ([]string, error) {
	pods, err := c.clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=vault,component=server",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list Vault pods: %v", err)
	}

	var podAddresses []string

	for _, pod := range pods.Items {
		if pod.Status.PodIP != "" {
			log.Printf("Found Vault pod %s with IP %s", pod.Name, pod.Status.PodIP)
			podAddresses = append(podAddresses, pod.Status.PodIP)
		}
	}

	return podAddresses, nil
}

// CreateSecret creates a new Kubernetes secret
func (c *Client) CreateSecret(secret *corev1.Secret) error {
	_, err := c.clientset.CoreV1().Secrets(secret.Namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create secret %s: %v", secret.Name, err)
	}

	return nil
}

// GetSecret retrieves a Kubernetes secret
func (c *Client) GetSecret(namespace, name string) (*corev1.Secret, error) {
	secret, err := c.clientset.CoreV1().Secrets(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %v", name, err)
	}

	return secret, nil
}

// CreateUnsealKeySecret creates a secret containing Vault unseal keys
func (c *Client) CreateUnsealKeySecret(namespace string, keys []string) error {
	unsealKeysData := make(map[string][]byte)
	for i, key := range keys {
		unsealKeysData[fmt.Sprintf("key%d", i+1)] = []byte(key)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vault-unseal-keys",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/component":     "vault-secrets",
				"vault.hashicorp.com/secret-type": "unseal-keys",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: unsealKeysData,
	}

	return c.CreateSecret(secret)
}

// CreateRootTokenSecret creates a secret containing the Vault root token
func (c *Client) CreateRootTokenSecret(namespace, rootToken string) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vault-root-token",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/component":     "vault-secrets",
				"vault.hashicorp.com/secret-type": "root-token",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"token": []byte(rootToken),
		},
	}

	return c.CreateSecret(secret)
}
