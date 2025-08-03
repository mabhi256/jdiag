.PHONY: build clean install-deps compile-ts build-go test format dev install help

# Detect OS and set binary name
ifeq ($(OS),Windows_NT)
    BINARY_NAME = jdiag.exe
    RM_CMD = del /Q
    RM_DIR_CMD = rmdir /S /Q
else
    BINARY_NAME = jdiag
    RM_CMD = rm -f
    RM_DIR_CMD = rm -rf
endif

# Default target
all: build

# Install dependencies
install-deps:
	@echo "[DEPS] Installing dependencies..."
	@if ! command -v tsc >/dev/null 2>&1; then \
		echo "Installing TypeScript..."; \
		npm install -g typescript; \
	fi
	@if [ -f package.json ]; then \
		npm install; \
	fi

# Format and lint code
format:
	@echo "[FORMAT] Formatting and linting code..."
	@npm run format
	@npm run lint
	@echo "[FORMAT] Code formatting and linting complete"

# TypeScript compilation with change detection
internal/html/dist/app.js: internal/html/templates/app.ts
	@echo "[TS] Source changed, compiling TypeScript..."
	@npm run build
	@echo "[TS] TypeScript compiled successfully"

# Compile TypeScript to JavaScript (force compilation)
compile-ts:
	@echo "[TS] Force compiling TypeScript..."
	@npm run build
	@echo "[TS] TypeScript compiled successfully"

# Build Go binary
build-go:
	@echo "[GO] Building Go binary..."
	@go build -o $(BINARY_NAME) .

# Full build process (optimized - only compiles TS if changed)
build: internal/html/dist/app.js build-go
	@echo "[DONE] Build complete!"

# Clean generated files
clean:
	@echo "🧹 Cleaning..."
ifeq ($(OS),Windows_NT)
	@if exist internal\html\dist\app.js $(RM_CMD) internal\html\dist\app.js
	@if exist internal\html\dist\app.js.map $(RM_CMD) internal\html\dist\app.js.map
	@if exist $(BINARY_NAME) $(RM_CMD) $(BINARY_NAME)
	@if exist node_modules $(RM_DIR_CMD) node_modules
else
	@$(RM_CMD) internal/html/dist/app.js internal/html/dist/app.js.map
	@$(RM_CMD) $(BINARY_NAME)
	@$(RM_DIR_CMD) node_modules
endif

# Development mode with TypeScript watching
dev:
	@echo "🔧 Starting development mode..."
	@tsc internal/html/templates/app.ts --outDir internal/html/dist/ --target ES2017 --lib ES2017,DOM --strict --watch &
	@echo "TypeScript compiler watching for changes..."

# Test the application
test:
	@echo "[TEST] Running tests..."
	@go test ./...

# Install for development
install: install-deps
	@echo "[DEV] Setting up development environment..."
	@go mod tidy
	@echo "[DEV] Development environment ready"

# Help
help:
	@echo "Available targets:"
	@echo "  build      - Full build (TypeScript + Go) - only compiles TS if changed"
	@echo "  compile-ts - Compile TypeScript only"
	@echo "  build-go   - Build Go binary only" 
	@echo "  format     - Format and lint code using prettier + eslint"
	@echo "  clean      - Clean generated files"
	@echo "  dev        - Development mode with TypeScript watching"
	@echo "  test       - Run tests"
	@echo "  install    - Install dependencies and setup dev environment"
	@echo "  help       - Show this help"