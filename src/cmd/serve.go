package cmd

import (
	"github.com/SpechtLabs/StaticPages/pkg/proxy"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
)

var (
	serveApi   bool
	serveProxy bool
)

func init() {
	serveCmd.Flags().BoolVar(&serveApi, "api", false, "Serve API?")
	serveCmd.Flags().BoolVar(&serveProxy, "proxy", false, "Serve Proxy?")

	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:     "serve",
	Short:   "Serves the static pages application",
	Example: "staticpages version",
	Args:    cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		defer undoFinalizer()

		p := proxy.NewProxy(zapLog, configuration)

		// Serve Rest-API
		if serveApi {
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
			p.ServeAsync(configuration.ProxyBindAddr())
		}

		viper.OnConfigChange(func(e fsnotify.Event) {
			zapLog.Infow("Config file change detected. Reloading", zap.String("filename", e.Name))

			readConfig()

			// refresh logger
			undoFinalizer()
			undoFinalizer, zapLog = initTelemetry()

			if serveProxy {
				if err := p.Shutdown(); err != nil {
					zapLog.Fatalw("Unable to shutdown proxy",
						zap.Error(err),
						zap.Strings("advice", err.Advice()),
						zap.String("cause", err.Cause().Error()))
					return
				}

				p = proxy.NewProxy(zapLog, configuration)
				p.ServeAsync(configuration.ProxyBindAddr())
			}
		})
		viper.WatchConfig()

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
	},
}
