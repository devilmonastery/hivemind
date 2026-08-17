# Tilt Development Guide

## Quick Start (Local Development)

Default mode uses local Go builds with fast iteration:

```bash
tilt up
```

This will:
- Start PostgreSQL via docker-compose
- Run server and web with `go run` (instant rebuilds)
- Auto-reload on file changes

## Remote Build Mode (Kubernetes)

For Kubernetes-based development with remote BuildKit and Athens proxy:

### 1. Create `.env` file

```bash
cp .env.example .env
```

### 2. Configure remote build in `.env`

```bash
# Enable remote builds by setting BuildKit endpoint
BUILDKIT_REMOTE_HOST=tcp://172.17.23.17:1234

# Optional: Internal cluster BuildKit endpoint
# BUILDKIT_CLUSTER_HOST=tcp://buildkitd.buildkit.svc.cluster.local:1234

# Go module proxy for faster builds
GOPROXY=https://athens.local.rothwell.us,https://proxy.golang.org,direct

# Container registry to push images
DOCKER_REGISTRY=registry.local.rothwell.us

# Kubernetes context (optional)
K8S_CONTEXT=turing

# Custom namespace (optional, defaults to hivemind-{username})
# TILT_NAMESPACE=hivemind-custom
```

### 3. Set up remote BuildKit builder

```bash
docker buildx create --name=remote --driver=remote ${BUILDKIT_REMOTE_HOST}
docker buildx use remote
```

### 4. Ensure kubectl is configured

```bash
kubectl config current-context  # Should match your K8S_CONTEXT
kubectl get nodes              # Verify cluster access
```

### 5. Run Tilt

```bash
tilt up
```

This will:
- Create a namespaced environment: `hivemind-{username}` (e.g., `hivemind-mrothwell`)
- Build images with remote BuildKit and push to your registry
- Deploy to Kubernetes with secrets, configmaps, and services
- Set up port forwarding: 4153 (gRPC), 8080 (Web)

### 4. Run Tilt

```bash
tilt up
```

## Configuration Options

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `BUILDKIT_REMOTE_HOST` | - | Remote BuildKit endpoint (enables remote builds) |
| `BUILDKIT_CLUSTER_HOST` | `tcp://buildkitd...` | Internal cluster BuildKit endpoint |
| `GOPROXY` | `proxy.golang.org,direct` | Go module proxy |
| `DOCKER_REGISTRY` | - | Docker registry for images |
| `K8S_CONTEXT` | - | Kubernetes context |
| `TILT_NAMESPACE` | `hivemind-{user}` | Kubernetes namespace |

## Build Modes Comparison

### Local Mode (Default)
- ✅ Fastest iteration (go run, no Docker build)
- ✅ Works out of the box
- ✅ Best for rapid development
- ❌ Doesn't test Docker images
- ❌ Doesn't test Kubernetes deployment

### Remote Mode
- ✅ Tests actual Docker images
- ✅ Tests Kubernetes deployment
- ✅ Faster Docker builds (BuildKit + caching)
- ✅ Athens proxy speeds up Go downloads
- ❌ Requires additional infrastructure
- ❌ Slightly slower iteration

## Tips

### Force rebuild proto files
```bash
tilt trigger generate-proto
```

### View logs for a specific service
Click on the service in Tilt UI or use:
```bash
tilt logs hivemind-server
```

### Reset everything
```bash
tilt down
docker-compose -f dev/docker-compose.yaml down -v
tilt up
```
