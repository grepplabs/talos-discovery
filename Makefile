SHELL := /usr/bin/env bash
.SHELLFLAGS += -o pipefail -O extglob
.DEFAULT_GOAL := help

ROOT_DIR       = $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
VERSION       ?= $(shell git describe --tags --always --dirty)

GOLANGCI_LINT_VERSION := v2.10.1
BUF_VERSION           := v1.66.0
PROTO_FILES     := $(shell find internal/proto -name '*.proto')
PROTO_GEN_FILES := $(patsubst %.proto,%.pb.go,$(PROTO_FILES))

TEST_SERVER_PORT ?= 3000

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


##@ Run targets
run-service: ## run discovery service
	SERVER_ADDR=:$(TEST_SERVER_PORT) LOG_LEVEL=debug go run cmd/discovery-service/main.go --discovery-snapshot-path=./state.bin --discovery-snapshot-path=./state.binpb


##@ Test targets

.PHONY: test
test: ## run tests
	go test -v -race -count=1 ./...


##@ Examples targets

get-proto: ## get cluster.proto
	curl https://raw.githubusercontent.com/siderolabs/discovery-api/refs/heads/main/api/v1alpha1/server/cluster.proto -o test/grpc/cluster.proto
	sed -i 's|vendor/google/duration.proto|google/protobuf/duration.proto|' test/grpc/cluster.proto


example-health:
	grpcurl -plaintext -d '{"service": "sidero.discovery.server.Cluster"}' \
		localhost:$(TEST_SERVER_PORT) grpc.health.v1.Health/Check

example-hello:
	grpcurl -plaintext -import-path test/grpc -proto cluster.proto \
		-d '{"clusterId": "abc", "clientVersion": "v1.10.1"}' -H 'X-Real-IP: 1.2.3.4' \
		localhost:$(TEST_SERVER_PORT) sidero.discovery.server.Cluster/Hello

example-list:
	grpcurl -plaintext -import-path test/grpc -proto cluster.proto \
		-d '{"clusterId": "abc"}' \
		localhost:$(TEST_SERVER_PORT) sidero.discovery.server.Cluster/List

example-watch:
	grpcurl -plaintext -import-path test/grpc -proto cluster.proto \
		-d '{"clusterId": "abc"}' \
		localhost:$(TEST_SERVER_PORT) sidero.discovery.server.Cluster/Watch

example-update-node-1:
	grpcurl -plaintext -import-path test/grpc -proto cluster.proto \
        -d '{"clusterId": "abc", "affiliateId": "node-1", "affiliateData": "dGVzdA==", "ttl": "300s"}' \
        localhost:$(TEST_SERVER_PORT) sidero.discovery.server.Cluster/AffiliateUpdate

example-update-node-2:
	grpcurl -plaintext -import-path test/grpc -proto cluster.proto \
		-d '{"clusterId": "abc", "affiliateId": "node-2", "affiliateData": "dGVzdA==", "ttl": "300s", "affiliateEndpoints": ["bm9kZS0yLWVwLTEK", "bm9kZS0yLWVwLTIK"]}' \
		localhost:$(TEST_SERVER_PORT) sidero.discovery.server.Cluster/AffiliateUpdate

example-delete-node-1:
	grpcurl -plaintext -import-path test/grpc -proto cluster.proto \
		-d '{"clusterId": "abc", "affiliateId": "node-1"}' \
		localhost:$(TEST_SERVER_PORT) sidero.discovery.server.Cluster/AffiliateDelete

example-delete-node-2:
	grpcurl -plaintext -import-path test/grpc -proto cluster.proto \
		-d '{"clusterId": "abc", "affiliateId": "node-2"}' \
		localhost:$(TEST_SERVER_PORT) sidero.discovery.server.Cluster/AffiliateDelete
