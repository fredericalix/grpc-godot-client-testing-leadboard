.PHONY: help proto sqlc migrate-up migrate-down migrate-create build run test clean \
        compose-up compose-down compose-logs compose-build dev-db lint fmt vet \
        install-tools proto-lint client server

# Configuration
DATABASE_URL ?= postgres://leaderboard:leaderboard@localhost:5432/leaderboard?sslmode=disable
MIGRATION_DIR = db/migrations
PROTO_DIR = proto
BIN_DIR = bin

# Colors for output
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
RESET  := $(shell tput -Txterm sgr0)

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  ${GREEN}%-18s${RESET} %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## Code Generation

proto: ## Generate Go code from protobuf definitions using Buf
	@echo "${GREEN}Generating protobuf code...${RESET}"
	buf lint
	buf generate
	@echo "${GREEN}✓ Protobuf code generated${RESET}"

proto-lint: ## Lint protobuf files
	@echo "${GREEN}Linting protobuf files...${RESET}"
	buf lint
	@echo "${GREEN}✓ Protobuf lint passed${RESET}"

sqlc: ## Generate Go code from SQL queries using sqlc
	@echo "${GREEN}Generating sqlc code...${RESET}"
	cd db/sql && sqlc generate
	@echo "${GREEN}✓ sqlc code generated${RESET}"

swagger: ## Generate OpenAPI/Swagger documentation
	@echo "${GREEN}Generating Swagger documentation...${RESET}"
	swag init -g internal/transport/rest/server.go -o docs --parseDependency --parseInternal
	@echo "${GREEN}✓ Swagger docs generated${RESET}"

generate: proto sqlc swagger ## Generate all code (proto + sqlc + swagger)

## Database Migrations

migrate-up: ## Run all database migrations up
	@echo "${GREEN}Running migrations up...${RESET}"
	migrate -path $(MIGRATION_DIR) -database "$(DATABASE_URL)" up
	@echo "${GREEN}✓ Migrations applied${RESET}"

migrate-down: ## Rollback last database migration
	@echo "${YELLOW}Rolling back last migration...${RESET}"
	migrate -path $(MIGRATION_DIR) -database "$(DATABASE_URL)" down 1

migrate-force: ## Force migration to specific version (usage: make migrate-force VERSION=1)
	@echo "${YELLOW}Forcing migration to version $(VERSION)...${RESET}"
	migrate -path $(MIGRATION_DIR) -database "$(DATABASE_URL)" force $(VERSION)

migrate-version: ## Show current migration version
	@migrate -path $(MIGRATION_DIR) -database "$(DATABASE_URL)" version

migrate-create: ## Create a new migration file (usage: make migrate-create NAME=add_users)
	@if [ -z "$(NAME)" ]; then \
		echo "${YELLOW}Usage: make migrate-create NAME=migration_name${RESET}"; \
		exit 1; \
	fi
	@migrate create -ext sql -dir $(MIGRATION_DIR) -seq $(NAME)
	@echo "${GREEN}✓ Migration created${RESET}"

## Build & Run

build: ## Build the server binary
	@echo "${GREEN}Building server...${RESET}"
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/server ./cmd/server
	@echo "${GREEN}✓ Server built: $(BIN_DIR)/server${RESET}"

server: build ## Build and run the server
	@echo "${GREEN}Starting server...${RESET}"
	$(BIN_DIR)/server

client: ## Build the gRPC client
	@echo "${GREEN}Building client...${RESET}"
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/client ./cmd/client
	@echo "${GREEN}✓ Client built: $(BIN_DIR)/client${RESET}"

run: build ## Run the server (without Docker)
	@echo "${GREEN}Running server...${RESET}"
	$(BIN_DIR)/server

## Testing

test: ## Run all tests
	@echo "${GREEN}Running tests...${RESET}"
	go test -v -race -coverprofile=coverage.out ./...
	@echo "${GREEN}✓ Tests passed${RESET}"

test-coverage: test ## Run tests with coverage report
	@echo "${GREEN}Generating coverage report...${RESET}"
	go tool cover -html=coverage.out -o coverage.html
	@echo "${GREEN}✓ Coverage report generated: coverage.html${RESET}"

test-integration: ## Run integration tests only
	@echo "${GREEN}Running integration tests...${RESET}"
	go test -v -race -tags=integration ./...

