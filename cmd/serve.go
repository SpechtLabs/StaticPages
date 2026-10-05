package cmd

import (
	"context"
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

func runServe(cmd *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return serve(ctx)
}

// servers are the API and the proxy as serve runs them; either is nil when
// it isn't served.
type servers struct {
	api   *api.RestApi
	proxy *proxy.Proxy
}

// serve starts the servers the flags ask for, restarts them when the
// configuration file changes, and shuts them down when ctx is done.
func serve(ctx context.Context) error {
	if !serveApi && !serveProxy {
		return humane.New("Nothing to serve: neither the API nor the proxy was enabled",
			"Pass --api to serve the upload API, --proxy to serve the reverse proxy, or both.",
			"Example: staticpages serve --api --proxy",
		)
	}

	running := start()

	reloads := make(chan fsnotify.Event, 1)
	viper.OnConfigChange(func(e fsnotify.Event) {
		select {
		case reloads <- e:
		default: // a reload is pending already, and it will read this change too
		}
	})
	viper.WatchConfig()

	for {
		select {
		case e := <-reloads:
			running = reload(e, running)

		case <-ctx.Done():
			return running.shutdown()
		}
	}
}

// start starts the servers the flags ask for with the current configuration.
func start() servers {
	var s servers

	if serveApi {
		s.api = api.NewRestApi(configuration)
		s.api.ServeAsync(configuration.ApiBindAddr())
	}

	if serveProxy {
		s.proxy = proxy.NewProxy(configuration)
		s.proxy.ServeAsync(configuration.ProxyBindAddr())
	}

	return s
}

// reload reads the changed configuration and restarts the servers with it. A
// configuration that can't be read leaves the servers running as they are.
func reload(e fsnotify.Event, running servers) servers {
	if herr := readConfig(); herr != nil {
		// zap.Error rather than WithError: otelzap's With adds the fields to the
		// shared logger, which races with the servers' goroutines logging.
		otelzap.L().Error("Config file changed but can't be read; keeping the running configuration",
			zap.String("filename", e.Name), zap.Error(herr))
		return running
	}

	otelzap.L().Info("Config file change detected. Reloading", zap.String("filename", e.Name))

	if herr := running.shutdown(); herr != nil {
		otelzap.L().WithError(herr).Fatal("Unable to shut down for the reload", zap.String("filename", e.Name))
	}

	return start()
}

// shutdown stops the running servers.
func (s servers) shutdown() humane.Error {
	if s.api != nil {
		if herr := s.api.Shutdown(); herr != nil {
			return humane.Wrap(herr, "Unable to shut down the API server", "In-flight uploads may have been cut off.")
		}
	}

	if s.proxy != nil {
		if herr := s.proxy.Shutdown(); herr != nil {
			return humane.Wrap(herr, "Unable to shut down the proxy", "In-flight requests may have been cut off.")
		}
	}

	return nil
}
