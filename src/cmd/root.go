package cmd

import (
	"fmt"
	"github.com/SpechtLabs/StaticPages/pkg/config"
	humane "github.com/sierrasoftworks/humane-errors-go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
	"os"
)

var (
	configFileName string
	configuration  config.StaticPagesConfig

	undoFinalizer func()
)

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVarP(&configFileName, "config", "c", "", "Name of the config file")

	RootCmd.PersistentFlags().IntP("port", "p", 50051, "Port of the Server")
	viper.SetDefault("server.port", 8099)
	err := viper.BindPFlag("server.port", RootCmd.PersistentFlags().Lookup("port"))
	if err != nil {
		panic(fmt.Errorf("fatal binding flag: %w", err))
	}

	RootCmd.PersistentFlags().StringP("server", "s", "", "")
	viper.SetDefault("server.host", "")
	err = viper.BindPFlag("server.host", RootCmd.PersistentFlags().Lookup("server"))
	if err != nil {
		panic(fmt.Errorf("fatal binding flag: %w", err))
	}

	RootCmd.PersistentFlags().BoolP("debug", "d", false, "enable debug logging")
	viper.SetDefault("output.debug", false)
	err = viper.BindPFlag("output.debug", RootCmd.PersistentFlags().Lookup("debug"))
	if err != nil {
		panic(fmt.Errorf("fatal binding flag: %w", err))
	}

	RootCmd.PersistentFlags().StringP("out", "o", string(config.ShortFormat), "Configure your output format (short, long)")
	viper.SetDefault("output.format", "short")
	err = viper.BindPFlag("output.format", RootCmd.PersistentFlags().Lookup("out"))
	if err != nil {
		panic(fmt.Errorf("fatal binding flag: %w", err))
	}
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

	if err := viper.Unmarshal(&configuration); err != nil {
		herr := humane.Wrap(err, "Unable to parse config file", "Make sure the config file exists, is readable, and conforms to the format.")
		fmt.Printf("Unable to read config file, assuming default values: %s\n", herr.Display())
		os.Exit(1)
	}

	if err := configuration.Parse(); err != nil {
		fmt.Println(err.Display())
		os.Exit(1)
	}
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "staticpages",
	Short: "A simple Static Pages Server for hosting your own static pages.",
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		readConfig()

		if otelzap.L().Core().Enabled(zap.DebugLevel) {
			file, err := os.ReadFile(viper.GetViper().ConfigFileUsed())
			if err != nil {
				herr := humane.Wrap(err, "Unable to read config file", "Make sure the config file exists, is readable, and conforms to the format.")
				panic(herr)
			}
			otelzap.L().Sugar().Debugw("Config file used", zap.String("config_file", string(file)))
		}

		if !serveApi && !serveProxy {
			err := humane.New("Unable to start StaticPages server.", "You need to specify at least one of the following options: --api, --proxy")
			panic(err)
		}

		return nil
	},
}
