.PHONY: all build run clean tidy test test-race test-dashboard test-e2e test-integration test-contract test-all lint lint-fix record-api swagger docs-openapi install-tools perf-check perf-bench infra image

all: build

# Get version info
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse --short HEAD)
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
DOCS_API_SERVERS ?= https://gomodel.example.com,http://localhost:8080
LOG_LEVEL ?= debug
SWAGGER_ENABLED ?= true

PROJECT_TMP_INFO := $(shell ./scripts/project-tmp.sh)
PROJECT_TMP_ROOT := $(patsubst PROJECT_TMP_ROOT=%,%,$(filter PROJECT_TMP_ROOT=%,$(PROJECT_TMP_INFO)))
PROJECT_CACHE_ROOT := $(patsubst PROJECT_CACHE_ROOT=%,%,$(filter PROJECT_CACHE_ROOT=%,$(PROJECT_TMP_INFO)))
PROJECT_BUILD_ROOT := $(patsubst PROJECT_BUILD_ROOT=%,%,$(filter PROJECT_BUILD_ROOT=%,$(PROJECT_TMP_INFO)))
PROJECT_RUNS_ROOT := $(patsubst PROJECT_RUNS_ROOT=%,%,$(filter PROJECT_RUNS_ROOT=%,$(PROJECT_TMP_INFO)))
GO_CACHE_ROOT ?= $(PROJECT_CACHE_ROOT)/go
GOCACHE ?= $(GO_CACHE_ROOT)/build
GOMODCACHE ?= $(GO_CACHE_ROOT)/mod
TMPDIR := $(PROJECT_RUNS_ROOT)/make/tmp
GOTOOLCHAIN ?= local
export PROJECT_TMP_ROOT PROJECT_CACHE_ROOT PROJECT_BUILD_ROOT PROJECT_RUNS_ROOT GOCACHE GOMODCACHE TMPDIR GOTOOLCHAIN

GO_ENV := PROJECT_TMP_ROOT=$(PROJECT_TMP_ROOT) PROJECT_CACHE_ROOT=$(PROJECT_CACHE_ROOT) PROJECT_BUILD_ROOT=$(PROJECT_BUILD_ROOT) PROJECT_RUNS_ROOT=$(PROJECT_RUNS_ROOT) GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) TMPDIR=$(TMPDIR) GOTOOLCHAIN=$(GOTOOLCHAIN)

# Linker flags to inject version info
LDFLAGS := -X "gomodel/internal/version.Version=$(VERSION)" \
           -X "gomodel/internal/version.Commit=$(COMMIT)" \
           -X "gomodel/internal/version.Date=$(DATE)"

ensure-tmp:
	@test -n "$(PROJECT_TMP_ROOT)" || { echo "failed to resolve PROJECT_TMP_ROOT" >&2; exit 1; }
	@./scripts/project-tmp.sh >/dev/null
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)" "$(PROJECT_BUILD_ROOT)" "$(PROJECT_RUNS_ROOT)" "$(TMPDIR)"

install-tools: ensure-tmp
	@command -v golangci-lint > /dev/null 2>&1 || (echo "Installing golangci-lint..." && $(GO_ENV) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10)
	@command -v pre-commit > /dev/null 2>&1 || (echo "Installing pre-commit..." && TMPDIR="$(TMPDIR)" pip install pre-commit==4.5.1)
	@echo "All tools are ready"

build: ensure-tmp
	$(GO_ENV) go build -ldflags '$(LDFLAGS)' -o bin/gomodel ./cmd/gomodel
# Run the application
run: ensure-tmp
	LOG_LEVEL=$(LOG_LEVEL) SWAGGER_ENABLED=$(SWAGGER_ENABLED) $(GO_ENV) go run -tags=swagger ./cmd/gomodel

# Clean build artifacts
clean:
	rm -rf bin/

# Tidy dependencies
tidy: ensure-tmp
	$(GO_ENV) go mod tidy

