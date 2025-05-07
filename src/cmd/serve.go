package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/SpechtLabs/StaticPages/pkg/api"
	"github.com/SpechtLabs/StaticPages/pkg/proxy"
	"github.com/fsnotify/fsnotify"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	serveApi   bool
	serveProxy bool
)

func init() {
	serveCmd.Flags().BoolVar(&serveApi, "api", false, "Serve API?")
	serveCmd.Flags().BoolVar(&serveProxy, "proxy", false, "Serve Proxy?")

	RootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:     "serve",
	Short:   "Serves the static pages application",
	Example: "staticpages serve --api --proxy",
	Args:    cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		p := proxy.NewProxy(configuration)
		a := api.NewRestApi(configuration)

		// Serve Rest-API
		if serveApi {
			a.ServeAsync(configuration.ApiBindAddr())
		}

		// Serve Reverse Proxy
		if serveProxy {
			p.ServeAsync(configuration.ProxyBindAddr())
		}

		viper.OnConfigChange(func(e fsnotify.Event) {
			otelzap.L().Info("Config file change detected. Reloading", zap.String("filename", e.Name))

			readConfig()

			if serveApi {
				if err := a.Shutdown(); err != nil {
					otelzap.L().WithError(err).Fatal("Unable to shutdown api")
					return
				}

				a = api.NewRestApi(configuration)
				a.ServeAsync(configuration.ProxyBindAddr())
			}

			if serveProxy {
				if err := p.Shutdown(); err != nil {
					otelzap.L().WithError(err).Fatal("Unable to shutdown proxy")
					return
				}

				p = proxy.NewProxy(configuration)
				p.ServeAsync(configuration.ProxyBindAddr())
			}
		})
		viper.WatchConfig()

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c

		if serveProxy {
			if err := p.Shutdown(); err != nil {
				otelzap.L().WithError(err).Fatal("Unable to shutdown proxy")
				return
			}
		}
	},
}
