# Kubernetes Deployment for Hivemind Bot

This directory contains Kubernetes manifests for deploying the Hivemind Discord bot with high availability and leader election support.

## Overview

The bot uses Kubernetes leader election to coordinate background sync jobs across multiple replicas:
- Only one replica (the leader) runs periodic guild member syncs
- All replicas handle Discord slash commands and interactions
- If the leader fails, another replica automatically takes over
- This prevents duplicate Discord API calls and database writes

## Prerequisites

1. **Kubernetes cluster** (1.19+)
2. **Backend server** deployed and accessible (`hivemind-server` service)
3. **Discord bot credentials** (bot token and application ID)
4. **Service token** for authenticating with the backend

## Quick Start

### 1. Create Namespace (optional)

```bash
kubectl create namespace hivemind
```

Update the `namespace` field in all manifests if not using `default`.

### 2. Create Secrets

```bash
# Generate service token (run on server pod or locally with server binary)
SERVICE_TOKEN=$(./bin/hivemind-server token --type service)

# Create secret
kubectl create secret generic hivemind-bot-secrets \
  --from-literal=discord-token="YOUR_DISCORD_BOT_TOKEN" \
  --from-literal=application-id="YOUR_APPLICATION_ID" \
  --from-literal=service-token="$SERVICE_TOKEN" \
  --namespace=default
```

### 3. Deploy RBAC Resources

```bash
kubectl apply -f bot-rbac.yaml
```

This creates:
- ServiceAccount: `hivemind-bot`
- Role: `hivemind-bot-leader-election` (for managing Leases)
- RoleBinding: Attaches the role to the service account

### 4. Deploy Bot

```bash
# Update environment variables in bot-deployment.yaml first!
kubectl apply -f bot-deployment.yaml
```

Update these values in `bot-deployment.yaml`:
- `spec.template.spec.containers[0].image` - Your Docker image
- `BACKEND_GRPC_HOST` - Your backend service name
- `WEB_BASE_URL` - Your web interface URL

### 5. Verify Deployment

```bash
# Check pods
kubectl get pods -l app=hivemind-bot

# Check leader election
kubectl get lease hivemind-bot-sync-leader

# View logs
kubectl logs -l app=hivemind-bot --tail=50 -f

# Look for leader election messages
kubectl logs -l app=hivemind-bot | grep -i "leader\|election"
```

Expected log messages:
```
detected Kubernetes environment, using leader election for syncs
detected namespace from service account namespace=hivemind
starting leader election for sync job identity=hivemind-bot-xxx namespace=hivemind
elected as sync leader, starting member sync job identity=hivemind-bot-xxx
```

## Configuration

### Environment Variables

Required in `bot-deployment.yaml`:

| Variable | Description | Example |
|----------|-------------|---------|
| `POD_NAMESPACE` | K8s namespace (optional - auto-detected from service account if not set) | `hivemind` |
| `DISCORD_BOT_TOKEN` | Discord bot token (from Secret) | `Bot MTk...` |
| `DISCORD_APPLICATION_ID` | Discord application ID (from Secret) | `123456789...` |
| `BACKEND_SERVICE_TOKEN` | Backend auth token (from Secret) | `service_...` |
| `BACKEND_GRPC_HOST` | Backend gRPC service name | `hivemind-server` |
| `BACKEND_GRPC_PORT` | Backend gRPC port | `50051` |
| `WEB_BASE_URL` | Web interface URL for links | `https://hivemind.example.com` |
| `LOG_LEVEL` | Logging level | `info` |
| `LOG_FORMAT` | Log format | `json` |

### Scaling

```bash
# Scale to 5 replicas
kubectl scale deployment hivemind-bot --replicas=5

# Only one replica will run sync jobs (the leader)
# All replicas handle Discord interactions
```

### Leader Election Configuration

Configured in `bot/internal/bot/bot.go`:
- **Lease Duration**: 15 seconds (how long leader holds the lock)
- **Renew Deadline**: 10 seconds (how often leader renews)
- **Retry Period**: 2 seconds (how often non-leaders check for takeover)

Failed leaders are detected within 2-5 seconds, and a new leader is elected.

## Troubleshooting

### Bot not starting

```bash
# Check pod status
kubectl describe pod -l app=hivemind-bot

# Check logs
kubectl logs -l app=hivemind-bot --tail=100
```

Common issues:
- Missing service account token (not using `serviceAccountName`)
- RBAC permissions not applied
- Backend server not reachable

### Leader election not working

```bash
# Check lease status (adjust namespace if not using default)
kubectl get lease hivemind-bot-sync-leader -n hivemind -o yaml

# Verify RBAC (adjust namespace to match your deployment)
kubectl auth can-i create leases --as=system:serviceaccount:hivemind:hivemind-bot -n hivemind
kubectl auth can-i update leases --as=system:serviceaccount:hivemind:hivemind-bot -n hivemind
```

If RBAC is wrong, reapply `bot-rbac.yaml`.

**Namespace Detection:**
- The bot automatically detects its namespace from `/var/run/secrets/kubernetes.io/serviceaccount/namespace`
- If `POD_NAMESPACE` env var is set, it takes precedence
- Falls back to `default` only if both methods fail
- Check logs for: `detected namespace from service account namespace=hivemind`

### Multiple replicas syncing

Check logs for leader election messages. If you see "detected standalone environment", the bot isn't detecting Kubernetes:
- Ensure `/var/run/secrets/kubernetes.io/serviceaccount/token` exists in the pod
- Verify `serviceAccountName: hivemind-bot` is set in deployment

### Bot crashes on leader loss

This is intentional - when a leader loses leadership, it stops sync jobs but continues handling Discord interactions. If the pod crashes, check for other errors in logs.

## Security Notes

1. **Don't commit secrets** - Use `kubectl create secret` or a secrets manager
2. **Namespace isolation** - Deploy in a dedicated namespace with network policies
3. **Resource limits** - Set appropriate CPU/memory limits
4. **Least privilege RBAC** - The role only grants access to `leases` in the namespace
5. **Image scanning** - Scan Docker images for vulnerabilities

## Production Recommendations

1. **Use a secrets manager** (Vault, AWS Secrets Manager, etc.) instead of K8s Secrets
2. **Set pod disruption budgets** to prevent all replicas from being evicted
3. **Configure horizontal pod autoscaling** based on Discord API latency
4. **Use network policies** to restrict pod communication
5. **Enable audit logging** for the service account
6. **Monitor leader election metrics** (lease renewals, leader changes)
7. **Set up alerts** for leader election failures

## Files

- `bot-rbac.yaml` - ServiceAccount, Role, and RoleBinding for leader election
- `bot-deployment.yaml` - Deployment with 3 replicas
- `bot-secrets.yaml` - Example Secret (DO NOT use in production)
- `README.md` - This file
