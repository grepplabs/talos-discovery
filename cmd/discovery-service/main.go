package main

import (
	"flag"
	"time"

	"github.com/grepplabs/loggo/zlog"
	"github.com/grepplabs/talos-discovery/internal/config"
	"github.com/grepplabs/talos-discovery/internal/server"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var cfg config.Config

func main() {
	root := &cobra.Command{
		Use:   "discovery-service",
		Short: "Talos Discovery Service",
		Run: func(cmd *cobra.Command, args []string) {
			run()
		},
	}
	config.BindFlagsToViper(root)

	// config flags
	root.PersistentFlags().StringP("config", "", "", "config file (env: CONFIG)")
	root.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	// server flags
	root.Flags().StringVar(&cfg.Server.Addr, "server-addr", ":3000", "Server listen address.")

	root.Flags().BoolVar(&cfg.Server.TLS.Enable, "server-tls-enable", false, "Enable server-side TLS.")
	root.Flags().DurationVar(&cfg.Server.TLS.Refresh, "server-tls-refresh", 0, "Interval for refreshing server TLS certificates. Set to 0 to disable auto-refresh.")
	root.Flags().StringVar(&cfg.Server.TLS.KeyPassword, "server-tls-key-password", "", "Password to decrypt RSA private key.")
	root.Flags().StringVar(&cfg.Server.TLS.File.Key, "server-tls-file-key", "", "Path to the server TLS private key file.")
	root.Flags().StringVar(&cfg.Server.TLS.File.Cert, "server-tls-file-cert", "", "Path to the server TLS certificate file.")
	root.Flags().StringVar(&cfg.Server.TLS.File.ClientCAs, "server-tls-file-client-ca", "", "Path to the server client CA file for client verification.")
	root.Flags().StringVar(&cfg.Server.TLS.File.ClientCRL, "server-tls-file-client-crl", "", "Path to the TLS X509 CRL signed by the client CA. If unspecified, only the client CA is verified.")

	// discovery config flags
	root.Flags().StringVar(&cfg.DiscoveryConfig.RedirectEndpoint, "discovery-redirect-endpoint", "", "Redirect endpoint to include in discovery responses.")
	root.Flags().DurationVar(&cfg.DiscoveryConfig.CleanupInterval, "discovery-cleanup-interval", 5*time.Minute, "Interval for cleaning up expired clusters.")
	root.Flags().StringVar(&cfg.DiscoveryConfig.SnapshotPath, "discovery-snapshot-path", "", "Path to persist discovery state snapshots. Empty disables snapshots.")
	root.Flags().DurationVar(&cfg.DiscoveryConfig.SnapshotInterval, "discovery-snapshot-interval", 1*time.Minute, "Interval between discovery state snapshots.")

	// Merge stdlib flags into pflag (so Cobra can see them)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	if err := root.Execute(); err != nil {
		zlog.Fatalw("execution error", "error", err)
	}
}

func run() {
	zlog.Infof("running")
	err := server.StartDiscoveryServer(cfg)
	if err != nil {
		zlog.Fatalw("problem running server", "error", err)
	}
}
