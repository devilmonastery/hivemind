.PHONY: all build clean server cli web test lint proto help

# Load .env file if it exists
-include .env
export

# Binary names
SERVER_BIN := bin/hivemind-server
CLI_BIN := bin/hivemind
WEB_BIN := bin/hivemind-web

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod

all: build

## build: Build all binaries
build: server cli web

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
	@$(GOBUILD) -o $(WEB_BIN) ./web

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -f $(SERVER_BIN) $(CLI_BIN) $(WEB_BIN)

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

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
