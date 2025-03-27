// Package main implements a Vault auto-unseal controller that automatically unseals
// HashiCorp Vault instances by monitoring their health status and applying unseal keys
// when necessary. The controller provides health and readiness endpoints for Kubernetes
// integration and supports configurable check intervals.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// VaultStatus represents the health status of a Vault instance.
// It contains information about whether the Vault is sealed and initialized.
type VaultStatus struct {
	// Sealed indicates whether the Vault is currently sealed.
	// A sealed Vault cannot process any requests until unsealed.
	Sealed bool `json:"sealed"`

	// Initialized indicates whether the Vault has been initialized.
	// An uninitialized Vault needs to be initialized before it can be unsealed.
	Initialized bool `json:"initialized"`
}

// UnsealResponse represents the response from a Vault unseal operation.
// It indicates whether the Vault remains sealed after applying an unseal key.
type UnsealResponse struct {
	// Sealed indicates whether the Vault is still sealed after the unseal operation.
	// This will be false only after all required unseal keys have been applied.
	Sealed bool `json:"sealed"`
}

// InitRequest represents the request to initialize Vault
type InitRequest struct {
	SecretShares    int `json:"secret_shares"`
	SecretThreshold int `json:"secret_threshold"`
}

// InitResponse represents the response from Vault initialization
type InitResponse struct {
	Keys       []string `json:"keys"`
	RootToken  string   `json:"root_token"`
	KeysBase64 []string `json:"keys_base64"`
}

const (
	unsealKeysSecret = "vault-unseal-keys"
	rootTokenSecret  = "vault-root-token"
)

// init configures the logging format to include timestamps and file locations
// for better debugging and monitoring capabilities.
func init() {
	// Configure log format to include timestamp and file location
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

// initializeVault initializes a new Vault instance and stores the keys in Kubernetes secrets
func initializeVault(clientset *kubernetes.Clientset, namespace, vaultAddr string) error {
	log.Printf("Initializing Vault at %s", vaultAddr)

	// Create initialization request
	initReq := InitRequest{
		SecretShares:    5,
		SecretThreshold: 3,
	}

	reqBody, err := json.Marshal(initReq)
	if err != nil {
		return fmt.Errorf("failed to marshal init request: %v", err)
	}

	// Send initialization request
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/v1/sys/init", vaultAddr), strings.NewReader(string(reqBody)))
	if err != nil {
		return fmt.Errorf("failed to create init request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to initialize Vault: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vault initialization failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read init response: %v", err)
	}

	var initResp InitResponse
	if err := json.Unmarshal(body, &initResp); err != nil {
		return fmt.Errorf("failed to parse init response: %v", err)
	}

	// Create Kubernetes secret for unseal keys
	unsealKeysData := make(map[string][]byte)
	for i, key := range initResp.Keys {
		unsealKeysData[fmt.Sprintf("key%d", i+1)] = []byte(key)
	}

	unsealSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      unsealKeysSecret,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/component":     "vault-secrets",
				"vault.hashicorp.com/secret-type": "unseal-keys",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: unsealKeysData,
	}

	// Create Kubernetes secret for root token
	rootTokenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rootTokenSecret,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/component":     "vault-secrets",
				"vault.hashicorp.com/secret-type": "root-token",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"token": []byte(initResp.RootToken),
		},
	}

	// Create or update the secrets
	_, err = clientset.CoreV1().Secrets(namespace).Create(context.Background(), unsealSecret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create unseal keys secret: %v", err)
	}

	_, err = clientset.CoreV1().Secrets(namespace).Create(context.Background(), rootTokenSecret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create root token secret: %v", err)
	}

	log.Printf("Successfully initialized Vault and stored secrets")

	// Unseal Vault using the first three keys
	for i := 0; i < 3; i++ {
		if err := unsealWithKey(vaultAddr, initResp.Keys[i]); err != nil {
			return fmt.Errorf("failed to unseal with key %d: %v", i+1, err)
		}
	}

	return nil
}

// unsealWithKey applies a single unseal key to the Vault
func unsealWithKey(vaultAddr, key string) error {
	resp, err := http.Post(
		fmt.Sprintf("%s/v1/sys/unseal", vaultAddr),
		"application/json",
		strings.NewReader(fmt.Sprintf(`{"key": "%s"}`, key)),
	)
	if err != nil {
		return fmt.Errorf("failed to apply unseal key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unseal request failed with status: %d", resp.StatusCode)
	}

	return nil
}

