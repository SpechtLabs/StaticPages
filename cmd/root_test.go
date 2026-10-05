package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRootCmd(t *testing.T) {
	rootCmd, herr := NewRootCmd()
	require.NoError(t, herr)

	for _, flag := range []string{"config", "port", "server", "debug", "out"} {
		assert.NotNil(t, rootCmd.PersistentFlags().Lookup(flag), "--%s", flag)
	}

	serveCmd, _, err := rootCmd.Find([]string{"serve"})
	require.NoError(t, err)
	assert.Equal(t, "serve", serveCmd.Name())
	assert.NotNil(t, serveCmd.Flags().Lookup("api"))
	assert.NotNil(t, serveCmd.Flags().Lookup("proxy"))
}

func TestRunServeWithNothingToServe(t *testing.T) {
	serveApi, serveProxy = false, false

	err := runServe(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Nothing to serve")
}