## Docker Compose

compose-up: ## Start all services with Docker Compose
	@echo "${GREEN}Starting services...${RESET}"
	docker compose up -d
	@echo "${GREEN}✓ Services started${RESET}"
	@echo "${YELLOW}Waiting for database to be ready...${RESET}"
	@sleep 3
	@echo "${GREEN}Running migrations...${RESET}"
	@docker compose exec -T postgres sh -c 'until pg_isready -U leaderboard; do sleep 1; done'
	@$(MAKE) migrate-up
	@echo "${GREEN}✓ System ready${RESET}"

compose-down: ## Stop all services
	@echo "${YELLOW}Stopping services...${RESET}"
	docker compose down
	@echo "${GREEN}✓ Services stopped${RESET}"

compose-down-volumes: ## Stop all services and remove volumes
	@echo "${YELLOW}Stopping services and removing volumes...${RESET}"
	docker compose down -v
	@echo "${GREEN}✓ Services stopped and volumes removed${RESET}"

compose-logs: ## Show logs from all services
	docker compose logs -f

compose-build: ## Build Docker images
	@echo "${GREEN}Building Docker images...${RESET}"
	docker compose build
	@echo "${GREEN}✓ Images built${RESET}"

compose-restart: compose-down compose-up ## Restart all services

## Development

dev-db: ## Start only PostgreSQL for local development
	@echo "${GREEN}Starting PostgreSQL...${RESET}"
	docker compose up -d postgres
	@echo "${GREEN}✓ PostgreSQL started${RESET}"
	@echo "${YELLOW}Waiting for database to be ready...${RESET}"
	@./scripts/dev-wait-for-db.sh
	@echo "${GREEN}Running migrations...${RESET}"
	@$(MAKE) migrate-up
	@echo "${GREEN}✓ Database ready${RESET}"

dev: dev-db generate ## Set up local development environment
	@echo "${GREEN}✓ Development environment ready${RESET}"

## Code Quality

fmt: ## Format Go code
	@echo "${GREEN}Formatting code...${RESET}"
	go fmt ./...
	@echo "${GREEN}✓ Code formatted${RESET}"

vet: ## Run go vet
	@echo "${GREEN}Running go vet...${RESET}"
	go vet ./...
	@echo "${GREEN}✓ go vet passed${RESET}"

lint: ## Run staticcheck linter
	@echo "${GREEN}Running staticcheck...${RESET}"
	@if ! command -v staticcheck > /dev/null; then \
		echo "${YELLOW}staticcheck not found, installing...${RESET}"; \
		go install honnef.co/go/tools/cmd/staticcheck@latest; \
	fi
	staticcheck ./...
	@echo "${GREEN}✓ Linting passed${RESET}"

check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)

## Utilities

clean: ## Clean build artifacts and generated files
	@echo "${GREEN}Cleaning...${RESET}"
	rm -rf $(BIN_DIR)
	rm -rf gen/
	rm -rf docs/
	rm -f coverage.out coverage.html
	@echo "${GREEN}✓ Cleaned${RESET}"

install-tools: ## Install required development tools
	@echo "${GREEN}Installing development tools...${RESET}"
	@echo "Installing buf..."
	@go install github.com/bufbuild/buf/cmd/buf@latest
	@echo "Installing sqlc..."
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	@echo "Installing migrate..."
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "Installing staticcheck..."
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@echo "Installing swag..."
	@go install github.com/swaggo/swag/cmd/swag@latest
	@echo "${GREEN}✓ Tools installed${RESET}"

deps: ## Download Go module dependencies
	@echo "${GREEN}Downloading dependencies...${RESET}"
	go mod download
	go mod tidy
	@echo "${GREEN}✓ Dependencies downloaded${RESET}"

## Quick Start

quickstart: install-tools deps generate compose-up ## Complete setup from scratch
	@echo "${GREEN}✓✓✓ Leaderboard is ready! ✓✓✓${RESET}"
	@echo ""
	@echo "gRPC server: localhost:50051"
	@echo "REST API:    http://localhost:8080"
	@echo ""
	@echo "Try the client: make client && ./bin/client -cmd stream"
	@echo "Or use grpcurl: grpcurl -plaintext localhost:50051 list"
