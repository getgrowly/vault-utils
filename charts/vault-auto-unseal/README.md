# Vault Auto-Unseal Helm Chart

A Helm chart that deploys both **HashiCorp Vault** and the **Vault Auto-Unseal Controller** on Kubernetes. The controller automatically monitors, initializes, and unseals Vault instances -- so you get a fully operational Vault with a single `helm install`.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   Kubernetes Cluster                │
│                                                     │
│  ┌───────────────────┐    ┌──────────────────────┐  │
│  │  Vault Server      │    │  Auto-Unseal         │  │
│  │  (sub-chart)       │◄───│  Controller          │  │
│  │                    │    │                      │  │
│  │  - Standalone/HA   │    │  - Monitors pods     │  │
│  │  - Port 8200       │    │  - Initializes Vault │  │
│  │  - File/Raft store │    │  - Stores secrets    │  │
│  └───────────────────┘    │  - Unseals Vault     │  │
│                            │  - Health: :8080     │  │
│  ┌───────────────────┐    └──────────────────────┘  │
│  │  K8s Secrets       │                              │
│  │  - vault-root-token│                              │
│  │  - vault-unseal-   │                              │
│  │    keys             │                              │
│  └───────────────────┘                              │
└─────────────────────────────────────────────────────┘
```

## Prerequisites

- Kubernetes 1.19+
- Helm 3.2.0+

## Quick Start

### Install with Vault included (default)

```bash
# Update dependencies first
helm dependency update ./charts/vault-auto-unseal

# Install the chart
helm install vault-stack ./charts/vault-auto-unseal \
  --namespace vault \
  --create-namespace
```

This deploys Vault in standalone mode along with the auto-unseal controller. The controller will automatically initialize and unseal Vault once it's ready.

### Install controller only (external Vault)

If you already have Vault running, disable the sub-chart:

```bash
helm install vault-auto-unseal ./charts/vault-auto-unseal \
  --namespace vault \
  --set vault.enabled=false \
  --set controller.vaultNamespace=vault
```

### Install with custom values file

```bash
helm install vault-stack ./charts/vault-auto-unseal \
  --namespace vault \
  --create-namespace \
  -f my-values.yaml
```

## How It Works

1. **Vault deploys** as a sub-chart (standalone or HA mode)
2. **Controller discovers** Vault pods using label selectors (`app.kubernetes.io/name=vault,component=server`)
3. **Controller checks status** of each Vault pod via the `/v1/sys/seal-status` API
4. **Initializes** uninitialized Vault instances (creates 5 key shares, threshold of 3)
5. **Stores secrets** as Kubernetes secrets:
   - `vault-root-token` -- Contains the Vault root token
   - `vault-unseal-keys` -- Contains unseal keys (`key1` through `key5`)
6. **Unseals** sealed Vault instances using the stored unseal keys

## Configuration

### Controller Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.vaultNamespace` | Namespace where Vault pods run | `vault` |
| `controller.vaultPort` | Port where Vault API is listening | `8200` |
| `controller.checkInterval` | Seconds between status checks | `10` |

### Image Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Controller image repository | `ghcr.io/getgrowly/vault-utils` |
| `image.tag` | Image tag (defaults to chart `appVersion`) | `""` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `imagePullSecrets` | Pull secrets for private registries | `[]` |

### Deployment Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of controller replicas | `1` |
| `podAnnotations` | Annotations for controller pods | `{}` |
| `podLabels` | Labels for controller pods | `{}` |
| `nodeSelector` | Node selector constraints | `{}` |
| `tolerations` | Tolerations for pod scheduling | `[]` |
| `affinity` | Affinity rules for pod scheduling | `{}` |

### RBAC Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceAccount.create` | Create a ServiceAccount | `true` |
| `serviceAccount.annotations` | ServiceAccount annotations | `{}` |
| `serviceAccount.name` | ServiceAccount name (auto-generated if empty) | `""` |
| `rbac.create` | Create Role and RoleBinding | `true` |

