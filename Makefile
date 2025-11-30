.PHONY: all build clean server cli web bot test lint proto assets build-css watch-css tidy run-server run-web run-bot db-shell docker-server docker-web docker-bot docker-all docker-publish-server docker-publish-web docker-publish-bot docker-publish-all buildx-remote-setup buildx-remote-teardown buildx-info help

# Load .env file if it exists
-include .env
export

# Binary names
SERVER_BIN := bin/hivemind-server
CLI_BIN := bin/hivemind
WEB_BIN := bin/hivemind-web
BOT_BIN := bin/hivemind-bot

# Docker configuration
DOCKER_REGISTRY := devilmonastery
DOCKER_VERSION ?= $(shell date +%Y.%m.%d.%H.%M)

# BuildKit configuration (optional - set in .env)
BUILDKIT_REMOTE_HOST ?=
BUILDX_BUILDER ?= default

# Version information
VERSION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
LDFLAGS := -X github.com/devilmonastery/hivemind/web/internal/render.Version=$(VERSION)

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod

all: build

## build: Build all binaries
build: server cli web bot

## server: Build the gRPC server
server:
	@echo "Building hivemind server..."
	@$(GOBUILD) -o $(SERVER_BIN) ./server

## cli: Build the CLI client
cli:
	@echo "Building hivemind CLI..."
	@$(GOBUILD) -o $(CLI_BIN) ./cli

## web: Build the web server
web:
	@echo "Building hivemind web..."
	@$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(WEB_BIN) ./web

## bot: Build the Discord bot
bot:
	@echo "Building hivemind bot..."
	@$(GOBUILD) -o $(BOT_BIN) ./bot

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -f $(SERVER_BIN) $(CLI_BIN) $(WEB_BIN) $(BOT_BIN)

## test: Run tests
test:
	@echo "Running tests..."
	@$(GOTEST) -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@$(GOTEST) -v -coverprofile=coverage.out ./...
	@$(GOCMD) tool cover -html=coverage.out -o coverage.html

## lint: Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run

## proto: Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@buf generate

## assets: Build all frontend assets (CSS and JS)
assets:
	@echo "Building frontend assets..."
	@npm install
	@npm run build

## build-css: Build Tailwind CSS
build-css:
	@echo "Building Tailwind CSS..."
	@npm install
	@npm run build:css

## watch-css: Watch and rebuild Tailwind CSS on changes
watch-css:
	@echo "Watching Tailwind CSS for changes..."
	@npm run build:css:watch

## tidy: Tidy go modules
tidy:
	@echo "Tidying go modules..."
	@$(GOMOD) tidy

## run-server: Build and run the server
run-server: server
	@echo "Starting hivemind server..."
	@$(SERVER_BIN) --config configs/dev-server.yaml

## run-cli: Build and run the CLI
run-cli: cli
	@echo "Starting hivemind CLI..."
	@$(CLI_BIN)

## run-web: Build and run the web server
run-web: web
	@echo "Starting hivemind web..."
	@$(WEB_BIN) --config configs/dev-web.yaml

## run-bot: Build and run the Discord bot
run-bot: bot
	@echo "Starting hivemind bot..."
	@$(BOT_BIN) run --config configs/dev-bot.yaml

## bot-register: Register bot commands (use GUILD=id for guild-specific, omit for global)
bot-register: bot
	@echo "Registering bot commands..."
ifdef GUILD
	@$(BOT_BIN) register --guild $(GUILD) --config configs/dev-bot.yaml
else
	@$(BOT_BIN) register --config configs/dev-bot.yaml
endif

## db-shell: Open PostgreSQL shell
db-shell:
	@psql postgresql://postgres:postgres@hivemind_devcontainer-postgres-1:5432/hivemind

## docker-server: Build Docker image for server (loads locally)
docker-server:
	@echo "Building Docker image for server: $(DOCKER_REGISTRY)/hivemind-server:$(DOCKER_VERSION)"
	@docker buildx build --builder default --platform linux/amd64 -f Dockerfile.server -t $(DOCKER_REGISTRY)/hivemind-server:$(DOCKER_VERSION) --load .
	@docker tag $(DOCKER_REGISTRY)/hivemind-server:$(DOCKER_VERSION) $(DOCKER_REGISTRY)/hivemind-server:latest
	@echo "Built $(DOCKER_REGISTRY)/hivemind-server:$(DOCKER_VERSION)"

## docker-web: Build Docker image for web (loads locally)
docker-web:
	@echo "Building Docker image for web: $(DOCKER_REGISTRY)/hivemind-web:$(DOCKER_VERSION)"
	@docker buildx build --builder default --platform linux/amd64 -f Dockerfile.web --build-arg VERSION=$(DOCKER_VERSION) -t $(DOCKER_REGISTRY)/hivemind-web:$(DOCKER_VERSION) --load .
	@docker tag $(DOCKER_REGISTRY)/hivemind-web:$(DOCKER_VERSION) $(DOCKER_REGISTRY)/hivemind-web:latest
	@echo "Built $(DOCKER_REGISTRY)/hivemind-web:$(DOCKER_VERSION)"

