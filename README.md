# Vault Auto-Unseal Controller

A lightweight Go-based controller that automatically unseals HashiCorp Vault instances. This controller monitors the Vault's status and automatically unseals it when necessary.

## Features

- Automatic Vault status monitoring
- Configurable check interval
- Health check endpoints (/health and /ready)
- Docker containerization
- Support for multiple unseal keys

## Prerequisites

- Docker
- HashiCorp Vault instance
- Unseal keys stored in `/vault/unseal-keys/` directory

## Configuration

The controller can be configured using the following environment variables:

- `VAULT_SERVICE`: The hostname or service name of the Vault instance
- `VAULT_PORT`: The port number of the Vault instance
- `CHECK_INTERVAL`: The interval (in seconds) between status checks (default: 10 seconds)

## Docker Images

Docker images are automatically built and pushed to GitHub Container Registry (ghcr.io) for each release and main branch.

### Using the Docker Image

```bash
docker pull ghcr.io/getgrowly/vault-utils:latest
```

### Available Tags

- `latest`: Latest stable version
- `main`: Latest main branch build
- `vX.Y.Z`: Specific version tags
- `vX.Y`: Minor version tags
- `sha-<commit-sha>`: Specific commit builds

### Running with Docker

```bash
docker run -d \
  -e VAULT_SERVICE=vault \
  -e VAULT_PORT=8200 \
  -e CHECK_INTERVAL=10 \
  -v /path/to/unseal-keys:/vault/unseal-keys \
  -p 8080:8080 \
  ghcr.io/getgrowly/vault-utils:latest
```

### Building Locally

```bash
docker build -t vault-utils .
```

## Usage

### Building the Docker Image

```bash
docker build -t vault-auto-unseal .
```

### Running the Container

```bash
docker run -d \
  -e VAULT_SERVICE=vault \
  -e VAULT_PORT=8200 \
  -e CHECK_INTERVAL=10 \
  -v /path/to/unseal-keys:/vault/unseal-keys \
  -p 8080:8080 \
  vault-auto-unseal
```

### Health Check Endpoints

- `/health`: Returns 200 OK if the service is running
- `/ready`: Returns 200 OK if Vault is initialized and unsealed

## Unseal Keys

The controller expects three unseal keys to be present in the `/vault/unseal-keys/` directory:
- `key1`
- `key2`
- `key3`

Each key should contain the corresponding unseal key for your Vault instance.

## Security Considerations

- Ensure unseal keys are stored securely and have appropriate permissions
- Use Docker secrets or Kubernetes secrets for production deployments
- Consider using Vault's auto-unseal feature with cloud KMS for production environments

## Development

The project is written in Go and uses the following structure:

```
.
├── Dockerfile
├── auto-unseal-controller.go
└── README.md
```

## License

This project is open source and available under the MIT License.

## Contributing

We welcome contributions! Please follow these steps:

1. Fork the repository
2. Create a new branch for your feature (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests if applicable
5. Commit your changes (`git commit -m 'Add some amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### Development Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/vault-utils.git
   cd vault-utils
   ```

2. Build the project:
   ```bash
   go build -o auto-unseal auto-unseal-controller.go
   ```

3. Run tests:
   ```bash
   go test -v ./...
   ```

### Testing

The project includes comprehensive tests covering:
- Vault status checking
- Unseal process
- Health check endpoints
- Main loop functionality

#### Test Coverage

To generate a test coverage report:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

#### Test Environment

Tests use mock HTTP servers to simulate Vault responses and temporary directories for unseal keys. No actual Vault instance is required to run the tests.

### Code Style

- Follow Go standard formatting (`go fmt`)
- Follow Go best practices and idioms
- Add comments for complex logic
- Update documentation as needed

### Pull Request Process

1. Update the README.md with details of changes if needed
2. Update the documentation if you're changing functionality
3. The PR will be merged once you have the sign-off of at least one other developer

### Reporting Bugs

Before creating bug reports, please check the issue list as you might find out that you don't need to create one. When you are creating a bug report, please include as many details as possible:

* Use a clear and descriptive title
* Describe the exact steps which reproduce the problem
* Provide specific examples to demonstrate the steps
* Describe the behavior you observed after following the steps
* Explain which behavior you expected to see instead and why
* Include screenshots and animated GIFs if possible
* Include any relevant code snippets 