### Security Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `podSecurityContext.runAsNonRoot` | Run pod as non-root | `true` |
| `podSecurityContext.runAsUser` | UID for pod user | `10001` |
| `podSecurityContext.fsGroup` | Group ID for filesystem | `10001` |
| `securityContext.allowPrivilegeEscalation` | Allow privilege escalation | `false` |
| `securityContext.readOnlyRootFilesystem` | Read-only root filesystem | `true` |
| `securityContext.capabilities.drop` | Linux capabilities to drop | `["ALL"]` |

### Service Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Kubernetes service type | `ClusterIP` |
| `service.port` | Health check service port | `8080` |

### Resource Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.limits.cpu` | CPU limit | `100m` |
| `resources.limits.memory` | Memory limit | `128Mi` |
| `resources.requests.cpu` | CPU request | `50m` |
| `resources.requests.memory` | Memory request | `64Mi` |

### Probe Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `livenessProbe.httpGet.path` | Liveness probe endpoint | `/health` |
| `livenessProbe.initialDelaySeconds` | Delay before first check | `5` |
| `livenessProbe.periodSeconds` | Interval between checks | `10` |
| `readinessProbe.httpGet.path` | Readiness probe endpoint | `/ready` |
| `readinessProbe.initialDelaySeconds` | Delay before first check | `5` |
| `readinessProbe.periodSeconds` | Interval between checks | `10` |

### Vault Sub-chart Settings

