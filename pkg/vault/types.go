package vault

// VaultStatus represents the health status of a Vault instance.
type VaultStatus struct {
	// Sealed indicates whether the Vault is currently sealed.
	// A sealed Vault cannot process any requests until unsealed.
	Sealed bool `json:"sealed"`

	// Initialized indicates whether the Vault has been initialized.
	// An uninitialized Vault needs to be initialized before it can be unsealed.
	Initialized bool `json:"initialized"`
}

// UnsealResponse represents the response from a Vault unseal operation.
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
	// UnsealKeysSecret is the name of the Kubernetes secret storing unseal keys
	UnsealKeysSecret = "vault-unseal-keys"
	// RootTokenSecret is the name of the Kubernetes secret storing the root token
	RootTokenSecret = "vault-root-token"
) 