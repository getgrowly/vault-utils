package vault

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	defaultSecretShares    = 5
	defaultSecretThreshold = 3
)

// Client represents a Vault client for managing Vault operations
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new Vault client
func NewClient(baseURL string) *Client {
	return &Client{
		httpClient: &http.Client{},
		baseURL:    baseURL,
	}
}

// CheckStatus queries the Vault health endpoint
func (c *Client) CheckStatus() (*Status, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/v1/sys/seal-status", c.baseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to check status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var status Status
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &status, nil
}

// Initialize initializes a new Vault instance
func (c *Client) Initialize() (*InitResponse, error) {
	req := InitRequest{
		SecretShares:    defaultSecretShares,
		SecretThreshold: defaultSecretThreshold,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/v1/sys/init", c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var initResp InitResponse
	if err := json.NewDecoder(resp.Body).Decode(&initResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &initResp, nil
}

// UnsealWithKey applies a single unseal key to the Vault
func (c *Client) UnsealWithKey(key string) error {
	req := map[string]string{"key": key}
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(fmt.Sprintf("%s/v1/sys/unseal", c.baseURL), "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to unseal: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var unsealResp UnsealResponse
	if err := json.NewDecoder(resp.Body).Decode(&unsealResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// If the vault is still sealed, this is not an error - it just means we need more keys
	if unsealResp.Sealed {
		return nil
	}

	return nil
}

// UnsealWithKeysFromDir unseals Vault using keys from a directory
func (c *Client) UnsealWithKeysFromDir(keys []string) error {
	for _, key := range keys {
		if err := c.UnsealWithKey(key); err != nil {
			return fmt.Errorf("failed to unseal with key: %w", err)
		}
	}
	return nil
}
