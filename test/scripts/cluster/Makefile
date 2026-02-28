.ONESHELL:
SHELL := /bin/bash

CLUSTER_NAME            := talos-default

DISCOVERY_API_PORT      := 3000
DISCOVERY_LANDING_PORT  := 3001

DISCOVERY_CONTAINER     := talos-discovery
DISCOVERY_IMAGE         := local/talos-discovery:latest

# docker0 gateway IP (override if different)
#   ip -4 addr show docker0 | awk '/inet /{print $2}' | cut -d/ -f1
DISCOVERY_HOST          := 172.17.0.1
DISCOVERY_ENDPOINT      := http://$(DISCOVERY_HOST):$(DISCOVERY_API_PORT)

prepare:
	sudo modprobe br_netfilter
	#echo 1 | sudo tee /proc/sys/net/bridge/bridge-nf-call-iptables
    #echo 1 | sudo tee /proc/sys/net/ipv4/ip_forward

check:
	@command -v docker >/dev/null 2>&1 || (echo "ERROR: docker not found" && exit 1)
	@command -v talosctl >/dev/null 2>&1 || (echo "ERROR: talosctl not found" && exit 1)

up: start-discovery start-talos

up-tls: start-discovery-tls start-talos-tls

down: destroy-talos stop-discovery

start-ui:
	google-chrome http://127.0.0.1:$(DISCOVERY_LANDING_PORT)

start-talos:
	talosctl config contexts
	talosctl cluster create docker --config-patch-controlplanes @discovery.patch.yaml --config-patch-workers @discovery.patch.yaml

start-talos-tls:
	talosctl config contexts
	talosctl cluster create docker --config-patch-controlplanes @discovery-tls.patch.yaml --config-patch-workers @discovery-tls.patch.yaml

destroy-talos:
	@echo "Destroying Talos cluster..."
	@talosctl cluster destroy --name $(CLUSTER_NAME) >/dev/null 2>&1 || true
	@talosctl config remove -y $(CLUSTER_NAME) || true
	#@rm -f ~/.talos/clusters/$(CLUSTER_NAME)
	@mv ~/.talos/config ~/.talos/config.$$(date +%Y%m%d-%H%M%S).bak
	kubectl config unset current-context >/dev/null 2>&1 || true
	kubectl config delete-context "admin@talos-default" >/dev/null 2>&1 || true
	kubectl config delete-cluster talos-default >/dev/null 2>&1 || true
	kubectl config delete-user admin@talos-default >/dev/null 2>&1 || true

start-discovery:
	@echo "Starting discovery-service..."
	@docker rm -f $(DISCOVERY_CONTAINER) >/dev/null 2>&1 || true
	@docker run -d --name $(DISCOVERY_CONTAINER) \
		-p $(DISCOVERY_API_PORT):3000 \
		-p $(DISCOVERY_LANDING_PORT):8080 \
		$(DISCOVERY_IMAGE) service --web-enable >/dev/null
	@sleep 1
	@echo "Discovery API (host):      http://127.0.0.1:$(DISCOVERY_API_PORT)"
	@echo "Discovery UI  (host):      http://127.0.0.1:$(DISCOVERY_LANDING_PORT)/"
	@echo "Discovery API (docker0):   $(DISCOVERY_ENDPOINT)"
	@$(MAKE) curl-ui >/dev/null || true

start-discovery-tls:
	@echo "Starting TLS discovery-service..."
	@docker rm -f $(DISCOVERY_CONTAINER) >/dev/null 2>&1 || true
	@docker run -d --name $(DISCOVERY_CONTAINER) \
		-p $(DISCOVERY_API_PORT):3000 \
		-p $(DISCOVERY_LANDING_PORT):8080 \
		--user root \
		-v $(PWD)/discovery-service-server.pem:/etc/discovery/tls.crt:ro \
		-v $(PWD)/discovery-service-server-key.pem:/etc/discovery/tls.key:ro \
		$(DISCOVERY_IMAGE) service --web-enable \
		--server-tls-enable \
		--server-tls-file-cert=/etc/discovery/tls.crt \
		--server-tls-file-key=/etc/discovery/tls.key\
		>/dev/null
	@sleep 1
	@echo "Discovery API (host):      https://127.0.0.1:$(DISCOVERY_API_PORT)"
	@echo "Discovery UI  (host):      http://127.0.0.1:$(DISCOVERY_LANDING_PORT)/"
	@$(MAKE) curl-ui >/dev/null || true

stop-discovery:
	@docker rm -f $(DISCOVERY_CONTAINER) >/dev/null 2>&1 || true

curl-ui:
	@curl -sS http://127.0.0.1:$(DISCOVERY_LANDING_PORT) | head -n 30

logs:
	@docker logs -f $(DISCOVERY_CONTAINER)

talos-logs:
	@talosctl -n 10.5.0.2 logs controller-runtime

talos-list-certs:
	@talosctl -n 10.5.0.2 ls /etc/ssl/certs/

talos-read-certs:
	@talosctl -n 10.5.0.2 read /etc/ssl/certs/ca-certificates.crt

talos-read-certs-subjects:
	@talosctl -n 10.5.0.2 read /etc/ssl/certs/ca-certificates.crt | openssl storeutl -noout -text /dev/stdin | grep "Subject:"

ports:
	@docker port $(DISCOVERY_CONTAINER) || true

curl-docker-api:
	@docker run --rm alpine sh -c 'apk add --no-cache curl >/dev/null && curl -sS http://$(DISCOVERY_HOST):$(DISCOVERY_API_PORT) | head -n 3'

health:
	@talosctl health

members:
	@talosctl get members -n 127.0.0.1

discovery-check:
	talosctl -c talos-default config info
	talosctl -c talos-default config node 10.5.0.2
	talosctl -c talos-default get affiliates

	talosctl get members --nodes 10.5.0.2
	talosctl get identities --nodes 10.5.0.2
	talosctl get identities --nodes 10.5.0.3

machineconfig:
	talosctl -c talos-default get machineconfig
	talosctl -c talos-default get machineconfig persistent --nodes 10.5.0.3 -o yaml | yq -r '.spec' | less

