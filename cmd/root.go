package cmd

import (
	"fmt"
	"os"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	configFileName string
	configuration  config.StaticPagesConfig
)

// NewRootCmd builds the staticpages command tree: the persistent flags, bound
// to viper, the configuration defaults, and the serve command.
func NewRootCmd() (*cobra.Command, humane.Error) {
	rootCmd := &cobra.Command{
		Use:   "staticpages",
		Short: "A simple Static Pages Server for hosting your own static pages.",
		// Render errors ourselves (as humane, advice-rich messages) in main rather
		// than letting cobra print a terse "Error:" line and a usage dump.
		SilenceErrors:     true,
		SilenceUsage:      true,
		PersistentPreRunE: loadConfig,
	}

	cobra.OnInitialize(initConfig)
	config.SetDefaults(viper.GetViper())

	flags := rootCmd.PersistentFlags()
	flags.StringVarP(&configFileName, "config", "c", "", "Name of the config file")
	flags.IntP("port", "p", 50051, "Port of the Server")
	flags.StringP("server", "s", "", "")
	flags.BoolP("debug", "d", false, "enable debug logging")
	flags.StringP("out", "o", string(config.ShortFormat), "Configure your output format (short, long)")

	// server.host and output.format have their defaults in config.SetDefaults.
	viper.SetDefault("server.port", 8099)
	viper.SetDefault("output.debug", false)

	for key, flag := range map[string]string{
		"server.port":   "port",
		"server.host":   "server",
		"output.debug":  "debug",
		"output.format": "out",
	} {
		if err := viper.BindPFlag(key, flags.Lookup(flag)); err != nil {
			return nil, humane.Wrap(err, fmt.Sprintf("Unable to bind the --%s flag to %s", flag, key),
				"This is a bug in staticpages; please report it.")
		}
	}

	rootCmd.AddCommand(newServeCmd())

	return rootCmd, nil
}

func initConfig() {
	if configFileName != "" {
		viper.SetConfigFile(configFileName)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath(home)
		viper.AddConfigPath("$HOME/.config/StaticPages/")
		viper.AddConfigPath("/data")
	}

	viper.SetEnvPrefix("SP")
	viper.AutomaticEnv()
}

func readConfig() {
	// Find and read the config file
	if err := viper.ReadInConfig(); err != nil {
		// Handle errors reading the config file
		herr := humane.Wrap(err, "Unable to read config file", "Make sure the config file exists, is readable, and conforms to the format.")
		fmt.Printf("Unable to read config file, assuming default values: %s\n", herr.Display())
		os.Exit(1)
	}

	// Expand the optional top-level pageDefaults block into each page before
	// unmarshalling, so shared settings can be declared once.
	config.ApplyPageDefaults(viper.GetViper())

	if err := viper.Unmarshal(&configuration); err != nil {
		herr := humane.Wrap(err, "Unable to parse config file", "Make sure the config file exists, is readable, and conforms to the format.")
		fmt.Printf("Unable to read config file, assuming default values: %s\n", herr.Display())
		os.Exit(1)
	}
}

func loadConfig(_ *cobra.Command, _ []string) error {
	readConfig()

	if otelzap.L().Core().Enabled(zap.DebugLevel) {
		file, err := os.ReadFile(viper.GetViper().ConfigFileUsed())
		if err != nil {
			return humane.Wrap(err, "Unable to read config file", "Make sure the config file exists, is readable, and conforms to the format.")
		}
		otelzap.L().Debug("Config file used", zap.String("config_file", string(file)))
	}

	return nil
}
