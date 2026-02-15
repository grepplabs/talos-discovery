package main

import (
	"flag"
	"time"

	"github.com/grepplabs/loggo/zlog"
	"github.com/grepplabs/talos-discovery/internal/config"
	"github.com/grepplabs/talos-discovery/internal/server"
	"github.com/grepplabs/talos-discovery/internal/web"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	defaultServicePort = "3000"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		zlog.Fatalw("execution error", "error", err)
	}
}

func newRootCommand() *cobra.Command {
	serviceCfg := config.ServiceCommandConfig{}
	webCfg := config.WebCommandConfig{}

	serviceCmd := newServiceCommand(&serviceCfg)
	webCmd := newWebCommand(&webCfg)

	root := &cobra.Command{
		Use:   "discovery-service",
		Short: "Talos Discovery Service",
	}

	registerRootFlags(root)
	registerServiceFlags(serviceCmd, &serviceCfg)
	registerWebFlags(webCmd, &webCfg)
	root.AddCommand(serviceCmd, webCmd)
	config.BindFlagsToViper(root)

	// Merge stdlib flags into pflag (so Cobra can see them)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	return root
}

func registerRootFlags(root *cobra.Command) {
	// shared flags
	root.PersistentFlags().StringP("config", "", "", "config file (env: CONFIG)")
}

func registerServiceFlags(cmd *cobra.Command, cfg *config.ServiceCommandConfig) {
	// server flags
	cmd.Flags().StringVar(&cfg.Server.Addr, "server-addr", ":"+defaultServicePort, "gRPC server listen address.")
	cmd.Flags().BoolVar(&cfg.Server.TLS.Enable, "server-tls-enable", false, "Enable server-side TLS.")
	cmd.Flags().DurationVar(&cfg.Server.TLS.Refresh, "server-tls-refresh", 0, "Interval for refreshing server TLS certificates. Set to 0 to disable auto-refresh.")
	cmd.Flags().StringVar(&cfg.Server.TLS.KeyPassword, "server-tls-key-password", "", "Password to decrypt RSA private key.")
	cmd.Flags().StringVar(&cfg.Server.TLS.File.Key, "server-tls-file-key", "", "Path to the server TLS private key file.")
	cmd.Flags().StringVar(&cfg.Server.TLS.File.Cert, "server-tls-file-cert", "", "Path to the server TLS certificate file.")
	cmd.Flags().StringVar(&cfg.Server.TLS.File.ClientCAs, "server-tls-file-client-ca", "", "Path to the server client CA file for client verification.")
	cmd.Flags().StringVar(&cfg.Server.TLS.File.ClientCRL, "server-tls-file-client-crl", "", "Path to the TLS X509 CRL signed by the client CA. If unspecified, only the client CA is verified.")

	// discovery config flags
	cmd.Flags().StringVar(&cfg.DiscoveryConfig.RedirectEndpoint, "discovery-redirect-endpoint", "", "Redirect endpoint to include in discovery responses.")
	cmd.Flags().DurationVar(&cfg.DiscoveryConfig.CleanupInterval, "discovery-cleanup-interval", 5*time.Minute, "Interval for cleaning up expired clusters.")
	cmd.Flags().StringVar(&cfg.DiscoveryConfig.SnapshotPath, "discovery-snapshot-path", "", "Path to persist discovery state snapshots. Empty disables snapshots.")
	cmd.Flags().DurationVar(&cfg.DiscoveryConfig.SnapshotInterval, "discovery-snapshot-interval", 1*time.Minute, "Interval between discovery state snapshots.")

	// web server flags
	registerWebServerFlags(cmd, &cfg.WebServer)
	cmd.Flags().BoolVar(&cfg.WebEnable, "web-enable", false, "Enable embedded web UI in service process.")
	cmd.Flags().StringVar(&cfg.WebDiscoveryClient.Target, "web-discovery-client-target", config.InMemoryTransport, `Discovery gRPC endpoint used by embedded web. Use ":in-memory" for in-memory transport.`)
	registerDiscoveryClientTLSFlags(cmd, &cfg.WebDiscoveryClient)
}

