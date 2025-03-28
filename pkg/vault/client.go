package vault

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Client represents a Vault client for managing Vault operations
type Client struct {
	addr string
}

// NewClient creates a new Vault client
func NewClient(addr string) *Client {
	return &Client{addr: addr}
}

// CheckStatus queries the Vault health endpoint
func (c *Client) CheckStatus() (*VaultStatus, error) {
	log.Printf("Checking Vault status at %s", c.addr)
	resp, err := http.Get(fmt.Sprintf("%s/v1/sys/health", c.addr))
	if err != nil {
		return nil, fmt.Errorf("failed to get Vault health status: %v", err)
	}
	defer resp.Body.Close()

	// Valid status codes:
	// 200 - initialized, unsealed, and active
	// 429 - unsealed and standby
	// 472 - disaster recovery mode replication secondary and active
	// 473 - performance standby
	// 501 - not initialized
	// 503 - sealed
	validCodes := map[int]bool{
		http.StatusOK:                 true, // 200
		http.StatusTooManyRequests:    true, // 429
		http.StatusNotImplemented:     true, // 501
		http.StatusServiceUnavailable: true, // 503
		472:                           true,
		473:                           true,
	}

	if !validCodes[resp.StatusCode] {
		return nil, fmt.Errorf("vault health check failed with unexpected status: %d", resp.StatusCode)
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

// Initialize initializes a new Vault instance
func (c *Client) Initialize() (*InitResponse, error) {
	log.Printf("Initializing Vault at %s", c.addr)

	initReq := InitRequest{
		SecretShares:    5,
		SecretThreshold: 3,
	}

	reqBody, err := json.Marshal(initReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal init request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/v1/sys/init", c.addr), strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create init request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Vault: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault initialization failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read init response: %v", err)
	}

	var initResp InitResponse
	if err := json.Unmarshal(body, &initResp); err != nil {
		return nil, fmt.Errorf("failed to parse init response: %v", err)
	}

	return &initResp, nil
}

// UnsealWithKey applies a single unseal key to the Vault
func (c *Client) UnsealWithKey(key string) error {
	resp, err := http.Post(
		fmt.Sprintf("%s/v1/sys/unseal", c.addr),
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

// UnsealWithKeysFromDir unseals Vault using keys from a directory
func (c *Client) UnsealWithKeysFromDir(keysDir string) error {
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
			fmt.Sprintf("%s/v1/sys/unseal", c.addr),
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
