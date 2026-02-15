package config

import (
	"time"

	tlsconfig "github.com/grepplabs/cert-source/config"
)

var (
	// Version is the current version of the app, generated at build time
	Version = "unknown"
)

type Config struct {
	Server          ServerConfig
	DiscoveryConfig DiscoveryConfig
}

type ServerConfig struct {
	Addr string
	TLS  tlsconfig.TLSServerConfig
}

type DiscoveryConfig struct {
	RedirectEndpoint string
	CleanupInterval  time.Duration
	SnapshotPath     string
	SnapshotInterval time.Duration
}
