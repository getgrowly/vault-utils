package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type VaultStatus struct {
	Sealed      bool `json:"sealed"`
	Initialized bool `json:"initialized"`
}

type UnsealResponse struct {
	Sealed bool `json:"sealed"`
}

func main() {
	// Configuration
	vaultService := os.Getenv("VAULT_SERVICE")
	vaultPort := os.Getenv("VAULT_PORT")
	
	// Parse check interval from environment variable
	checkIntervalStr := os.Getenv("CHECK_INTERVAL")
	checkInterval := 10 * time.Second // default value
	if checkIntervalStr != "" {
		if interval, err := strconv.Atoi(checkIntervalStr); err == nil {
			checkInterval = time.Duration(interval) * time.Second
		}
	}

	maxRetries := 5
	retryDelay := 2 * time.Second

	vaultAddr := fmt.Sprintf("http://%s:%s", vaultService, vaultPort)

	fmt.Printf("Starting Vault auto-unseal controller with check interval: %v\n", checkInterval)

	// Setup HTTP server for health checks
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		status, err := checkVaultStatus(vaultAddr)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if !status.Initialized {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	go http.ListenAndServe(":8080", nil)

	// Main monitoring loop
	for {
		status, err := checkVaultStatus(vaultAddr)
		if err != nil {
			fmt.Printf("Error checking Vault status: %v\n", err)
			time.Sleep(checkInterval)
			continue
		}

		if !status.Initialized {
			fmt.Println("Vault is not initialized. Waiting for initialization...")
			time.Sleep(checkInterval)
			continue
		}

		if status.Sealed {
			fmt.Println("Vault is sealed. Attempting to unseal...")
			if err := unsealVault(vaultAddr); err != nil {
				fmt.Printf("Error unsealing Vault: %v\n", err)
			} else {
				fmt.Println("Successfully unsealed Vault!")
			}
		}

		time.Sleep(checkInterval)
	}
}

func checkVaultStatus(vaultAddr string) (*VaultStatus, error) {
	resp, err := http.Get(fmt.Sprintf("%s/v1/sys/health", vaultAddr))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var status VaultStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

func unsealVault(vaultAddr string) error {
	// Read unseal keys
	keysDir := "/vault/unseal-keys"
	keys := make([]string, 3)
	for i := 1; i <= 3; i++ {
		keyPath := filepath.Join(keysDir, fmt.Sprintf("key%d", i))
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("error reading unseal key %d: %v", i, err)
		}
		keys[i-1] = string(key)
	}

	// Apply each key
	for i, key := range keys {
		resp, err := http.Post(
			fmt.Sprintf("%s/v1/sys/unseal", vaultAddr),
			"application/json",
			strings.NewReader(fmt.Sprintf(`{"key": "%s"}`, key)),
		)
		if err != nil {
			return fmt.Errorf("error applying unseal key %d: %v", i+1, err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading unseal response: %v", err)
		}

		var unsealResp UnsealResponse
		if err := json.Unmarshal(body, &unsealResp); err != nil {
			return fmt.Errorf("error parsing unseal response: %v", err)
		}

		if unsealResp.Sealed {
			fmt.Printf("Applied key %d, Vault still sealed\n", i+1)
		} else {
			fmt.Printf("Applied key %d, Vault unsealed\n", i+1)
		}
	}

	return nil
} 