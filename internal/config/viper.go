package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/grepplabs/loggo/zlog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func BindFlagsToViper(cmd *cobra.Command) {
	cobra.OnInitialize(func() {
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
		viper.AutomaticEnv()

		if err := viper.BindPFlags(cmd.PersistentFlags()); err != nil {
			zlog.Fatalw("unable to bind persistent flags to viper", "error", err)
		}
		if err := viper.BindPFlags(cmd.Flags()); err != nil {
			zlog.Fatalw("unable to bind flags to viper", "error", err)
		}

		if viper.IsSet("config") {
			cfg := viper.GetString("config")
			viper.SetConfigFile(cfg)

			if err := viper.ReadInConfig(); err != nil {
				zlog.Fatalf("failed to read config file %s: %v", cfg, err)
			}
			zlog.Infof("using config file: %s", viper.ConfigFileUsed())
		}
		applyViperToCommand(cmd)
	})
}

func applyViperToCommand(cmd *cobra.Command) {
	applyViperToFlagSet(cmd.InheritedFlags())
	applyViperToFlagSet(cmd.Flags())
}

func applyViperToFlagSet(fs *pflag.FlagSet) {
	if fs == nil {
		return
	}
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Changed || !viper.IsSet(f.Name) {
			return
		}
		if err := fs.Set(f.Name, fmt.Sprint(viper.Get(f.Name))); err != nil {
			log.Fatalf("Unable to set flag %q from viper: %v", f.Name, err)
		}
	})
}
