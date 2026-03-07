# talos-discovery

[![Build](https://github.com/grepplabs/talos-discovery/actions/workflows/build.yml/badge.svg)](https://github.com/grepplabs/talos-discovery/actions/workflows/build.yml)
[![Release](https://img.shields.io/github/v/release/grepplabs/talos-discovery?sort=semver)](https://github.com/grepplabs/talos-discovery/releases)

An alternative implementation of the Talos [Discovery Service], compatible with requests from the Talos [Discovery Client].

## Overview

This project provides a drop-in alternative to the official Talos discovery components:

- [Discovery Service]
- [Discovery Client]
- [Talos Discovery]

## Build

Build the binary and print the CLI help:

```bash
make build && ./discovery-service --help
```

## Quick Start
### 1. Run Discovery Service Only

Starts the backend discovery service.

```bash
./discovery-service service
```

### 2. Run Web UI Only

Starts only the web interface and connects to an external discovery service endpoint.

```bash
./discovery-service web --web-discovery-client-target=localhost:3000
```

### 3. Run Service with Embedded Web UI

Starts the discovery service together with the embedded web UI.

```bash
./discovery-service service --web-enable
```

Then open your browser and navigate to [http://localhost:8080](http://localhost:8080)

## Help output
 ./discovery-service service --help

    Run discovery service
    
    Usage:
      discovery-service service [flags]
    
    Flags:
          --discovery-cleanup-interval duration             Interval for cleaning up expired clusters. (default 5m0s)
          --discovery-redirect-endpoint string              Redirect endpoint to include in discovery responses.
          --discovery-snapshot-interval duration            Interval between discovery state snapshots. (default 1m0s)
          --discovery-snapshot-path string                  Path to persist discovery state snapshots. Empty disables snapshots.
      -h, --help                                            help for service
          --server-addr string                              gRPC server listen address. (default ":3000")
          --server-tls-enable                               Enable server-side TLS.
          --server-tls-file-cert string                     Path to the server TLS certificate file.
          --server-tls-file-client-ca string                Path to the server client CA file for client verification.
          --server-tls-file-client-crl string               Path to the TLS X509 CRL signed by the client CA. If unspecified, only the client CA is verified.
          --server-tls-file-key string                      Path to the server TLS private key file.
          --server-tls-key-password string                  Password to decrypt RSA private key.
          --server-tls-refresh duration                     Interval for refreshing server TLS certificates. Set to 0 to disable auto-refresh.
          --web-addr string                                 Web listen address. (default ":8080")
          --web-discovery-client-target string              Discovery gRPC endpoint used by embedded web. Use ":in-memory" for in-memory transport. (default ":in-memory")
          --web-discovery-client-tls-enable                 Enable TLS configuration for the discovery client.
          --web-discovery-client-tls-file-cert string       Path to the client TLS certificate file (for mTLS).
          --web-discovery-client-tls-file-key string        Path to the client TLS private key file (for mTLS).
          --web-discovery-client-tls-file-root-ca string    Path to a custom root CA bundle for verifying the discovery server.
          --web-discovery-client-tls-insecure-skip-verify   Skip server certificate verification (insecure; use only for testing).
          --web-discovery-client-tls-key-password string    Password to decrypt RSA private key.
          --web-discovery-client-tls-refresh duration       Interval for reloading client TLS certificates. Set to 0 to disable auto-refresh.
          --web-discovery-client-tls-use-system-pool        Use the system certificate pool for verifying server certificates. (default true)
          --web-enable                                      Enable embedded web UI in service process.
          --web-tls-enable                                  Enable server-side TLS.
          --web-tls-file-cert string                        Path to the server TLS certificate file.
          --web-tls-file-client-ca string                   Path to the server client CA file for client verification.
          --web-tls-file-client-crl string                  Path to the TLS X509 CRL signed by the client CA. If unspecified, only the client CA is verified.
          --web-tls-file-key string                         Path to the server TLS private key file.
          --web-tls-key-password string                     Password to decrypt RSA private key.
          --web-tls-refresh duration                        Interval for refreshing server TLS certificates. Set to 0 to disable auto-refresh.
    
    Global Flags:
          --config string   config file (env: CONFIG)


## Talos Discovery - Docker Example

This example shows how to run the **Talos Discovery Service** locally using Docker and create a Talos cluster that registers with it.

### 1. Prepare Linux Networking (Linux Only)

On Linux systems you may need to enable the `br_netfilter` kernel module:

```bash
sudo modprobe br_netfilter
```

### 2. Start the Discovery Service

Run the discovery service container:

```bash
docker run -d --name talos-discovery \
  -p 3000:3000 -p 8080:8080 --user root \
  -v ./test/scripts/cluster/discovery-service-server.pem:/etc/discovery/tls.crt:ro \
  -v ./test/scripts/cluster/discovery-service-server-key.pem:/etc/discovery/tls.key:ro \
  ghcr.io/grepplabs/talos-discovery:latest service \
  --web-enable \
  --server-tls-enable \
  --server-tls-file-cert=/etc/discovery/tls.crt \
  --server-tls-file-key=/etc/discovery/tls.key
```

### 3. Create a Talos Cluster

Create a Talos cluster configured to use the discovery service:


```bash
talosctl cluster create docker \
  --config-patch-controlplanes @./test/scripts/cluster/discovery-tls.patch.yaml \
  --config-patch-workers @./test/scripts/cluster/discovery-tls.patch.yaml
```

### 4. Get the Cluster ID

Check the discovery service logs to obtain the generated `clusterID`:

```bash
docker logs talos-discovery
```

### 5. Verify Discovery in the Web UI

Open the discovery service web interface:

```bash
google-chrome http://localhost:8080
```

## License

This repository is licensed under AGPL-3.0, except for specific files listed in
`THIRD_PARTY_NOTICES.md`.

`api/cluster.proto` is derived from Sidero Labs discovery API schema and is
licensed under MPL-2.0. Generated files derived from that schema are listed in
`THIRD_PARTY_NOTICES.md`. The MPL-2.0 license text is available at
`LICENSES/MPL-2.0.txt`.


[Discovery Service]: https://github.com/siderolabs/discovery-service
[Discovery Client]: https://github.com/siderolabs/discovery-client
[Talos Discovery]: https://docs.siderolabs.com/talos/v1.12/configure-your-talos-cluster/system-configuration/discovery