// getKubernetesClient creates a Kubernetes client using in-cluster configuration
func getKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	return clientset, nil
}

// getVaultPods returns a list of all Vault pods in the specified namespace
func getVaultPods(clientset kubernetes.Interface, namespace string) ([]string, error) {
	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=vault",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list Vault pods: %v", err)
	}

	var podAddresses []string
	for _, pod := range pods.Items {
		podIP := pod.Status.PodIP
		if podIP != "" {
			podAddresses = append(podAddresses, podIP)
		}
	}

	return podAddresses, nil
}

// main is the entry point of the Vault auto-unseal controller.
// It performs the following operations:
// 1. Loads configuration from environment variables
// 2. Sets up HTTP endpoints for health and readiness checks
// 3. Starts a monitoring loop to check Vault status and perform unseal operations
func main() {
	// Configuration
	vaultNamespace := os.Getenv("VAULT_NAMESPACE")
	if vaultNamespace == "" {
		vaultNamespace = "vault"
	}
	vaultPort := os.Getenv("VAULT_PORT")
	if vaultPort == "" {
		vaultPort = "8200"
	}

	// Parse check interval from environment variable
	checkIntervalStr := os.Getenv("CHECK_INTERVAL")
	checkInterval := 10 * time.Second // default value
	if checkIntervalStr != "" {
		if interval, err := strconv.Atoi(checkIntervalStr); err == nil {
			checkInterval = time.Duration(interval) * time.Second
		}
	}

	// Initialize Kubernetes client
	clientset, err := getKubernetesClient()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	log.Printf("Starting Vault auto-unseal controller with configuration:")
	log.Printf("- Vault Namespace: %s", vaultNamespace)
	log.Printf("- Vault Port: %s", vaultPort)
	log.Printf("- Check Interval: %v", checkInterval)

	// Setup HTTP server for health checks
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Health check request received from %s", r.RemoteAddr)
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Readiness check request received from %s", r.RemoteAddr)
		pods, err := getVaultPods(clientset, vaultNamespace)
		if err != nil {
			log.Printf("Failed to get Vault pods: %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		allReady := true
		for _, podIP := range pods {
			vaultAddr := fmt.Sprintf("http://%s:%s", podIP, vaultPort)
			status, err := checkVaultStatus(vaultAddr)
			if err != nil || !status.Initialized || status.Sealed {
				allReady = false
				break
			}
		}

		if allReady {
			log.Printf("All Vault pods are ready")
			w.WriteHeader(http.StatusOK)
		} else {
			log.Printf("Some Vault pods are not ready")
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})

	// Start HTTP server in a goroutine
	go func() {
		log.Printf("Starting HTTP server on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// Main monitoring loop
	for {
		pods, err := getVaultPods(clientset, vaultNamespace)
		if err != nil {
			log.Printf("Error getting Vault pods: %v", err)
			time.Sleep(checkInterval)
			continue
		}

		log.Printf("Found %d Vault pods", len(pods))

		for _, podIP := range pods {
			vaultAddr := fmt.Sprintf("http://%s:%s", podIP, vaultPort)
			log.Printf("Checking Vault pod at %s", vaultAddr)

			status, err := checkVaultStatus(vaultAddr)
			if err != nil {
				log.Printf("Error checking Vault status for %s: %v", vaultAddr, err)
				continue
			}

			if !status.Initialized {
				log.Printf("Vault pod %s is not initialized. Attempting initialization...", vaultAddr)
				if err := initializeVault(clientset, vaultNamespace, vaultAddr); err != nil {
					log.Printf("Error initializing Vault pod %s: %v", vaultAddr, err)
					continue
				}
				log.Printf("Successfully initialized Vault pod %s", vaultAddr)
				continue
			}

			if status.Sealed {
				log.Printf("Vault pod %s is sealed. Attempting to unseal...", vaultAddr)
				// Try to get unseal keys from Kubernetes secret first
				secret, err := clientset.CoreV1().Secrets(vaultNamespace).Get(context.Background(), unsealKeysSecret, metav1.GetOptions{})
				if err != nil {
					log.Printf("Error getting unseal keys secret: %v", err)
					// Fall back to environment variable keys
					if err := unsealVault(vaultAddr); err != nil {
						log.Printf("Error unsealing Vault pod %s: %v", vaultAddr, err)
					} else {
						log.Printf("Successfully unsealed Vault pod %s!", vaultAddr)
					}
					continue
				}

				// Use keys from Kubernetes secret
				var keys []string
				for i := 1; i <= 3; i++ {
					key, ok := secret.Data[fmt.Sprintf("key%d", i)]
					if !ok {
						log.Printf("Key %d not found in secret", i)
						continue
					}
					keys = append(keys, string(key))
				}

				// Apply each key
				for i, key := range keys {
					if err := unsealWithKey(vaultAddr, key); err != nil {
						log.Printf("Error applying key %d to pod %s: %v", i+1, vaultAddr, err)
						break
					}
					log.Printf("Successfully applied key %d to pod %s", i+1, vaultAddr)
				}
			} else {
				log.Printf("Vault pod %s is unsealed and healthy", vaultAddr)
			}
		}

		time.Sleep(checkInterval)
	}
}

// checkVaultStatus queries the Vault health endpoint to determine the current
// status of the Vault instance. It returns a VaultStatus struct containing
// information about whether the Vault is sealed and initialized.
//
// Parameters:
//   - vaultAddr: The HTTP address of the Vault instance
//
// Returns:
//   - *VaultStatus: The current status of the Vault
//   - error: Any error that occurred during the health check
func checkVaultStatus(vaultAddr string) (*VaultStatus, error) {
	log.Printf("Checking Vault status at %s", vaultAddr)
	resp, err := http.Get(fmt.Sprintf("%s/v1/sys/health", vaultAddr))
	if err != nil {
		return nil, fmt.Errorf("failed to get Vault health status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault health check failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Vault health response: %v", err)
	}

	var status VaultStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("failed to parse Vault health response: %v", err)
	}

	log.Printf("Vault status: initialized=%v, sealed=%v", status.Initialized, status.Sealed)
	return &status, nil
}

// unsealVault attempts to unseal the Vault instance by reading unseal keys from
// the configured directory and applying them sequentially. The function expects
// three unseal keys to be present in the keys directory.
//
// Parameters:
//   - vaultAddr: The HTTP address of the Vault instance
//
// Returns:
//   - error: Any error that occurred during the unseal process
//
// Environment Variables:
//   - VAULT_UNSEAL_KEYS_DIR: Directory containing the unseal keys (default: "/vault/unseal-keys")
func unsealVault(vaultAddr string) error {
	// Get keys directory from environment or use default
	keysDir := os.Getenv("VAULT_UNSEAL_KEYS_DIR")
	if keysDir == "" {
		keysDir = "/vault/unseal-keys"
	}
	log.Printf("Using unseal keys directory: %s", keysDir)

	// Read unseal keys
	keys := make([]string, 3)
	for i := 1; i <= 3; i++ {
		keyPath := filepath.Join(keysDir, fmt.Sprintf("key%d", i))
		log.Printf("Reading unseal key from: %s", keyPath)
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("error reading unseal key %d: %v", i, err)
		}
		keys[i-1] = string(key)
	}

	// Apply each key
	for i, key := range keys {
		log.Printf("Applying unseal key %d/3", i+1)
		resp, err := http.Post(
			fmt.Sprintf("%s/v1/sys/unseal", vaultAddr),
			"application/json",
			strings.NewReader(fmt.Sprintf(`{"key": "%s"}`, key)),
		)
		if err != nil {
			return fmt.Errorf("error applying unseal key %d: %v", i+1, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("vault unseal failed with status: %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading unseal response: %v", err)
		}

		var unsealResp UnsealResponse
		if err := json.Unmarshal(body, &unsealResp); err != nil {
			return fmt.Errorf("error parsing unseal response: %v", err)
		}

		if unsealResp.Sealed {
			log.Printf("Applied key %d/3, Vault still sealed", i+1)
		} else {
			log.Printf("Applied key %d/3, Vault unsealed successfully", i+1)
		}
	}

	return nil
} 