# Docker Compose: Redis, PostgreSQL, MongoDB, Adminer (no app image build)
infra:
	docker compose up -d

# Docker Compose: full stack (GoModel + Prometheus; builds app image when needed)
image:
	docker compose --profile app up -d

# Run unit tests only
test: ensure-tmp
	$(GO_ENV) go test ./cmd/... ./internal/... ./config/... -v

# Run unit tests with race detection and coverage
test-race: ensure-tmp
	$(GO_ENV) go test -v -race -coverprofile=$(PROJECT_BUILD_ROOT)/coverage.out ./cmd/... ./internal/... ./config/...

# Run dashboard JavaScript unit tests
test-dashboard:
	node --test internal/admin/dashboard/static/js/modules/*.test.cjs

# Run e2e tests (uses an in-process mock LLM server; no Docker required)
test-e2e: ensure-tmp
	$(GO_ENV) go test -v -tags=e2e ./tests/e2e/...

# Run integration tests (requires Docker)
test-integration: ensure-tmp
	$(GO_ENV) go test -v -tags=integration -timeout=10m ./tests/integration/...

# Run contract tests (validates API response structures against golden files)
test-contract: ensure-tmp
	$(GO_ENV) go test -v -tags=contract -timeout=5m ./tests/contract/...

# Run all tests including dashboard, e2e, integration, and contract tests
test-all: test test-dashboard test-e2e test-integration test-contract

perf-check: ensure-tmp
	$(GO_ENV) go test -run '^TestHotPathPerfGuard$$' -count=1 -v ./tests/perf/...

perf-bench: ensure-tmp
	$(GO_ENV) go test -bench=. -benchmem ./tests/perf/...

# Record API responses for contract tests
# Usage: OPENAI_API_KEY=sk-xxx make record-api
record-api: ensure-tmp
	@echo "Recording OpenAI chat completion..."
	$(GO_ENV) go run ./cmd/recordapi -provider=openai -endpoint=chat \
		-output=tests/contract/testdata/openai/chat_completion.json
	@echo "Recording OpenAI models..."
	$(GO_ENV) go run ./cmd/recordapi -provider=openai -endpoint=models \
		-output=tests/contract/testdata/openai/models.json
	@echo "Done! Golden files saved to tests/contract/testdata/"

swagger: ensure-tmp
	$(GO_ENV) go run github.com/swaggo/swag/v2/cmd/swag init --generalInfo main.go \
		--dir cmd/gomodel,internal \
		--output cmd/gomodel/docs \
		--outputTypes go \
		--parseDependency
	$(MAKE) docs-openapi

docs-openapi: ensure-tmp
	@command -v node >/dev/null 2>&1 || { echo "node is required to build docs; install from https://nodejs.org" >&2; exit 1; }
	@command -v npx >/dev/null 2>&1 || { echo "npx is required; install npm (includes npx)" >&2; exit 1; }
	@tmp_dir=$$(mktemp -d "$(PROJECT_RUNS_ROOT)/docs-openapi.XXXXXX"); \
	trap 'rm -rf "$$tmp_dir"' EXIT; \
	$(GO_ENV) go run github.com/swaggo/swag/v2/cmd/swag init --quiet --generalInfo main.go \
		--dir cmd/gomodel,internal \
		--output "$$tmp_dir" \
		--outputTypes json \
		--parseDependency; \
	npx -y swagger2openapi@7.0.8 --patch -o docs/openapi.json "$$tmp_dir/swagger.json"; \
	DOCS_API_SERVERS="$(DOCS_API_SERVERS)" node tools/openapi-postprocess.mjs docs/openapi.json

# Run linter
lint: ensure-tmp
	$(GO_ENV) golangci-lint run --build-tags=swagger,e2e,integration,contract ./cmd/... ./config/... ./internal/... ./tests/...

# Run linter with auto-fix
lint-fix: ensure-tmp
	$(GO_ENV) golangci-lint run --fix ./cmd/... ./config/... ./internal/...
