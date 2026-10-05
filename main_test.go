package main

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionCmd(t *testing.T) {
	Version, Commit, Date, BuiltBy = "1.2.3", "abc123", "2026-10-05", "goreleaser"

	stdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = stdout })

	require.NoError(t, newVersionCmd().Execute())
	require.NoError(t, w.Close())

	var out bytes.Buffer
	_, err = io.Copy(&out, r)
	require.NoError(t, err)

	assert.Equal(t, "Version: 1.2.3\nDate:    2026-10-05\nCommit:  abc123\nBuiltBy: goreleaser\n", out.String())
}
