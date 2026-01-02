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

# BuildKit configuration (for remote builds)
buildkit_remote = os.getenv('BUILDKIT_REMOTE_HOST', '')
buildkit_cluster = os.getenv('BUILDKIT_CLUSTER_HOST', 'tcp://buildkitd.buildkit.svc.cluster.local:1234')
use_remote_build = buildkit_remote != ''
buildkit_host = buildkit_remote if buildkit_remote else buildkit_cluster

# Go proxy configuration
goproxy = os.getenv('GOPROXY', 'https://proxy.golang.org,direct')

# Registry configuration (required for Kubernetes mode)
registry = os.getenv('DOCKER_REGISTRY', '')
if registry:
    default_registry(registry)

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
    
if use_remote_build:
    os.putenv('DOCKER_BUILDKIT', '1')
    os.putenv('BUILDKIT_HOST', buildkit_host)

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
        }
    ))
    
    # Create ConfigMaps from config files (use names that match default search paths)
    server_config = local('kubectl create configmap hivemind-server-config --from-file=config.yaml=configs/k8s-server.yaml --dry-run=client -o yaml --namespace=' + namespace)
    k8s_yaml(server_config)
    
    web_config = local('kubectl create configmap hivemind-web-config --from-file=config.yaml=configs/k8s-web.yaml --dry-run=client -o yaml --namespace=' + namespace)
    k8s_yaml(web_config)
    
    # Create ConfigMap for migrations
    migrations_config = local('kubectl create configmap hivemind-migrations --from-file=migrations/postgres --dry-run=client -o yaml --namespace=' + namespace)
    k8s_yaml(migrations_config)
    
    # Build server image
    if use_remote_build:
        custom_build(
            'hivemind-server',
            'docker buildx build --builder=remote --platform=linux/amd64 --build-arg GOPROXY=' + goproxy + ' -f Dockerfile.server --tag $EXPECTED_REF --push .',
            deps=[
                './server',
                './internal',
                './api',
                './migrations',
                './configs',
                './go.mod',
                './go.sum',
            ],
            env={'BUILDKIT_HOST': buildkit_host},
            skips_local_docker=True,
        )
    else:
        docker_build(
            'hivemind-server',
            '.',
            dockerfile='Dockerfile.server',
            build_args={'GOPROXY': goproxy},
        )
    
    # Build web image
    # Generate version string for cache busting
    version = str(local('git rev-parse --short HEAD 2>/dev/null || echo "dev"')).strip()
    if use_remote_build:
        custom_build(
            'hivemind-web',
            'docker buildx build --builder=remote --platform=linux/amd64 --build-arg GOPROXY=' + goproxy + ' --build-arg VERSION=' + version + ' -f Dockerfile.web --tag $EXPECTED_REF --push .',
            deps=[
                './web',
                './internal',
                './api',
                './configs',
                './go.mod',
                './go.sum',
            ],
            env={'BUILDKIT_HOST': buildkit_host},
            skips_local_docker=True,
        )
    else:
        docker_build(
            'hivemind-web',
            '.',
            dockerfile='Dockerfile.web',
            build_args={
                'GOPROXY': goproxy,
                'VERSION': version,
            },
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
