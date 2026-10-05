package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCmd(t *testing.T) {
	Version, Commit, Date, BuiltBy = "1.2.3", "abc123", "2026-10-05", "goreleaser"

	out := captureStdout(t, func() {
		require.NoError(t, newVersionCmd().Execute())
	})

	assert.Equal(t, "Version: 1.2.3\nDate:    2026-10-05\nCommit:  abc123\nBuiltBy: goreleaser\n", out)
}

func TestRun(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte("server:\n  host: 127.0.0.1\n"), 0o600))
	missing := filepath.Join(t.TempDir(), "missing.yaml")

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantOutput string
	}{
		{name: "version", args: []string{"--config", configFile, "version"}, wantCode: 0, wantOutput: "Version:"},
		{name: "unreadable config", args: []string{"--config", missing, "version"}, wantCode: 1, wantOutput: "Unable to read config file"},
		{name: "nothing to serve", args: []string{"--config", configFile, "serve"}, wantCode: 1, wantOutput: "Nothing to serve"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var code int
			out := captureStdout(t, func() {
				code = run(tt.args)
			})

			assert.Equal(t, tt.wantCode, code)
			assert.Contains(t, out, tt.wantOutput)
		})
	}
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()

	stdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = stdout }()

	f()
	require.NoError(t, w.Close())

	var out bytes.Buffer
	_, err = io.Copy(&out, r)
	require.NoError(t, err)
	return out.String()
}