## docker-bot: Build Docker image for bot (loads locally)
docker-bot:
	@echo "Building Docker image for bot: $(DOCKER_REGISTRY)/hivemind-bot:$(DOCKER_VERSION)"
	@docker buildx build --builder default --platform linux/amd64 -f Dockerfile.bot -t $(DOCKER_REGISTRY)/hivemind-bot:$(DOCKER_VERSION) --load .
	@docker tag $(DOCKER_REGISTRY)/hivemind-bot:$(DOCKER_VERSION) $(DOCKER_REGISTRY)/hivemind-bot:latest
	@echo "Built $(DOCKER_REGISTRY)/hivemind-bot:$(DOCKER_VERSION)"

## docker-all: Build all Docker images in parallel
docker-all:
	@$(MAKE) docker-server docker-web docker-bot

## docker-publish-server: Build and push server Docker image (uses remote builder if active)
docker-publish-server:
	@echo "Building and pushing Docker image: $(DOCKER_REGISTRY)/hivemind-server:$(DOCKER_VERSION)"
	@docker buildx build --platform linux/amd64 \
		$(if $(BUILDKIT_REMOTE_HOST),--builder remote,) \
		-f Dockerfile.server \
		-t $(DOCKER_REGISTRY)/hivemind-server:$(DOCKER_VERSION) \
		-t $(DOCKER_REGISTRY)/hivemind-server:latest \
		--push .
	@echo "Pushed $(DOCKER_REGISTRY)/hivemind-server:$(DOCKER_VERSION)"

## docker-publish-web: Build and push web Docker image (uses remote builder if active)
docker-publish-web:
	@echo "Building and pushing Docker image: $(DOCKER_REGISTRY)/hivemind-web:$(DOCKER_VERSION)"
	@docker buildx build --platform linux/amd64 \
		$(if $(BUILDKIT_REMOTE_HOST),--builder remote,) \
		-f Dockerfile.web \
		--build-arg VERSION=$(DOCKER_VERSION) \
		-t $(DOCKER_REGISTRY)/hivemind-web:$(DOCKER_VERSION) \
		-t $(DOCKER_REGISTRY)/hivemind-web:latest \
		--push .
	@echo "Pushed $(DOCKER_REGISTRY)/hivemind-web:$(DOCKER_VERSION)"

## docker-publish-bot: Build and push bot Docker image (uses remote builder if active)
docker-publish-bot:
	@echo "Building and pushing Docker image: $(DOCKER_REGISTRY)/hivemind-bot:$(DOCKER_VERSION)"
	@docker buildx build --platform linux/amd64 \
		$(if $(BUILDKIT_REMOTE_HOST),--builder remote,) \
		-f Dockerfile.bot \
		-t $(DOCKER_REGISTRY)/hivemind-bot:$(DOCKER_VERSION) \
		-t $(DOCKER_REGISTRY)/hivemind-bot:latest \
		--push .
	@echo "Pushed $(DOCKER_REGISTRY)/hivemind-bot:$(DOCKER_VERSION)"

## docker-publish-all: Build and push all Docker images (uses remote builder if active)
docker-publish-all:
	@$(MAKE) docker-publish-server DOCKER_VERSION=$(DOCKER_VERSION)
	@$(MAKE) docker-publish-web DOCKER_VERSION=$(DOCKER_VERSION)
	@$(MAKE) docker-publish-bot DOCKER_VERSION=$(DOCKER_VERSION)

## buildx-remote-setup: Configure Docker buildx to use remote BuildKit (set BUILDKIT_REMOTE_HOST in .env)
buildx-remote-setup:
	@if [ -z "$(BUILDKIT_REMOTE_HOST)" ]; then \
		echo "Error: BUILDKIT_REMOTE_HOST is not set"; \
		echo ""; \
		echo "Add to your .env file:"; \
		echo "  BUILDKIT_REMOTE_HOST=tcp://HOST:PORT"; \
		echo ""; \
		echo "Or run with:"; \
		echo "  make buildx-remote-setup BUILDKIT_REMOTE_HOST=tcp://HOST:PORT"; \
		exit 1; \
	fi
	@echo "Setting up remote BuildKit builder at $(BUILDKIT_REMOTE_HOST)..."
	@docker buildx inspect remote >/dev/null 2>&1 && \
		(echo "Builder 'remote' already exists. Removing..."; docker buildx rm remote) || true
	@docker buildx create --name remote --driver remote $(BUILDKIT_REMOTE_HOST)
	@docker buildx use remote
	@docker buildx inspect --bootstrap
	@echo "✓ Remote BuildKit builder configured."

## buildx-remote-teardown: Switch back to default Docker buildx builder
buildx-remote-teardown:
	@echo "Switching to default builder..."
	@docker buildx use default || true
	@echo "✓ Using default builder. Remote builder still exists (use 'docker buildx rm remote' to remove)."

## buildx-info: Show current buildx builder information
buildx-info:
	@echo "Current builder:"
	@docker buildx inspect 2>/dev/null || echo "No builder selected"
	@echo ""
	@echo "Available builders:"
	@docker buildx ls

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
