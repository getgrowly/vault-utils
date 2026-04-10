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
