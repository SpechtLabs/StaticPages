package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServe(t *testing.T) {
	tests := []struct {
		name       string
		api, proxy bool
	}{
		{name: "api", api: true},
		{name: "proxy", proxy: true},
		{name: "both", api: true, proxy: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serveApi, serveProxy = tt.api, tt.proxy
			configuration = config.StaticPagesConfig{Server: config.Server{Host: "127.0.0.1"}}

			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- serve(ctx) }()

			time.Sleep(50 * time.Millisecond)
			cancel()

			select {
			case err := <-done:
				assert.NoError(t, err)
			case <-time.After(10 * time.Second):
				t.Fatal("serve didn't return after its context was canceled")
			}
		})
	}
}

func TestReload(t *testing.T) {
	serveApi, serveProxy = true, true
	configuration = config.StaticPagesConfig{Server: config.Server{Host: "127.0.0.1"}}

	configFile := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("server:\n  host: 127.0.0.1\n"), 0o600))
	viper.SetConfigFile(configFile)

	running := start()
	reloaded := reload(fsnotify.Event{Name: configFile}, running)
	assert.NotSame(t, running.api, reloaded.api, "a readable config restarts the API")
	assert.NotSame(t, running.proxy, reloaded.proxy, "a readable config restarts the proxy")

	viper.SetConfigFile(filepath.Join(t.TempDir(), "missing.yaml"))
	kept := reload(fsnotify.Event{Name: configFile}, reloaded)
	assert.Same(t, reloaded.api, kept.api, "an unreadable config keeps the running API")
	assert.Same(t, reloaded.proxy, kept.proxy, "an unreadable config keeps the running proxy")

	assert.NoError(t, kept.shutdown())
}
