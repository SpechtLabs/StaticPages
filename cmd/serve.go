package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/SpechtLabs/StaticPages/pkg/api"
	"github.com/SpechtLabs/StaticPages/pkg/proxy"
	"github.com/fsnotify/fsnotify"
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	serveApi   bool
	serveProxy bool
)

func newServeCmd() *cobra.Command {
	serveCmd := &cobra.Command{
		Use:     "serve",
		Short:   "Serves the static pages application",
		Example: "staticpages serve --api --proxy",
		Args:    cobra.ExactArgs(0),
		RunE:    runServe,
	}

	serveCmd.Flags().BoolVar(&serveApi, "api", false, "Serve API?")
	serveCmd.Flags().BoolVar(&serveProxy, "proxy", false, "Serve Proxy?")

	return serveCmd
}

func runServe(_ *cobra.Command, _ []string) error {
	if !serveApi && !serveProxy {
		return humane.New("Nothing to serve: neither the API nor the proxy was enabled",
			"Pass --api to serve the upload API, --proxy to serve the reverse proxy, or both.",
			"Example: staticpages serve --api --proxy",
		)
	}

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
		a, p = reload(e, a, p)
	})
	viper.WatchConfig()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	if serveProxy {
		if err := p.Shutdown(); err != nil {
			return humane.Wrap(err, "Unable to shut down the proxy", "The process exits anyway; in-flight requests may have been cut off.")
		}
	}

	return nil
}

// reload reads the changed configuration and restarts the servers that are
// running with it, returning the new ones.
func reload(e fsnotify.Event, a *api.RestApi, p *proxy.Proxy) (*api.RestApi, *proxy.Proxy) {
	otelzap.L().Info("Config file change detected. Reloading", zap.String("filename", e.Name))

	readConfig()

	if serveApi {
		if err := a.Shutdown(); err != nil {
			otelzap.L().WithError(err).Fatal("Unable to shutdown api")
			return a, p
		}

		a = api.NewRestApi(configuration)
		a.ServeAsync(configuration.ApiBindAddr())
	}

	if serveProxy {
		if err := p.Shutdown(); err != nil {
			otelzap.L().WithError(err).Fatal("Unable to shutdown proxy")
			return a, p
		}

		p = proxy.NewProxy(configuration)
		p.ServeAsync(configuration.ProxyBindAddr())
	}

	return a, p
}
