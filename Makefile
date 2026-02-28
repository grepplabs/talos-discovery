SHELL := /usr/bin/env bash
.SHELLFLAGS += -o pipefail -O extglob
.DEFAULT_GOAL := help

ROOT_DIR       = $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
VERSION       ?= $(shell git describe --tags --always --dirty)
LDFLAGS       ?= -X github.com/grepplabs/talos-discovery/internal/config.Version=$(VERSION)

GOLANGCI_LINT_VERSION := v2.10.1
BUF_VERSION           := v1.66.0
PROTO_FILES     := $(shell find internal/proto -name '*.proto')
PROTO_GEN_FILES := $(patsubst %.proto,%.pb.go,$(PROTO_FILES))


DOCKER_BUILD_ARGS ?=
LOCAL_IMAGE := local/talos-discovery:latest

TEST_SERVER_PORT ?= 3000
TEST_CERT_DIR    ?= $(ROOT_DIR)/test/scripts/certs/output

##@ General

.PHONY: help
help: ## display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


## Tool Binaries
GO_RUN := go run
GOLANGCI_LINT ?= $(GO_RUN) github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
BUF           ?= $(GO_RUN) github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)

.PHONY: lint
lint: ## run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: ## run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

.PHONY: lint-config
lint-config: ## verify golangci-lint linter configuration
	$(GOLANGCI_LINT) config verify


##@ Generate

.PHONY: buf_lint
buf_lint: $(PROTO_FILES) ## lint proto files
	$(BUF) lint

$(PROTO_GEN_FILES) &: $(PROTO_FILES) buf.yaml buf.gen.yaml
	$(BUF) generate

.PHONY: generate ## generate code
generate: $(PROTO_GEN_FILES)


##@ Development

.PHONY: fmt
fmt: ## run go fmt against code
	go fmt ./...

.PHONY: vet
vet: ## run go vet against code
	go vet ./...

.PHONY: tidy
tidy: ## run go mod tidy
	go mod tidy

##@ Build

.PHONY: build
build: ## build binary
	CGO_ENABLED=0 go build -gcflags "all=-N -l" -ldflags "$(LDFLAGS)" -o ./discovery-service ./cmd/discovery-service/main.go

.PHONY: clean
clean: ## clean
	@rm -f ./state.binpb
	@rm -f ./discovery-service

##@ Docker

.PHONY: docker-build
docker-build: ## build docker image
	docker build --build-arg VERSION=$(VERSION) $(DOCKER_BUILD_ARGS) -t ${LOCAL_IMAGE} .

##@ Run targets
run-service: ## run discovery service
	SERVER_ADDR=:$(TEST_SERVER_PORT) LOG_LEVEL=debug go run cmd/discovery-service/main.go service --discovery-snapshot-path=./state.bin --discovery-snapshot-path=./state.binpb

run-service-tls: ## run TLS discovery service
	SERVER_ADDR=:$(TEST_SERVER_PORT) LOG_LEVEL=debug go run cmd/discovery-service/main.go service --discovery-snapshot-path=./state.bin --discovery-snapshot-path=./state.binpb \
		--server-tls-enable \
		--server-tls-refresh=10s \
		--server-tls-file-key=$(TEST_CERT_DIR)/discovery-service-server-key.pem \
		--server-tls-file-cert=$(TEST_CERT_DIR)/discovery-service-server.pem

run-service-web: ## run discovery service and web
	SERVER_ADDR=:$(TEST_SERVER_PORT) LOG_LEVEL=debug go run cmd/discovery-service/main.go service --discovery-snapshot-path=./state.bin --discovery-snapshot-path=./state.binpb --web-enable

run-service-web-tls: ## run TLS discovery service and web
	SERVER_ADDR=:$(TEST_SERVER_PORT) LOG_LEVEL=debug go run cmd/discovery-service/main.go service --discovery-snapshot-path=./state.bin --discovery-snapshot-path=./state.binpb --web-enable  \
		--server-tls-enable \
		--server-tls-refresh=10s \
		--server-tls-file-key=$(TEST_CERT_DIR)/discovery-service-server-key.pem \
		--server-tls-file-cert=$(TEST_CERT_DIR)/discovery-service-server.pem

run-web: ## run discovery web
	LOG_LEVEL=debug go run cmd/discovery-service/main.go web

run-web-tls: ## run discovery web (TLS client)
	LOG_LEVEL=debug go run cmd/discovery-service/main.go web \
		--web-discovery-client-tls-enable \
		--web-discovery-client-tls-file-root-ca=$(TEST_CERT_DIR)/ca.pem

##@ Test targets

.PHONY: test
test: ## run tests
	go test -v -race -count=1 ./...

.PHONY: test-ui
test-ui: export CHROMEDP_E2E=1
test-ui: ## run ui tests
	go test -v -race -count=1 -run '^TestUI' ./internal/web/...


##@ Examples targets

include test/scripts/mk/examples.mk