func registerWebFlags(cmd *cobra.Command, cfg *config.WebCommandConfig) {
	// web server flags
	registerWebServerFlags(cmd, &cfg.WebServer)

	// discovery client flags
	cmd.Flags().StringVar(&cfg.WebDiscoveryClient.Target, "web-discovery-client-target", "localhost:"+defaultServicePort, "gRPC endpoint for the discovery service (e.g. localhost:3000).")
	registerDiscoveryClientTLSFlags(cmd, &cfg.WebDiscoveryClient)
}

func registerWebServerFlags(cmd *cobra.Command, webServer *config.ServerConfig) {
	cmd.Flags().StringVar(&webServer.Addr, "web-addr", ":8080", "Web listen address.")
	cmd.Flags().BoolVar(&webServer.TLS.Enable, "web-tls-enable", false, "Enable server-side TLS.")
	cmd.Flags().DurationVar(&webServer.TLS.Refresh, "web-tls-refresh", 0, "Interval for refreshing server TLS certificates. Set to 0 to disable auto-refresh.")
	cmd.Flags().StringVar(&webServer.TLS.KeyPassword, "web-tls-key-password", "", "Password to decrypt RSA private key.")
	cmd.Flags().StringVar(&webServer.TLS.File.Key, "web-tls-file-key", "", "Path to the server TLS private key file.")
	cmd.Flags().StringVar(&webServer.TLS.File.Cert, "web-tls-file-cert", "", "Path to the server TLS certificate file.")
	cmd.Flags().StringVar(&webServer.TLS.File.ClientCAs, "web-tls-file-client-ca", "", "Path to the server client CA file for client verification.")
	cmd.Flags().StringVar(&webServer.TLS.File.ClientCRL, "web-tls-file-client-crl", "", "Path to the TLS X509 CRL signed by the client CA. If unspecified, only the client CA is verified.")
}

func registerDiscoveryClientTLSFlags(cmd *cobra.Command, cfg *config.ClientConfig) {
	cmd.Flags().BoolVar(&cfg.TLS.Enable, "web-discovery-client-tls-enable", false, "Enable TLS configuration for the discovery client.")
	cmd.Flags().DurationVar(&cfg.TLS.Refresh, "web-discovery-client-tls-refresh", 0, "Interval for reloading client TLS certificates. Set to 0 to disable auto-refresh.")
	cmd.Flags().BoolVar(&cfg.TLS.InsecureSkipVerify, "web-discovery-client-tls-insecure-skip-verify", false, "Skip server certificate verification (insecure; use only for testing).")
	cmd.Flags().BoolVar(&cfg.TLS.UseSystemPool, "web-discovery-client-tls-use-system-pool", true, "Use the system certificate pool for verifying server certificates.")
	cmd.Flags().StringVar(&cfg.TLS.KeyPassword, "web-discovery-client-tls-key-password", "", "Password to decrypt RSA private key.")
	cmd.Flags().StringVar(&cfg.TLS.File.RootCAs, "web-discovery-client-tls-file-root-ca", "", "Path to a custom root CA bundle for verifying the discovery server.")
	cmd.Flags().StringVar(&cfg.TLS.File.Key, "web-discovery-client-tls-file-key", "", "Path to the client TLS private key file (for mTLS).")
	cmd.Flags().StringVar(&cfg.TLS.File.Cert, "web-discovery-client-tls-file-cert", "", "Path to the client TLS certificate file (for mTLS).")
}

func newServiceCommand(cfg *config.ServiceCommandConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "service",
		Short: "Run discovery service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runService(cfg)
		},
	}
}

func newWebCommand(cfg *config.WebCommandConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "web",
		Short: "Run discovery web",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWeb(cfg)
		},
	}
}

func runService(cfg *config.ServiceCommandConfig) error {
	zlog.Infof("service running")
	err := server.StartDiscoveryServer(*cfg)
	if err != nil {
		return err
	}
	return nil
}

func runWeb(cfg *config.WebCommandConfig) error {
	zlog.Infof("web running")
	err := web.StartDiscoveryWeb(*cfg)
	if err != nil {
		return err
	}
	return nil
}
