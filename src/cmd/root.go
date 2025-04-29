package cmd

import (
	"fmt"
	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/gin-gonic/gin"
	humane "github.com/sierrasoftworks/humane-errors-go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.uber.org/zap"
	"os"
)

var (
	// Version represents the Version of the StaticPages binary, should be set via ldflags -X
	Version string

	// Date represents the Date of when the StaticPages binary was build, should be set via ldflags -X
	Date string

	// Commit represents the Commit-hash from which StaticPages binary was build, should be set via ldflags -X
	Commit string

	// BuiltBy represents who build the binary
	BuiltBy string

	configFileName string
	configuration  config.StaticPagesConfig

	zapLog        *otelzap.SugaredLogger
	undoFinalizer func()
)

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&configFileName, "config", "c", "", "Name of the config file")

	rootCmd.PersistentFlags().IntP("port", "p", 50051, "Port of the Server")
	viper.SetDefault("server.port", 8099)
	err := viper.BindPFlag("server.port", rootCmd.PersistentFlags().Lookup("port"))
	if err != nil {
		panic(fmt.Errorf("fatal binding flag: %w", err))
	}

	rootCmd.PersistentFlags().StringP("server", "s", "", "")
	viper.SetDefault("server.host", "")
	err = viper.BindPFlag("server.host", rootCmd.PersistentFlags().Lookup("server"))
	if err != nil {
		panic(fmt.Errorf("fatal binding flag: %w", err))
	}

	rootCmd.PersistentFlags().BoolP("debug", "d", false, "enable debug logging")
	viper.SetDefault("output.debug", false)
	err = viper.BindPFlag("output.debug", rootCmd.PersistentFlags().Lookup("debug"))
	if err != nil {
		panic(fmt.Errorf("fatal binding flag: %w", err))
	}

	rootCmd.PersistentFlags().StringP("out", "o", string(config.ShortFormat), "Configure your output format (short, long)")
	viper.SetDefault("output.format", "short")
	err = viper.BindPFlag("output.format", rootCmd.PersistentFlags().Lookup("out"))
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

func initTelemetry() (func(), *otelzap.SugaredLogger) {
	var err error

	// Initialize Logging
	var zapLog *zap.Logger
	if configuration.Output.Debug {
		zapLog, err = zap.NewDevelopment()
		gin.SetMode(gin.DebugMode)
	} else {
		zapLog, err = zap.NewProduction()
		gin.SetMode(gin.ReleaseMode)
	}

	if err != nil {
		fmt.Printf("failed to initialize logger: %w", err)
		os.Exit(1)
	}

	otelZap := otelzap.New(zapLog,
		otelzap.WithCaller(true),
		otelzap.WithMinLevel(zap.InfoLevel),
		otelzap.WithErrorStatusLevel(zap.ErrorLevel),
		otelzap.WithStackTrace(false),
	)

	undo := otelzap.ReplaceGlobals(otelZap)

	finalizer := func() {
		_ = otelZap.Sync()
		undo()
	}

	return finalizer, otelZap.Sugar()
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

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "staticpages",
	Short: "A simple Static Pages Server for hosting your own static pages.",
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		readConfig()

		undoFinalizer, zapLog = initTelemetry()

		if configuration.Output.Debug {
			file, err := os.ReadFile(viper.GetViper().ConfigFileUsed())
			if err != nil {
				herr := humane.Wrap(err, "Unable to read config file", "Make sure the config file exists, is readable, and conforms to the format.")
				panic(herr)
			}
			zapLog.Debugw("Config file used", zap.String("config_file", string(file)))
		}

		if !serveApi && !serveProxy {
			err := humane.New("Unable to start StaticPages server.", "You need to specify at least one of the following options: --api, --proxy")
			panic(err)
		}

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(version, commit, date, builtBy string) {
	Version = version
	Date = date
	Commit = commit
	BuiltBy = builtBy

	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
