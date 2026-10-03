VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -X github.com/ownerofglory/ondota/internal/version.Version=$(VERSION)
BIN_DIR ?= bin
HELM_IMAGE_REPOSITORY ?= example.invalid/ondota-server

# Integration tests drop and recreate the ondota schema: point this at a
# disposable database (e.g. ondota_test via `make db-port-forward`).
TEST_DATABASE_URL ?= postgres://ondota:ondota@localhost:5432/ondota_test?sslmode=disable

.PHONY: help run test test-integration coverage fmt vet build check helm-lint helm-template db-port-forward clean

help:
	@echo "Targets: run test test-integration coverage fmt vet build check helm-lint helm-template db-port-forward clean"

run:
	go run ./cmd/ondota-server

test:
	go test ./...

test-integration:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -tags=integration -race -count=1 ./...

coverage:
	go test -race -coverprofile=coverage.out ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './bin/*')

vet:
	go vet ./...

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/ondota-server ./cmd/ondota-server

check: vet test build helm-lint helm-template

helm-lint:
	helm lint charts/ondota --set image.repository=$(HELM_IMAGE_REPOSITORY)
	helm lint infra/migrations --set image.repository=$(HELM_IMAGE_REPOSITORY) --set image.tag=dev

helm-template:
	helm template ondota charts/ondota --set image.repository=$(HELM_IMAGE_REPOSITORY) --set ingress.enabled=true

# Forwards the in-cluster PostgreSQL (namespace ondota) to localhost:5432.
db-port-forward:
	kubectl -n ondota port-forward svc/ondota-postgres 5432:5432

clean:
	@rm -rf $(BIN_DIR) coverage.out
