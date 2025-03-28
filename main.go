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
	cfg := config.LoadConfig()
	log.Printf("Starting Vault auto-unseal controller with config: namespace=%s, port=%s, interval=%v",
		cfg.VaultNamespace, cfg.VaultPort, cfg.CheckInterval)

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

	for {
		pods, err := k8sClient.GetVaultPods(cfg.VaultNamespace)
		if err != nil {
			log.Printf("Error getting Vault pods: %v", err)
			time.Sleep(cfg.CheckInterval)
			continue
		}

		if len(pods) == 0 {
			log.Printf("No Vault pods found in namespace %s", cfg.VaultNamespace)
			time.Sleep(cfg.CheckInterval)
			continue
		}

		// First, check if any Vault is initialized
		initialized := false
		var initResp *vault.InitResponse
		var initErr error

		for _, podIP := range pods {
			vaultAddr := fmt.Sprintf("http://%s:%s", podIP, cfg.VaultPort)
			vaultClient := vault.NewClient(vaultAddr)

			status, err := vaultClient.CheckStatus()
			if err != nil {
				log.Printf("Error checking Vault status for %s: %v", vaultAddr, err)
				continue
			}

			if status.Initialized {
				initialized = true
				break
			}
		}

		// If no Vault is initialized, initialize the first one
		if !initialized {
			log.Printf("No initialized Vault found. Initializing first Vault pod...")
			vaultAddr := fmt.Sprintf("http://%s:%s", pods[0], cfg.VaultPort)
			vaultClient := vault.NewClient(vaultAddr)

			initResp, initErr = vaultClient.Initialize()
			if initErr != nil {
				log.Printf("Error initializing Vault: %v", initErr)
				time.Sleep(cfg.CheckInterval)
				continue
			}

			// Store keys in Kubernetes secrets
			if err := k8sClient.CreateUnsealKeySecret(cfg.VaultNamespace, initResp.Keys); err != nil {
				log.Printf("Error storing unseal keys: %v", err)
				time.Sleep(cfg.CheckInterval)
				continue
			}
			if err := k8sClient.CreateRootTokenSecret(cfg.VaultNamespace, initResp.RootToken); err != nil {
				log.Printf("Error storing root token: %v", err)
				time.Sleep(cfg.CheckInterval)
				continue
			}

			log.Printf("Successfully initialized Vault and stored secrets")
		}

		// Get the secrets for unsealing
		unsealSecret, err := k8sClient.GetSecret(cfg.VaultNamespace, vault.UnsealKeysSecret)
		if err != nil {
			log.Printf("Error getting unseal keys secret: %v", err)
			time.Sleep(cfg.CheckInterval)
			continue
		}

		// Try to unseal all Vaults
		for _, podIP := range pods {
			vaultAddr := fmt.Sprintf("http://%s:%s", podIP, cfg.VaultPort)
			vaultClient := vault.NewClient(vaultAddr)

			status, err := vaultClient.CheckStatus()
			if err != nil {
				log.Printf("Error checking Vault status for %s: %v", vaultAddr, err)
				continue
			}

			if status.Sealed {
				log.Printf("Vault pod %s is sealed. Attempting to unseal...", vaultAddr)
				
				// Use keys from Kubernetes secret
				for i := 1; i <= 3; i++ {
					key, ok := unsealSecret.Data[fmt.Sprintf("key%d", i)]
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
