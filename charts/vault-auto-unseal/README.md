# Vault Auto-Unseal Helm Chart

A Helm chart for deploying the Vault Auto-Unseal Controller on Kubernetes. This controller automatically monitors, initializes, and unseals HashiCorp Vault instances running in your cluster.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.2.0+
- HashiCorp Vault deployed in the target namespace (with pod labels `app.kubernetes.io/name=vault` and `component=server`)

## Quick Start

### Add and install the chart

```bash
helm install vault-auto-unseal ./charts/vault-auto-unseal \
  --namespace vault \
  --create-namespace
```

### Install with custom values

```bash
helm install vault-auto-unseal ./charts/vault-auto-unseal \
  --namespace vault \
  --create-namespace \
  --set vault.namespace=vault \
  --set vault.checkInterval=30
```

### Install using a values file

```bash
helm install vault-auto-unseal ./charts/vault-auto-unseal \
  --namespace vault \
  --create-namespace \
  -f my-values.yaml
```

## How It Works

The controller runs as a Kubernetes Deployment and performs the following actions in a continuous loop:

1. **Discovers Vault pods** in the configured namespace using Kubernetes label selectors (`app.kubernetes.io/name=vault,component=server`)
2. **Checks status** of each Vault pod via the `/v1/sys/seal-status` API
3. **Initializes** uninitialized Vault instances (creates 5 key shares with a threshold of 3)
4. **Stores secrets** as Kubernetes secrets:
   - `vault-root-token` - Contains the Vault root token
   - `vault-unseal-keys` - Contains unseal keys (`key1` through `key5`)
5. **Unseals** sealed Vault instances using stored unseal keys

## Configuration

### Vault Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `vault.namespace` | Kubernetes namespace where Vault pods run | `vault` |
| `vault.port` | Port where Vault API is listening | `8200` |
| `vault.checkInterval` | Seconds between status checks | `10` |

### Image Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Container image repository | `ghcr.io/getgrowly/vault-utils` |
| `image.tag` | Image tag (defaults to chart `appVersion`) | `""` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `imagePullSecrets` | Image pull secrets for private registries | `[]` |

### Deployment Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of controller replicas | `1` |
| `podAnnotations` | Annotations to add to the pod | `{}` |
| `podLabels` | Labels to add to the pod | `{}` |
| `nodeSelector` | Node selector for pod scheduling | `{}` |
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
| `podSecurityContext.runAsNonRoot` | Run pod as non-root user | `true` |
| `podSecurityContext.runAsUser` | UID for pod user | `10001` |
| `podSecurityContext.fsGroup` | Group ID for filesystem access | `10001` |
| `securityContext.allowPrivilegeEscalation` | Allow privilege escalation | `false` |
| `securityContext.readOnlyRootFilesystem` | Read-only root filesystem | `true` |
| `securityContext.capabilities.drop` | Linux capabilities to drop | `["ALL"]` |

### Service Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Kubernetes service type | `ClusterIP` |
| `service.port` | Service port for health checks | `8080` |

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
| `livenessProbe.initialDelaySeconds` | Delay before first liveness check | `5` |
| `livenessProbe.periodSeconds` | Interval between liveness checks | `10` |
| `readinessProbe.httpGet.path` | Readiness probe endpoint | `/ready` |
| `readinessProbe.initialDelaySeconds` | Delay before first readiness check | `5` |
| `readinessProbe.periodSeconds` | Interval between readiness checks | `10` |

## Examples

### Minimal installation

```bash
helm install vault-auto-unseal ./charts/vault-auto-unseal -n vault
```

### Production configuration

```yaml
# production-values.yaml
replicaCount: 1

image:
  repository: ghcr.io/getgrowly/vault-utils
  tag: "v1.0.0"

vault:
  namespace: vault
  port: "8200"
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

tolerations:
  - key: "node-role.kubernetes.io/infra"
    operator: "Exists"
    effect: "NoSchedule"

podAnnotations:
  prometheus.io/scrape: "false"
```

```bash
helm install vault-auto-unseal ./charts/vault-auto-unseal \
  -n vault -f production-values.yaml
```

### Using with an existing ServiceAccount

```yaml
serviceAccount:
  create: false
  name: my-existing-sa

rbac:
  create: false
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
helm upgrade vault-auto-unseal ./charts/vault-auto-unseal \
  -n vault -f my-values.yaml
```

## Uninstalling

```bash
helm uninstall vault-auto-unseal -n vault
```

> **Note:** Uninstalling the chart does NOT delete the managed secrets (`vault-root-token` and `vault-unseal-keys`). These contain critical Vault credentials and must be removed manually if desired.

## Security Considerations

- The controller runs as a non-root user (UID 10001) with a read-only root filesystem
- All Linux capabilities are dropped
- RBAC is scoped to the minimum required permissions (secrets + pod listing in the Vault namespace only)
- Unseal keys are stored as Kubernetes secrets -- ensure your cluster has encryption at rest enabled for etcd
- For production environments, consider using Vault's auto-unseal feature with a cloud KMS provider instead of Shamir keys

## Troubleshooting

### Controller cannot find Vault pods

Ensure your Vault pods have the correct labels:
```yaml
labels:
  app.kubernetes.io/name: vault
  component: server
```

### Permission denied errors

Verify that RBAC resources are created and the ServiceAccount is correctly bound:
```bash
kubectl get role,rolebinding -n vault
kubectl auth can-i get secrets --as=system:serviceaccount:vault:<sa-name> -n vault
```

### Controller is not ready

The `/ready` endpoint returns `503` until all Vault pods are unsealed. Check controller logs:
```bash
kubectl logs -f deployment/vault-auto-unseal -n vault
```
