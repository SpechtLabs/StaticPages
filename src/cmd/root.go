package cmd

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/sierrasoftworks/humane-errors-go"
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

	hostname       string
	port           int
	configFileName string
	debug          bool
	outputFormat   string
)

func initTelemetry() (func(), *otelzap.Logger) {
	var err error

	// Initialize Logging
	var zapLog *zap.Logger
	if debug {
		zapLog, err = zap.NewDevelopment()
		gin.SetMode(gin.DebugMode)
	} else {
		zapLog, err = zap.NewProduction()
		gin.SetMode(gin.ReleaseMode)
	}

	if err != nil {
		panic(fmt.Errorf("failed to initialize logger: %w", err))
	}

	otelZap := otelzap.New(zapLog,
		otelzap.WithCaller(true),
		otelzap.WithErrorStatusLevel(zap.ErrorLevel),
		otelzap.WithStackTrace(false),
	)

	undo := otelzap.ReplaceGlobals(otelZap)

	return undo, otelZap
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&configFileName, "config", "c", "", "Name of the config file")

	rootCmd.PersistentFlags().IntVar(&port, "port", 50051, "Port of the Server")
	viper.SetDefault("server.port", 8099)
	err := viper.BindPFlag("server.port", rootCmd.PersistentFlags().Lookup("port"))
	if err != nil {
		panic(fmt.Errorf("fatal binding flag: %w", err))
	}

	rootCmd.PersistentFlags().StringVarP(&hostname, "server", "s", "", "")
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

	rootCmd.PersistentFlags().StringVarP(&outputFormat, "out", "o", "short", "Configure your output format (short, long, json)")
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

	// Find and read the config file
	if err := viper.ReadInConfig(); err != nil {
		// Handle errors reading the config file
		herr := humane.Wrap(err, "Unable to read config file", "Make sure the config file exists, is readable, and conforms to the format.")
		fmt.Printf("Unable to read config file, assuming default values: %s\n", herr.Display())
	}

	hostname = viper.GetString("server.host")
	port = viper.GetInt("server.port")
	debug = viper.GetBool("output.debug")
}

func viperConfigChange(undo func(), zapLog *otelzap.Logger) {
	viper.OnConfigChange(func(e fsnotify.Event) {
		otelzap.L().Sugar().Infow("Config file change detected. Reloading.", "filename", e.Name)

		// refresh logger
		zapLog.Sync()
		undo()
		undo, zapLog = initTelemetry()

		if hostname != viper.GetString("server.host") ||
			port != viper.GetInt("server.port") {
			zapLog.Sugar().Errorw("Unable to change host or port at runtime!",
				"new_host", viper.GetString("server.host"),
				"old_host", hostname,
				"new_port", viper.GetInt("server.port"),
				"old_port", port,
			)
		}
	})
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "staticpages",
	Short: "A simple Static Pages Server for hosting your own static pages.",
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
