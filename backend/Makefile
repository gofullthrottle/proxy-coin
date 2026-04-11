BINARY_DIR := bin
MODULE     := github.com/gofullthrottle/proxy-coin/backend

.PHONY: build test lint proto migrate-up migrate-down docker clean

## build: Compile all three service binaries into ./bin/
build:
	@mkdir -p $(BINARY_DIR)
	go build -o $(BINARY_DIR)/orchestrator ./cmd/orchestrator
	go build -o $(BINARY_DIR)/api          ./cmd/api
	go build -o $(BINARY_DIR)/metering     ./cmd/metering

## test: Run the full test suite with race detection
test:
	go test -race -count=1 ./...

## lint: Run golangci-lint across all packages
lint:
	golangci-lint run ./...

## proto: Regenerate Go code from .proto definitions
proto:
	@bash ../scripts/generate-proto.sh

## migrate-up: Apply all pending database migrations
migrate-up:
	migrate -path migrations -database "$(DATABASE_DSN)" up

## migrate-down: Roll back the most recent migration
migrate-down:
	migrate -path migrations -database "$(DATABASE_DSN)" down 1

## docker: Build the multi-stage Docker image for all services
docker:
	docker build -t proxy-coin-backend:latest .

## clean: Remove compiled binaries and build artifacts
clean:
	rm -rf $(BINARY_DIR)