The Vault dependency uses the [official HashiCorp Vault Helm chart](https://developer.hashicorp.com/vault/docs/platform/k8s/helm/configuration). All values under the `vault:` key are passed directly to the sub-chart.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `vault.enabled` | Deploy HashiCorp Vault as a dependency | `true` |
| `vault.server.replicas` | Number of Vault server replicas | `1` |
| `vault.server.image.repository` | Vault server image | `hashicorp/vault` |
| `vault.server.image.tag` | Vault server image tag | `1.15.4` |
| `vault.server.resources.requests.memory` | Vault memory request | `256Mi` |
| `vault.server.resources.requests.cpu` | Vault CPU request | `250m` |
| `vault.server.dataStorage.enabled` | Enable persistent storage | `true` |
| `vault.server.dataStorage.size` | Storage volume size | `10Gi` |
| `vault.server.standalone.enabled` | Enable standalone mode | `true` |
| `vault.server.ha.enabled` | Enable HA mode | `false` |
| `vault.server.ha.replicas` | HA replicas | `3` |
| `vault.ui.enabled` | Enable Vault UI | `false` |
| `vault.injector.enabled` | Enable Vault Agent Injector | `false` |

For the full list of Vault chart parameters, see the [official documentation](https://developer.hashicorp.com/vault/docs/platform/k8s/helm/configuration).

## Examples

### Standalone Vault with auto-unseal (default)

```bash
helm dependency update ./charts/vault-auto-unseal
helm install vault-stack ./charts/vault-auto-unseal -n vault --create-namespace
```

### HA Vault with Raft storage

```yaml
# ha-values.yaml
controller:
  vaultNamespace: vault
  checkInterval: 15

vault:
  enabled: true
  server:
    replicas: 3
    ha:
      enabled: true
      replicas: 3
      raft:
        enabled: true
        setNodeId: true
        config: |
          ui = false
          listener "tcp" {
            tls_disable = 1
            address = "[::]:8200"
            cluster_address = "[::]:8201"
          }
          storage "raft" {
            path = "/vault/data"
          }
          service_registration "kubernetes" {}
    standalone:
      enabled: false
    dataStorage:
      enabled: true
      size: 20Gi
```

```bash
helm install vault-stack ./charts/vault-auto-unseal -n vault -f ha-values.yaml
```

### Vault with UI enabled

```yaml
# ui-values.yaml
vault:
  enabled: true
  ui:
    enabled: true
    serviceType: LoadBalancer
```

### Controller only (existing Vault)

```yaml
# controller-only-values.yaml
vault:
  enabled: false

controller:
  vaultNamespace: my-vault-namespace
  vaultPort: "8200"
  checkInterval: 30
```

### Production configuration

```yaml
# production-values.yaml
replicaCount: 1

image:
  tag: "v1.0.0"

controller:
  vaultNamespace: vault
  checkInterval: 30

resources:
  limits:
    cpu: 200m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi

nodeSelector:
  node-role.kubernetes.io/infra: "true"

vault:
  enabled: true
  server:
    replicas: 3
    image:
      tag: "1.15.4"
    resources:
      requests:
        memory: 512Mi
        cpu: 500m
      limits:
        memory: 512Mi
        cpu: 500m
    ha:
      enabled: true
      replicas: 3
      raft:
        enabled: true
        setNodeId: true
        config: |
          ui = false
          listener "tcp" {
            tls_disable = 1
            address = "[::]:8200"
            cluster_address = "[::]:8201"
          }
          storage "raft" {
            path = "/vault/data"
          }
          service_registration "kubernetes" {}
    standalone:
      enabled: false
    dataStorage:
      enabled: true
      size: 50Gi
  ui:
    enabled: false
  injector:
    enabled: false
```

## Health Check Endpoints

The controller exposes two HTTP endpoints on port 8080:

| Endpoint | Description |
|----------|-------------|
| `/health` | Returns `200 OK` if the controller is running (liveness) |
| `/ready` | Returns `200 OK` if all discovered Vault pods are unsealed (readiness) |

## Managed Kubernetes Secrets

The controller creates and manages these secrets in the Vault namespace:

| Secret Name | Key(s) | Description |
|-------------|--------|-------------|
| `vault-root-token` | `token` | The Vault root token |
| `vault-unseal-keys` | `key1`..`key5` | Shamir unseal key shares |

### Accessing Secrets

**Get the root token:**

```bash
kubectl get secret vault-root-token -n vault -o jsonpath='{.data.token}' | base64 -d
```

**Use the root token to log in to Vault:**

```bash
# Store the token in a variable
export VAULT_TOKEN=$(kubectl get secret vault-root-token -n vault -o jsonpath='{.data.token}' | base64 -d)

# Port-forward Vault and authenticate
kubectl port-forward svc/vault 8200:8200 -n vault &
export VAULT_ADDR=http://127.0.0.1:8200
vault status
vault token lookup
```

**Get all unseal keys:**

```bash
kubectl get secret vault-unseal-keys -n vault -o jsonpath='{.data}' | python3 -c "
import sys, json, base64
data = json.load(sys.stdin)
for k, v in sorted(data.items()):
    print(f'{k}: {base64.b64decode(v).decode()}')
"
```

**Get a single unseal key (e.g. key1):**

```bash
kubectl get secret vault-unseal-keys -n vault -o jsonpath='{.data.key1}' | base64 -d
```

**View all secrets as YAML:**

```bash
kubectl get secret vault-root-token vault-unseal-keys -n vault -o yaml
```

> **Warning:** Root tokens and unseal keys are highly sensitive. Avoid logging or storing them in plain text. For production, consider revoking the initial root token after configuring Vault and creating proper auth methods.

## Testing

Run Helm tests after installation to verify the controller is working:

```bash
helm test vault-stack -n vault
```

This runs two test pods:

| Test | Description |
|------|-------------|
| `test-health` | Verifies the `/health` endpoint returns `200 OK` |
| `test-connection` | Verifies TCP connectivity and `/ready` endpoint responsiveness |

## Upgrading

```bash
helm dependency update ./charts/vault-auto-unseal
helm upgrade vault-stack ./charts/vault-auto-unseal -n vault -f my-values.yaml
```

## Uninstalling

```bash
helm uninstall vault-stack -n vault
```

> **Note:** Uninstalling the chart does NOT delete:
> - The managed secrets (`vault-root-token` and `vault-unseal-keys`)
> - Persistent Volume Claims created by the Vault sub-chart
>
> Remove these manually if desired:
> ```bash
> kubectl delete secret vault-root-token vault-unseal-keys -n vault
> kubectl delete pvc -l app.kubernetes.io/name=vault -n vault
> ```

## Security Considerations

- The controller runs as a non-root user (UID 10001) with a read-only root filesystem
- All Linux capabilities are dropped
- RBAC is scoped to the minimum required permissions (secrets + pod listing)
- Unseal keys are stored as Kubernetes secrets -- ensure encryption at rest is enabled for etcd
- For production, consider using Vault's auto-unseal with a cloud KMS instead of Shamir keys
- Review and restrict network policies between the controller and Vault pods

## Troubleshooting

### Controller cannot find Vault pods

Ensure Vault pods have the required labels. The sub-chart sets `app.kubernetes.io/name=vault` automatically. The `component=server` label is added via `vault.server.extraLabels`:

```bash
kubectl get pods -n vault -l app.kubernetes.io/name=vault,component=server
```

### Vault is deployed but not initializing

Check that the controller and Vault are in the same namespace, or that `controller.vaultNamespace` matches:

```bash
kubectl logs -f deployment/<release-name>-vault-auto-unseal -n vault
```

### Permission denied errors

Verify RBAC resources:

```bash
kubectl get role,rolebinding -n vault
kubectl auth can-i get secrets --as=system:serviceaccount:vault:<sa-name> -n vault
```

### Vault PVC stuck in Pending

Ensure a StorageClass is available for dynamic provisioning:

```bash
kubectl get storageclass
kubectl get pvc -n vault
```

### Dependency issues

If `helm install` fails with dependency errors:

```bash
helm dependency update ./charts/vault-auto-unseal
helm dependency list ./charts/vault-auto-unseal
```

## Backup & Disaster Recovery

### Backing up secrets

Always maintain an offline backup of root tokens and unseal keys. If these are lost and Vault is sealed, data recovery is impossible.

```bash
# Export secrets to an encrypted file
kubectl get secret vault-root-token -n vault -o json > vault-root-token.json
kubectl get secret vault-unseal-keys -n vault -o json > vault-unseal-keys.json

# Encrypt with GPG before storing
gpg --encrypt --recipient your@email.com vault-root-token.json
gpg --encrypt --recipient your@email.com vault-unseal-keys.json

# Remove unencrypted files
rm vault-root-token.json vault-unseal-keys.json
```

### Backing up Vault data

For **file storage** (standalone mode):

```bash
# Create a snapshot from the Vault pod
kubectl exec -n vault vault-0 -- tar czf /tmp/vault-backup.tar.gz /vault/data
kubectl cp vault/vault-0:/tmp/vault-backup.tar.gz ./vault-backup-$(date +%Y%m%d).tar.gz
```

For **Raft storage** (HA mode):

```bash
export VAULT_TOKEN=$(kubectl get secret vault-root-token -n vault -o jsonpath='{.data.token}' | base64 -d)
kubectl port-forward svc/vault 8200:8200 -n vault &
vault operator raft snapshot save vault-raft-$(date +%Y%m%d).snap
```

### Restoring from backup

**Raft snapshot restore:**

```bash
kubectl port-forward svc/vault 8200:8200 -n vault &
export VAULT_TOKEN=$(kubectl get secret vault-root-token -n vault -o jsonpath='{.data.token}' | base64 -d)
vault operator raft snapshot restore vault-raft-20260101.snap
```

**Full disaster recovery (new cluster):**

1. Deploy the chart:
   ```bash
   helm install vault-stack ./charts/vault-auto-unseal -n vault --create-namespace
   ```
2. Wait for the controller to initialize Vault (new keys will be generated)
3. Restore the Raft snapshot (overwrites new data with backup)
4. Re-apply the original unseal keys secret:
   ```bash
   kubectl apply -f vault-unseal-keys.json -n vault
   ```
5. The controller will unseal Vault with the restored keys

### Automated backup CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: vault-backup
  namespace: vault
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: vault-auto-unseal
          containers:
            - name: backup
              image: bitnami/kubectl:latest
              command:
                - /bin/sh
                - -c
                - |
                  kubectl get secret vault-root-token -n vault -o json > /backup/vault-root-token.json
                  kubectl get secret vault-unseal-keys -n vault -o json > /backup/vault-unseal-keys.json
                  echo "Backup completed at $(date)"
              volumeMounts:
                - name: backup-volume
                  mountPath: /backup
          restartPolicy: OnFailure
          volumes:
            - name: backup-volume
              persistentVolumeClaim:
                claimName: vault-backup-pvc
```

## Monitoring & Logging

### Controller logs

The controller outputs structured logs to stdout. View them with:

```bash
# Stream logs
kubectl logs -f deployment/<release-name>-vault-auto-unseal -n vault

# View logs with timestamps
kubectl logs --timestamps deployment/<release-name>-vault-auto-unseal -n vault

# View logs from previous container (after a restart)
kubectl logs --previous deployment/<release-name>-vault-auto-unseal -n vault
```

### Key log messages

| Log Message | Meaning |
|-------------|---------|
| `Starting Vault auto-unseal controller` | Controller started successfully |
| `Found Vault pod <name> with IP <ip>` | Vault pod discovered |
| `Successfully initialized Vault and stored secrets` | First-time init completed |
| `No Vault pods found` | No pods match the label selector |
| `Error checking Vault status` | Cannot reach Vault API |
| `Error unsealing Vault` | Unseal attempt failed |

### Health check monitoring

Use the built-in health endpoints with your monitoring stack:

```yaml
# Prometheus ServiceMonitor (if using Prometheus Operator)
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: vault-auto-unseal
  namespace: vault
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: vault-auto-unseal
  endpoints:
    - port: http
      path: /health
      interval: 30s
```

### Alerting examples

**Prometheus alert rules:**

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: vault-auto-unseal-alerts
  namespace: vault
spec:
  groups:
    - name: vault-auto-unseal
      rules:
        - alert: VaultAutoUnsealNotReady
          expr: |
            kube_deployment_status_ready_replicas{deployment=~".*vault-auto-unseal.*"} == 0
          for: 5m
          labels:
            severity: critical
          annotations:
            summary: "Vault auto-unseal controller is not ready"
            description: "The auto-unseal controller has no ready replicas for 5 minutes."

        - alert: VaultAutoUnsealPodRestarting
          expr: |
            increase(kube_pod_container_status_restarts_total{container="vault-auto-unseal"}[1h]) > 3
          labels:
            severity: warning
          annotations:
            summary: "Vault auto-unseal controller is restarting frequently"
            description: "The controller has restarted more than 3 times in the last hour."
```

### Log aggregation

For centralized logging with Fluentd or Loki, the controller's stdout output can be collected by any standard Kubernetes log collector. No additional configuration is needed -- logs are plain text with timestamps.

## Network Policies

Restrict network traffic between the controller and Vault for defense-in-depth.

### Allow controller to Vault only

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: vault-auto-unseal-egress
  namespace: vault
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: vault-auto-unseal
  policyTypes:
    - Egress
  egress:
    # Allow traffic to Vault pods on port 8200
    - to:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: vault
              component: server
      ports:
        - protocol: TCP
          port: 8200
    # Allow DNS resolution
    - to:
        - namespaceSelector: {}
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
    # Allow traffic to Kubernetes API server (for secret management)
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
      ports:
        - protocol: TCP
          port: 443
```

### Restrict ingress to Vault

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: vault-server-ingress
  namespace: vault
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: vault
      component: server
  policyTypes:
    - Ingress
  ingress:
    # Allow auto-unseal controller
    - from:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: vault-auto-unseal
      ports:
        - protocol: TCP
          port: 8200
    # Allow Vault-to-Vault cluster traffic (HA mode)
    - from:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: vault
              component: server
      ports:
        - protocol: TCP
          port: 8200
        - protocol: TCP
          port: 8201
```

## TLS Configuration

By default, Vault runs without TLS (`tls_disable = 1`). For production, enable TLS.

### Using cert-manager

1. Install cert-manager and create a Certificate:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: vault-tls
  namespace: vault
spec:
  secretName: vault-tls-cert
  issuerRef:
    name: letsencrypt-prod  # or your ClusterIssuer
    kind: ClusterIssuer
  dnsNames:
    - vault.vault.svc.cluster.local
    - vault.vault.svc
    - vault
  duration: 8760h  # 1 year
  renewBefore: 720h  # 30 days
```

2. Update Vault configuration in values.yaml:

```yaml
vault:
  enabled: true
  server:
    extraVolumes:
      - type: secret
        name: vault-tls-cert

    standalone:
      enabled: true
      config: |
        ui = false
        listener "tcp" {
          address = "[::]:8200"
          cluster_address = "[::]:8201"
          tls_cert_file = "/vault/userconfig/vault-tls-cert/tls.crt"
          tls_key_file  = "/vault/userconfig/vault-tls-cert/tls.key"
        }
        storage "file" {
          path = "/vault/data"
        }
```

> **Note:** When TLS is enabled, the auto-unseal controller communicates with Vault over HTTP using pod IPs within the cluster. If you require end-to-end TLS, you will need to build a custom controller image with CA certificates mounted.

### Using a self-signed CA

```bash
# Generate CA
openssl genrsa -out ca.key 4096
openssl req -x509 -new -nodes -key ca.key -sha256 -days 3650 -out ca.crt -subj "/CN=Vault CA"

# Generate Vault server certificate
openssl genrsa -out vault.key 4096
openssl req -new -key vault.key -out vault.csr -subj "/CN=vault.vault.svc"
openssl x509 -req -in vault.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out vault.crt -days 365

# Create Kubernetes secret
kubectl create secret tls vault-tls-cert --cert=vault.crt --key=vault.key -n vault
```

## Root Token Revocation & Auth Methods

The initial root token should be revoked after setting up proper authentication. Root tokens have unlimited privileges and no TTL.

### Step 1: Set up a proper auth method

```bash
export VAULT_TOKEN=$(kubectl get secret vault-root-token -n vault -o jsonpath='{.data.token}' | base64 -d)
kubectl port-forward svc/vault 8200:8200 -n vault &
export VAULT_ADDR=http://127.0.0.1:8200

# Enable Kubernetes auth method
vault auth enable kubernetes
vault write auth/kubernetes/config \
  kubernetes_host="https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT"

# Create an admin policy
vault policy write admin - <<EOF
path "*" {
  capabilities = ["create", "read", "update", "delete", "list", "sudo"]
}
EOF

# Create a role for cluster admins
vault write auth/kubernetes/role/admin \
  bound_service_account_names=vault-admin \
  bound_service_account_namespaces=vault \
  policies=admin \
  ttl=1h
```

### Step 2: Verify the new auth method works

```bash
# Create a service account for testing
kubectl create serviceaccount vault-admin -n vault

# Get a token from Kubernetes auth
JWT=$(kubectl create token vault-admin -n vault)
vault write auth/kubernetes/login role=admin jwt=$JWT

# Verify access
vault token lookup
```

### Step 3: Revoke the root token

```bash
vault token revoke $VAULT_TOKEN
```

### Generating a new root token (emergency)

If you need root access again, use the unseal keys:

```bash
# Start root token generation
vault operator generate-root -init

# Each key holder provides their unseal key
vault operator generate-root \
  -nonce=<nonce-from-init> \
  <unseal-key>

# Decode the final encoded token
vault operator generate-root -decode=<encoded-token> -otp=<otp-from-init>
```

## Pod Security Standards

The chart is compatible with the Kubernetes **Restricted** Pod Security Standard. The controller runs with:

- `runAsNonRoot: true` (UID 10001)
- `readOnlyRootFilesystem: true`
- `allowPrivilegeEscalation: false`
- `capabilities.drop: ["ALL"]`
- No host namespaces, no privilege, no host ports

### Enforcing with Pod Security Admission

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: vault
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

> **Note:** The Vault sub-chart may require `baseline` level instead of `restricted`, depending on the Vault version and configuration. Test with `warn` mode first before enforcing.

### Verifying compliance

```bash
# Dry-run to check PSS violations
kubectl label --dry-run=server --overwrite ns vault \
  pod-security.kubernetes.io/enforce=restricted

# Check using kubeaudit (if installed)
helm template vault-stack ./charts/vault-auto-unseal -n vault \
  --set vault.enabled=false | kubeaudit all -f -
```

## Migration Guide

### From raw Kubernetes manifests to Helm

If you are currently using the raw manifests from the `k8s/` directory:

**Step 1: Identify your current configuration**

```bash
# Check existing resources
kubectl get deployment,serviceaccount,role,rolebinding -n vault -l app=vault-auto-unseal
```

**Step 2: Create a values file matching your current setup**

```yaml
# migration-values.yaml
controller:
  vaultNamespace: vault      # match your current VAULT_NAMESPACE env var
  vaultPort: "8200"          # match your current VAULT_PORT env var
  checkInterval: 10          # match your current CHECK_INTERVAL env var

vault:
  enabled: false             # keep using your existing Vault

image:
  repository: ghcr.io/getgrowly/vault-utils
  tag: "latest"              # match your current image tag
```

**Step 3: Remove old resources**

```bash
kubectl delete deployment vault-auto-unseal -n vault
kubectl delete serviceaccount vault-auto-unseal -n vault
kubectl delete role vault-auto-unseal -n vault
kubectl delete rolebinding vault-auto-unseal -n vault
```

**Step 4: Install the Helm chart**

```bash
helm dependency update ./charts/vault-auto-unseal
helm install vault-auto-unseal ./charts/vault-auto-unseal \
  -n vault -f migration-values.yaml
```

**Step 5: Verify**

```bash
helm test vault-auto-unseal -n vault
kubectl logs -f deployment/<release>-vault-auto-unseal -n vault
```

> **Note:** Existing Kubernetes secrets (`vault-root-token`, `vault-unseal-keys`) are preserved during migration. The controller will detect and use them.

## Multi-namespace Setup

The controller can monitor Vault pods in a different namespace than where it is deployed.

### Controller in `monitoring`, Vault in `vault`

```yaml
# multi-ns-values.yaml
controller:
  vaultNamespace: vault

vault:
  enabled: false  # Vault is managed separately

rbac:
  create: true    # Creates Role/RoleBinding in the vault namespace

serviceAccount:
  create: true    # Creates ServiceAccount in the vault namespace
```

```bash
helm install vault-auto-unseal ./charts/vault-auto-unseal \
  -n monitoring \
  -f multi-ns-values.yaml
```

> **Important:** The RBAC Role and ServiceAccount are created in `controller.vaultNamespace` (the Vault namespace), not in the release namespace. This is required because the controller needs permissions to list pods and manage secrets in the Vault namespace.

### Multiple Vault clusters

To monitor multiple Vault clusters, deploy one controller per namespace:

```bash
# Cluster 1
helm install unseal-prod ./charts/vault-auto-unseal \
  -n vault-prod \
  --set controller.vaultNamespace=vault-prod \
  --set vault.enabled=false

# Cluster 2
helm install unseal-staging ./charts/vault-auto-unseal \
  -n vault-staging \
  --set controller.vaultNamespace=vault-staging \
  --set vault.enabled=false
```

## Cloud KMS Auto-Unseal

For production environments, consider replacing Shamir-based unsealing with cloud KMS auto-unseal. This eliminates the need for unseal keys entirely.

### AWS KMS

```yaml
# aws-kms-values.yaml
vault:
  enabled: true
  server:
    extraEnvironmentVars:
      AWS_REGION: us-east-1
    serviceAccount:
      annotations:
        eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/vault-kms
    standalone:
      enabled: true
      config: |
        ui = false
        listener "tcp" {
          tls_disable = 1
          address = "[::]:8200"
          cluster_address = "[::]:8201"
        }
        storage "file" {
          path = "/vault/data"
        }
        seal "awskms" {
          region     = "us-east-1"
          kms_key_id = "arn:aws:kms:us-east-1:123456789012:key/your-key-id"
        }
```

### GCP Cloud KMS

```yaml
# gcp-kms-values.yaml
vault:
  enabled: true
  server:
    extraEnvironmentVars:
      GOOGLE_APPLICATION_CREDENTIALS: /vault/userconfig/gcp-sa/credentials.json
    extraVolumes:
      - type: secret
        name: gcp-sa-key
    serviceAccount:
      annotations:
        iam.gke.io/gcp-service-account: vault-kms@your-project.iam.gserviceaccount.com
    standalone:
      enabled: true
      config: |
        ui = false
        listener "tcp" {
          tls_disable = 1
          address = "[::]:8200"
          cluster_address = "[::]:8201"
        }
        storage "file" {
          path = "/vault/data"
        }
        seal "gcpckms" {
          project    = "your-project"
          region     = "global"
          key_ring   = "vault-keyring"
          crypto_key = "vault-unseal-key"
        }
```

### Azure Key Vault

```yaml
# azure-kv-values.yaml
vault:
  enabled: true
  server:
    extraEnvironmentVars:
      AZURE_TENANT_ID: "your-tenant-id"
      AZURE_CLIENT_ID: "your-client-id"
      AZURE_CLIENT_SECRET: "your-client-secret"
    standalone:
      enabled: true
      config: |
        ui = false
        listener "tcp" {
          tls_disable = 1
          address = "[::]:8200"
          cluster_address = "[::]:8201"
        }
        storage "file" {
          path = "/vault/data"
        }
        seal "azurekeyvault" {
          vault_name = "your-keyvault-name"
          key_name   = "vault-unseal-key"
        }
```

> **Note:** When using cloud KMS auto-unseal, the auto-unseal controller is still useful for initial Vault initialization and for monitoring Vault health status. However, Vault will automatically unseal itself on restarts using the KMS key, so the controller's unseal function becomes a fallback mechanism.

## FAQ

### Does the controller work with Vault Enterprise?

Yes. The controller uses standard Vault API endpoints (`/v1/sys/seal-status`, `/v1/sys/init`, `/v1/sys/unseal`) that are available in both Community and Enterprise editions.

### What happens if the controller pod restarts?

The controller resumes monitoring immediately on restart. Since unseal keys and root tokens are stored as Kubernetes secrets, no state is lost. If Vault is already unsealed, the controller simply continues health checks.

### Can I run multiple controller replicas?

Yes, but it is not necessary. Multiple replicas will all attempt to check and unseal Vault, which is safe but redundant. A single replica with Kubernetes restart policy is sufficient for most use cases.

### What happens when Vault is already initialized?

The controller detects the initialized state and skips the initialization step. It only attempts to unseal if Vault is sealed.

### How does the controller discover Vault pods?

It uses the Kubernetes API with the label selector `app.kubernetes.io/name=vault,component=server`. The Vault sub-chart automatically sets `app.kubernetes.io/name=vault`, and the `component=server` label is added via `vault.server.extraLabels` in the default values.

### Can I use this with an external Vault (outside Kubernetes)?

No. The controller discovers Vault instances by listing Kubernetes pods. For external Vault instances, use Vault's built-in auto-unseal with a cloud KMS provider.

### What if unseal keys are deleted accidentally?

If the `vault-unseal-keys` secret is deleted while Vault is running (unsealed), Vault continues to operate. However, on the next restart, Vault will be sealed and the controller cannot unseal it. Always maintain offline backups of unseal keys (see [Backup & Disaster Recovery](#backup--disaster-recovery)).

### How do I upgrade the Vault version?

Update the `vault.server.image.tag` in your values file and run `helm upgrade`. Vault handles rolling upgrades gracefully:

```bash
helm upgrade vault-stack ./charts/vault-auto-unseal \
  -n vault --set vault.server.image.tag=1.16.0
```

### Is the controller affected by Vault's seal/unseal status?

The controller's `/health` endpoint always returns `200 OK` as long as the controller is running. The `/ready` endpoint returns `200 OK` only when all discovered Vault pods are unsealed, and `503` otherwise.

## Changelog

### v0.1.0

- Initial release
- Vault Auto-Unseal Controller deployment
- HashiCorp Vault as optional sub-chart dependency (v0.28.1)
- RBAC with least-privilege permissions
- Health check Service (`/health`, `/ready`)
- Helm test pods for health and connectivity verification
- Standalone and HA (Raft) Vault configurations
- Security hardening (non-root, read-only fs, dropped capabilities)
- Artifact Hub publishing support
