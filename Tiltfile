# Tiltfile for Hivemind local development

print('🧠 Hivemind Tilt Configuration')

# Load environment variables from .env if it exists
load('ext://dotenv', 'dotenv')
load('ext://secret', 'secret_from_dict')
dotenv(fn='.env')

# =============================================================================
# Configuration
# =============================================================================

# Determine deployment mode: Kubernetes if K8S_CONTEXT is set, otherwise local
k8s_context = os.getenv('K8S_CONTEXT', '')
use_k8s = k8s_context != ''

# Registry configuration: external registry accessible from everywhere (matches daisybot)
registry = os.getenv('DOCKER_REGISTRY', 'registry.local.rothwell.us')
default_registry(registry)

# BuildKit configuration (matches daisybot).
# Default: local buildx (`default` builder). Set BUILDKIT_HOST to build with the
# cluster BuildKit (e.g. tcp://buildkit.local.rothwell.us:PORT); the remote builder
# is created on demand if it does not already exist.
buildkit_host = os.getenv('BUILDKIT_HOST', os.getenv('BUILDKIT_REMOTE_HOST', ''))
buildkit_builder = os.getenv('BUILDKIT_BUILDER', 'remote' if buildkit_host else 'default')
docker_context = os.getenv('DOCKER_CONTEXT', 'desktop-linux')
use_remote_build = buildkit_host != ''

# Go proxy: Athens with public fallback (matches daisybot; required so builds can
# resolve the private github.com/devilmonastery/hivemind/api module).
goproxy = os.getenv('GOPROXY', 'https://athens.local.rothwell.us,https://proxy.golang.org,direct')

os.putenv('DOCKER_BUILDKIT', '1')
if buildkit_host:
    os.putenv('BUILDKIT_HOST', buildkit_host)

# buildkit_build runs a buildx build via the cluster/remote BuildKit when
# BUILDKIT_HOST is set (creating the remote builder if missing), otherwise via the
# local `default` builder. Mirrors daisybot's Tiltfile.
def buildkit_build(image, dockerfile, deps, extra_build_args=''):
    remote = 'docker --context="$DOCKER_CONTEXT" buildx inspect "$BUILDKIT_BUILDER" >/dev/null 2>&1 || docker --context="$DOCKER_CONTEXT" buildx create --name "$BUILDKIT_BUILDER" --driver remote "$BUILDKIT_HOST"; docker --context="$DOCKER_CONTEXT" buildx build --builder="$BUILDKIT_BUILDER"'
    localb = 'docker --context="$DOCKER_CONTEXT" buildx build --builder=default'
    common = ' --platform=linux/amd64 --build-arg GOPROXY=' + goproxy + ' ' + extra_build_args + ' -f ' + dockerfile + ' --tag $EXPECTED_REF --push .'
    cmd = 'if [ -n "$BUILDKIT_HOST" ]; then ' + remote + common + '; else ' + localb + common + '; fi'
    custom_build(
        image,
        cmd,
        deps=deps,
        env={'BUILDKIT_HOST': buildkit_host, 'BUILDKIT_BUILDER': buildkit_builder, 'DOCKER_CONTEXT': docker_context},
        skips_local_docker=True,
    )

# Kubernetes configuration
if k8s_context:
    allow_k8s_contexts([k8s_context, 'default', 'turing'])
else:
    allow_k8s_contexts(['default', 'turing'])
    
user = os.getenv('USER', 'dev')
namespace = os.getenv('TILT_NAMESPACE', 'hivemind-' + user)

# Print configuration
print('👤 Developer: ' + user)
if use_k8s:
    print('☸️  Mode: Kubernetes')
    print('📦 Namespace: ' + namespace)
    print('🔧 Context: ' + k8s_context)
    print('🔨 BuildKit: ' + ('Remote (' + buildkit_host + ')' if use_remote_build else 'Local Docker'))
    print('📦 Registry: ' + (registry if registry else 'WARNING: No registry set!'))
    print('🐹 GOPROXY: ' + goproxy)
else:
    print('💻 Mode: Local (go run)')

# =============================================================================
# Database
# =============================================================================

if not use_k8s:
    # Local mode: Start PostgreSQL using docker-compose
    docker_compose('./dev/docker-compose.yaml')
    dc_resource('postgres', labels=['database'])

# =============================================================================
# Application Services
# =============================================================================

