package main

import (
	"fmt"
	"log"
	"time"

	"github.com/getgrowly/vault-utils/pkg/config"
	"github.com/getgrowly/vault-utils/pkg/kubernetes"
	"github.com/getgrowly/vault-utils/pkg/server"
	"github.com/getgrowly/vault-utils/pkg/vault"
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize Kubernetes client
	k8sClient, err := kubernetes.NewClient()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Start HTTP server
	srv := server.NewServer(k8sClient, "8080")
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// Main monitoring loop
	for {
		pods, err := k8sClient.GetVaultPods(cfg.VaultNamespace)
		if err != nil {
			log.Printf("Error getting Vault pods: %v", err)
			time.Sleep(cfg.CheckInterval)
			continue
		}

		log.Printf("Found %d Vault pods", len(pods))

		for _, podIP := range pods {
			vaultAddr := fmt.Sprintf("http://%s:%s", podIP, cfg.VaultPort)
			vaultClient := vault.NewClient(vaultAddr)

			status, err := vaultClient.CheckStatus()
			if err != nil {
				log.Printf("Error checking Vault status for %s: %v", vaultAddr, err)
				continue
			}

			if !status.Initialized {
				log.Printf("Vault pod %s is not initialized. Attempting initialization...", vaultAddr)
				initResp, err := vaultClient.Initialize()
				if err != nil {
					log.Printf("Error initializing Vault pod %s: %v", vaultAddr, err)
					continue
				}

				// Store keys in Kubernetes secrets
				if err := k8sClient.CreateUnsealKeySecret(cfg.VaultNamespace, initResp.Keys); err != nil {
					log.Printf("Error storing unseal keys: %v", err)
				}
				if err := k8sClient.CreateRootTokenSecret(cfg.VaultNamespace, initResp.RootToken); err != nil {
					log.Printf("Error storing root token: %v", err)
				}

				// Unseal with the first three keys
				for i := 0; i < 3; i++ {
					if err := vaultClient.UnsealWithKey(initResp.Keys[i]); err != nil {
						log.Printf("Error unsealing with key %d: %v", i+1, err)
						break
					}
				}
				continue
			}

			if status.Sealed {
				log.Printf("Vault pod %s is sealed. Attempting to unseal...", vaultAddr)
				// Try to get unseal keys from Kubernetes secret first
				secret, err := k8sClient.GetSecret(cfg.VaultNamespace, vault.UnsealKeysSecret)
				if err != nil {
					log.Printf("Error getting unseal keys secret: %v", err)
					// Fall back to environment variable keys
					if err := vaultClient.UnsealWithKeysFromDir(""); err != nil {
						log.Printf("Error unsealing Vault pod %s: %v", vaultAddr, err)
					}
					continue
				}

				// Use keys from Kubernetes secret
				for i := 1; i <= 3; i++ {
					key, ok := secret.Data[fmt.Sprintf("key%d", i)]
					if !ok {
						log.Printf("Key %d not found in secret", i)
						continue
					}
					if err := vaultClient.UnsealWithKey(string(key)); err != nil {
						log.Printf("Error applying key %d to pod %s: %v", i, vaultAddr, err)
						break
					}
					log.Printf("Successfully applied key %d to pod %s", i, vaultAddr)
				}
			} else {
				log.Printf("Vault pod %s is unsealed and healthy", vaultAddr)
			}
		}

		time.Sleep(cfg.CheckInterval)
	}
}
