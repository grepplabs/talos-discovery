package config

import (
	"runtime/debug"
	"time"

	tlsconfig "github.com/grepplabs/cert-source/config"
)

var (
	// Version is the current version of the app, generated at build time
	Version = "unknown"
)

const (
	InMemoryTransport = ":in-memory"
)

func GetBuildVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return Version
}

type ServiceCommandConfig struct {
	Server             ServerConfig
	DiscoveryConfig    DiscoveryConfig
	WebServer          ServerConfig
	WebEnable          bool
	WebDiscoveryClient ClientConfig
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
type WebCommandConfig struct {
	WebServer          ServerConfig
	WebDiscoveryClient ClientConfig
}

type ClientConfig struct {
	Target string
	TLS    tlsconfig.TLSClientConfig
}
