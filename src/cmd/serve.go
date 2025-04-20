package cmd

import (
	"fmt"
	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/SpechtLabs/StaticPages/pkg/proxy"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"syscall"

	humane "github.com/sierrasoftworks/humane-errors-go"
)

var (
	serveApi   bool
	serveProxy bool
)

var serveCmd = &cobra.Command{
	Use:     "serve",
	Short:   "Serves the static pages application",
	Example: "staticpages version",
	Args:    cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		undo, zapLog := initTelemetry()
		defer zapLog.Sync()
		defer undo()

		if debug {
			file, err := os.ReadFile(viper.GetViper().ConfigFileUsed())
			if err != nil {
				herr := humane.Wrap(err, "Unable to read config file", "Make sure the config file exists, is readable, and conforms to the format.")
				panic(herr)
			}
			zapLog.Sugar().With("config_file", string(file)).Debug("Config file used")
		}

		if !serveApi && !serveProxy {
			err := humane.New("Unable to start StaticPages server.", "You need to specify at least one of the following options: --api, --proxy")
			panic(err)
		}

		viperConfigChange(undo, zapLog)
		viper.WatchConfig()

		// Serve Rest-API
		if serveProxy {
			go func() {
				// TODO: Implement api!
				//restApiServer := api.NewRestApiServer(otelZap, iCalClient)
				//if err := restApiServer.ListenAndServe(); err != nil {
				//	panic(err.Error())
				//}
			}()
		}

		// Serve Reverse Proxy
		if serveProxy {
			pages, err := config.ParsePages()
			if err != nil {
				zapLog.Sugar().Errorw("Unable to parse pages", "error", err.Error(), "advice", err.Advice(), "cause", err.Cause())
			}

			proxy := proxy.NewProxy(zapLog, pages)

			go func() {
				if err := proxy.Serve(fmt.Sprintf("%s:%d", hostname, port)); err != nil {
					zapLog.Sugar().Errorw("Unable to serve static pages reverse proxy", "error", err.Error(), "advice", err.Advice(), "cause", err.Cause())
				}
			}()
		}

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
	},
}

func init() {
	serveCmd.Flags().BoolVarP(&serveApi, "api", "a", false, "Serve API?")
	serveCmd.Flags().BoolVarP(&serveProxy, "proxy", "p", false, "Serve Proxy?")

	rootCmd.AddCommand(serveCmd)
}
