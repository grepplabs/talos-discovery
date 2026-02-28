GRPCURL          ?= grpcurl
GRPC_HOST        ?= localhost:$(TEST_SERVER_PORT)
GRPC_IMPORT_PATH ?= $(ROOT_DIR)/test/grpc
GRPC_PROTO       ?= cluster.proto
GRPC_PROTO_FLAGS := -import-path $(GRPC_IMPORT_PATH) -proto $(GRPC_PROTO)

# Toggle TLS by setting TLS=1 when calling make
ifdef TLS
GRPC_SECURITY_FLAGS := -cacert $(TEST_CERT_DIR)/ca.pem
else
GRPC_SECURITY_FLAGS := -plaintext
endif

get-proto: ## get cluster.proto
	curl https://raw.githubusercontent.com/siderolabs/discovery-api/refs/heads/main/api/v1alpha1/server/cluster.proto -o $(GRPC_IMPORT_PATH)/cluster.proto
	sed -i 's|vendor/google/duration.proto|google/protobuf/duration.proto|' $(GRPC_IMPORT_PATH)/cluster.proto

.PHONY: example-health
example-health: ## grpc health check (use TLS=1 for TLS)
	$(GRPCURL) $(GRPC_SECURITY_FLAGS) \
		-d '{"service":"sidero.discovery.server.Cluster"}' \
		$(GRPC_HOST) grpc.health.v1.Health/Check

.PHONY: example-hello
example-hello: ## hello (use TLS=1 for TLS)
	$(GRPCURL) $(GRPC_SECURITY_FLAGS) $(GRPC_PROTO_FLAGS) \
		-d '{"clusterId":"abc","clientVersion":"v1.10.1"}' -H 'X-Real-IP: 1.2.3.4' \
		$(GRPC_HOST) sidero.discovery.server.Cluster/Hello

.PHONY: example-list
example-list: ## list (use TLS=1 for TLS)
	$(GRPCURL) $(GRPC_SECURITY_FLAGS) $(GRPC_PROTO_FLAGS) \
		-d '{"clusterId":"abc"}' \
		$(GRPC_HOST) sidero.discovery.server.Cluster/List

.PHONY: example-watch
example-watch: ## watch (use TLS=1 for TLS)
	$(GRPCURL) $(GRPC_SECURITY_FLAGS) $(GRPC_PROTO_FLAGS) \
		-d '{"clusterId":"abc"}' \
		$(GRPC_HOST) sidero.discovery.server.Cluster/Watch

.PHONY: example-update-node-1
example-update-node-1: ## affiliate update node-1 (use TLS=1 for TLS)
	$(GRPCURL) $(GRPC_SECURITY_FLAGS) $(GRPC_PROTO_FLAGS) \
		-d '{"clusterId":"abc","affiliateId":"node-1","affiliateData":"dGVzdA==","ttl":"300s"}' \
		$(GRPC_HOST) sidero.discovery.server.Cluster/AffiliateUpdate

.PHONY: example-update-node-2
example-update-node-2: ## affiliate update node-2 (use TLS=1 for TLS)
	$(GRPCURL) $(GRPC_SECURITY_FLAGS) $(GRPC_PROTO_FLAGS) \
		-d '{"clusterId":"abc","affiliateId":"node-2","affiliateData":"dGVzdA==","ttl":"300s","affiliateEndpoints":["bm9kZS0yLWVwLTEK","bm9kZS0yLWVwLTIK"]}' \
		$(GRPC_HOST) sidero.discovery.server.Cluster/AffiliateUpdate

.PHONY: example-delete-node-1
example-delete-node-1: ## affiliate delete node-1 (use TLS=1 for TLS)
	$(GRPCURL) $(GRPC_SECURITY_FLAGS) $(GRPC_PROTO_FLAGS) \
		-d '{"clusterId":"abc","affiliateId":"node-1"}' \
		$(GRPC_HOST) sidero.discovery.server.Cluster/AffiliateDelete

.PHONY: example-delete-node-2
example-delete-node-2: ## affiliate delete node-2 (use TLS=1 for TLS)
	$(GRPCURL) $(GRPC_SECURITY_FLAGS) $(GRPC_PROTO_FLAGS) \
		-d '{"clusterId":"abc","affiliateId":"node-2"}' \
		$(GRPC_HOST) sidero.discovery.server.Cluster/AffiliateDelete