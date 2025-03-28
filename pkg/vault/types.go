package vault

const (
	RootTokenSecret  = "vault-root-token"
	UnsealKeysSecret = "vault-unseal-keys"
)

// Status represents the current status of a Vault instance
type Status struct {
	Initialized bool `json:"initialized"`
	Sealed      bool `json:"sealed"`
}

// InitRequest represents a request to initialize a new Vault instance
type InitRequest struct {
	SecretShares    int `json:"secret_shares"`
	SecretThreshold int `json:"secret_threshold"`
}

// InitResponse represents the response from initializing a new Vault instance
type InitResponse struct {
	RootToken string   `json:"root_token"`
	Keys      []string `json:"keys"`
}

// UnsealResponse represents the response from unsealing a Vault instance
type UnsealResponse struct {
	Sealed bool `json:"sealed"`
}

// VaultStatus represents the health status of a Vault instance.
type VaultStatus struct {
	// Sealed indicates whether the Vault is currently sealed.
	// A sealed Vault cannot process any requests until unsealed.
	Sealed bool `json:"sealed"`

	// Initialized indicates whether the Vault has been initialized.
	// An uninitialized Vault needs to be initialized before it can be unsealed.
	Initialized bool `json:"initialized"`
}
