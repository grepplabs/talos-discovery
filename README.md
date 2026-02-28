# talos-discovery

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


[Discovery Service]: https://github.com/siderolabs/discovery-service
[Discovery Client]: https://github.com/siderolabs/discovery-client
[Talos Discovery]: https://docs.siderolabs.com/talos/v1.12/configure-your-talos-cluster/system-configuration/discovery