if use_k8s:
    # Kubernetes mode: Build images and deploy to Kubernetes
    
    # Create namespace
    namespace_yaml = """
apiVersion: v1
kind: Namespace
metadata:
  name: %s
""" % namespace
    k8s_yaml(blob(namespace_yaml))
    
    # Create secrets from .env
    k8s_yaml(secret_from_dict(
        'hivemind-secrets',
        namespace=namespace,
        inputs={
            'DB_PASSWORD': os.getenv('DB_PASSWORD', 'postgres'),
            'JWT_SIGNING_KEY': os.getenv('JWT_SIGNING_KEY', ''),
            'OAUTH_ENCRYPTION_KEY': os.getenv('OAUTH_ENCRYPTION_KEY', ''),
            'SESSION_SECRET': os.getenv('SESSION_SECRET', ''),
            'DISCORD_APPLICATION_ID': os.getenv('DISCORD_APPLICATION_ID', ''),
            'DISCORD_CLIENT_SECRET': os.getenv('DISCORD_CLIENT_SECRET', ''),
            'DISCORD_BOT_TOKEN': os.getenv('DISCORD_BOT_TOKEN', ''),
            'DEV_BOT_TOKEN': os.getenv('DEV_BOT_TOKEN', ''),
        }
    ))
    
    # Create ConfigMaps from config files (use names that match default search paths)
    server_config = local('kubectl create configmap hivemind-server-config --from-file=config.yaml=configs/k8s-server.yaml --dry-run=client -o yaml --namespace=' + namespace)
    k8s_yaml(server_config)
    
    web_config = local('kubectl create configmap hivemind-web-config --from-file=config.yaml=configs/k8s-web.yaml --dry-run=client -o yaml --namespace=' + namespace)
    k8s_yaml(web_config)
    
    bot_config = local('kubectl create configmap hivemind-bot-config --from-file=config.yaml=configs/k8s-bot.yaml --dry-run=client -o yaml --namespace=' + namespace)
    k8s_yaml(bot_config)
    
    # Create ConfigMap for migrations
    migrations_config = local('kubectl create configmap hivemind-migrations --from-file=migrations/postgres --dry-run=client -o yaml --namespace=' + namespace)
    k8s_yaml(migrations_config)
    
    # Build server image
    buildkit_build(
        'hivemind-server',
        'Dockerfile.server',
        deps=[
            './server',
            './internal',
            './api',
            './migrations',
            './configs',
            './go.mod',
            './go.sum',
        ],
    )
    
    # Build web image
    # Generate version string for cache busting
    version = str(local('git rev-parse --short HEAD 2>/dev/null || echo "dev"')).strip()
    buildkit_build(
        'hivemind-web',
        'Dockerfile.web',
        deps=[
            './web',
            './internal',
            './api',
            './configs',
            './go.mod',
            './go.sum',
        ],
        extra_build_args='--build-arg VERSION=' + version,
    )
    
    # Build bot image
    buildkit_build(
        'hivemind-bot',
        'Dockerfile.bot',
        deps=[
            './bot',
            './internal',
            './api',
            './configs',
            './go.mod',
            './go.sum',
        ],
    )
    
    # Deploy to Kubernetes with namespace injection
    yaml_with_namespace = local('kubectl create -f k8s/deployment.yaml --dry-run=client -o yaml --namespace=' + namespace)
    k8s_yaml(yaml_with_namespace)
    
    # Configure resources
    k8s_resource(
        'postgres',
        labels=['database'],
        resource_deps=[],
    )
    
    k8s_resource(
        'hivemind-server',
        labels=['backend'],
        port_forwards=[
            port_forward(4153, 4153, name='grpc'),
            port_forward(4163, 4163, name='metrics'),
        ],
        links=[
            link('http://localhost:4163/metrics', 'Server Metrics'),
        ],
        resource_deps=['postgres'],
    )
    
    k8s_resource(
        'hivemind-web',
        labels=['frontend'],
        port_forwards=[
            port_forward(8080, 8080, name='web'),
        ],
        links=[
            link('http://localhost:8080', 'Web UI'),
        ],
        resource_deps=['hivemind-server'],
    )
    
    k8s_resource(
        'hivemind-bot',
        labels=['bot'],
        port_forwards=[
            port_forward(9091, 9091, name='metrics'),
        ],
        links=[
            link('http://localhost:9091/metrics', 'Bot Metrics'),
        ],
        resource_deps=['hivemind-server'],
    )

else:
    # Local build mode: Fast iteration with go run
    
    # Build and run gRPC server with fast rebuild
    local_resource(
        'hivemind-server',
        serve_cmd='go run ./server -config ./configs/dev-server.yaml',
        deps=[
            './server',
            './internal',
            './api/generated',
            './migrations',
            './configs/dev-server.yaml',
        ],
        labels=['backend'],
        resource_deps=['postgres'],
        readiness_probe=probe(
            period_secs=5,
            exec=exec_action(['nc', '-z', 'localhost', '4153']),
        ),
    )
    
    # Build and run web server with fast rebuild
    local_resource(
        'hivemind-web',
        serve_cmd='go run ./web -config ./configs/dev-web.yaml',
        deps=[
            './web',
            './internal',
            './api/generated',
            './configs/dev-web.yaml',
        ],
        labels=['frontend'],
        resource_deps=['hivemind-server'],
        links=[
            link('http://localhost:8080', 'Web UI'),
        ],
    )

# =============================================================================
# Build Tools
# =============================================================================

# Optional: Proto generation (manual trigger)
local_resource(
    'generate-proto',
    cmd='make proto',
    deps=['./api/proto'],
    labels=['build'],
    auto_init=False,
    trigger_mode=TRIGGER_MODE_MANUAL,
)

# =============================================================================
# Summary
# =============================================================================

print('✅ Tilt configuration loaded!')
print('👉 Run: tilt up')
print('🌐 Web UI: http://localhost:8080')
print('🔌 gRPC: localhost:4153')
print('📊 Tilt UI: http://localhost:10350')
