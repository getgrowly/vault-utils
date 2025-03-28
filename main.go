package main

import (
	"fmt"
	"log"
	"time"

	"github.com/getgrowly/vault-utils/pkg/config"
	"github.com/getgrowly/vault-utils/pkg/kubernetes"
	"github.com/getgrowly/vault-utils/pkg/server"
	"github.com/getgrowly/vault-utils/pkg/vault"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func initializeVault(vaultClient *vault.Client, kubeClient *kubernetes.Client, config *config.Config) error {
	resp, err := vaultClient.Initialize()
	if err != nil {
		return fmt.Errorf("error initializing Vault: %v", err)
	}

	rootTokenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vault.RootTokenSecret,
			Namespace: config.VaultNamespace,
		},
		Data: map[string][]byte{
			"token": []byte(resp.RootToken),
		},
	}

	if err := kubeClient.CreateSecret(rootTokenSecret); err != nil {
		return fmt.Errorf("error storing root token: %v", err)
	}

	unsealKeys := make(map[string][]byte)
	for i, key := range resp.Keys {
		unsealKeys[fmt.Sprintf("key%d", i+1)] = []byte(key)
	}

	unsealKeysSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vault.UnsealKeysSecret,
			Namespace: config.VaultNamespace,
		},
		Data: unsealKeys,
	}

	if err := kubeClient.CreateSecret(unsealKeysSecret); err != nil {
		return fmt.Errorf("error storing unseal keys: %v", err)
	}

	log.Printf("Successfully initialized Vault and stored secrets")

	return nil
}

func unsealVault(vaultClient *vault.Client, kubeClient *kubernetes.Client, config *config.Config) error {
	unsealSecret, err := kubeClient.GetSecret(config.VaultNamespace, vault.UnsealKeysSecret)
	if err != nil {
		return fmt.Errorf("error getting unseal keys secret: %v", err)
	}

	for key := range unsealSecret.Data {
		if err := vaultClient.UnsealWithKey(string(unsealSecret.Data[key])); err != nil {
			return fmt.Errorf("error unsealing Vault with key %s: %v", key, err)
		}
	}

	return nil
}

func main() {
	cfg := config.LoadConfig()
	log.Printf("Starting Vault auto-unseal controller with config: namespace=%s, port=%s, interval=%v",
		cfg.VaultNamespace, cfg.VaultPort, cfg.CheckInterval)

	k8sClient, err := kubernetes.NewClient()
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

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

			continue
		}

		if len(pods) == 0 {
			log.Printf("No Vault pods found")

			continue
		}

		for _, pod := range pods {
			vaultAddr := fmt.Sprintf("http://%s:%s", pod, cfg.VaultPort)
			vaultClient := vault.NewClient(vaultAddr)

			status, err := vaultClient.CheckStatus()
			if err != nil {
				log.Printf("Error checking Vault status for pod %s: %v", pod, err)

				continue
			}

			if !status.Initialized {
				if err := initializeVault(vaultClient, k8sClient, cfg); err != nil {
					log.Printf("Error initializing Vault for pod %s: %v", pod, err)

					continue
				}
			}

			if status.Sealed {
				if err := unsealVault(vaultClient, k8sClient, cfg); err != nil {
					log.Printf("Error unsealing Vault for pod %s: %v", pod, err)

					continue
				}
			}
		}

		time.Sleep(cfg.CheckInterval)
	}
}
