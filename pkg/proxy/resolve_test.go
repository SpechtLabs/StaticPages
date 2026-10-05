package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SpechtLabs/StaticPages/pkg/api"
	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/SpechtLabs/StaticPages/pkg/s3_client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveCommit(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	index := s3_client.PageIndex{
		"main-old": {Branch: "main", Date: now.Add(-time.Hour)},
		"main-new": {Branch: "main", Date: now},
		"feat-sha": {Branch: "feature", Date: now},
	}

	tests := []struct {
		name       string
		previews   bool
		sub        string
		wantSub    string
		wantSHA    string
		wantErrMsg string
	}{
		{name: "previews off serves main", previews: false, sub: "feature", wantSub: "main", wantSHA: "main-new"},
		{name: "no subdomain serves main", previews: true, sub: "", wantSub: "main", wantSHA: "main-new"},
		{name: "branch subdomain", previews: true, sub: "feature", wantSub: "feature", wantSHA: "feat-sha"},
		{name: "commit subdomain", previews: true, sub: "main-old", wantSub: "main-old", wantSHA: "main-old"},
		{name: "unknown subdomain", previews: true, sub: "nope", wantErrMsg: "could not find a commit to serve page for"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := &config.Page{
				Git:     config.GitConfig{MainBranch: "main"},
				Preview: config.PreviewConfig{Enabled: tt.previews},
			}

			sub, sha, herr := resolveCommit(page, index, tt.sub)
			if tt.wantErrMsg != "" {
				require.Error(t, herr)
				assert.Contains(t, herr.Error(), tt.wantErrMsg)
				return
			}

			require.NoError(t, herr)
			assert.Equal(t, tt.wantSub, sub)
			assert.Equal(t, tt.wantSHA, sha)
		})
	}

	t.Run("main never published", func(t *testing.T) {
		page := &config.Page{Git: config.GitConfig{MainBranch: "trunk"}}
		_, _, herr := resolveCommit(page, index, "")
		require.Error(t, herr)
		assert.Contains(t, herr.Error(), "could not find a commit to serve page for")
	})
}

func TestJoinPagePath(t *testing.T) {
	tests := []struct {
		name      string
		proxyPath string
		base      string
		p         string
		want      string
	}{
		{name: "no proxy path stays relative", base: "repo/sha", p: "/guides/intro", want: "repo/sha/guides/intro"},
		{name: "no proxy path, root", base: "repo/sha", p: "/", want: "repo/sha"},
		{name: "proxy path", proxyPath: "/sites", base: "/sites/repo/sha", p: "/guides/../intro", want: "/sites/repo/sha/intro"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := &config.Page{Proxy: config.PageProxy{Path: config.EnvValue(tt.proxyPath)}}
			assert.Equal(t, tt.want, joinPagePath(page, tt.base, tt.p))
		})
	}
}

func TestErrorHandler(t *testing.T) {
	initLogger()

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "client went away", err: context.Canceled, wantStatus: api.StatusRequestContextCanceled},
		{name: "wrapped cancellation", err: errors.Join(errors.New("proxy"), context.Canceled), wantStatus: api.StatusRequestContextCanceled},
		{name: "backend failure", err: errors.New("connection refused"), wantStatus: http.StatusBadGateway},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProxy(config.StaticPagesConfig{})
			rr := httptest.NewRecorder()
			p.ErrorHandler(rr, httptest.NewRequest(http.MethodGet, "http://example.com/", nil), tt.err)
			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestProbeFailure(t *testing.T) {
	initLogger()

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "timeout is inconclusive", err: context.DeadlineExceeded, wantStatus: statusProbeInconclusive},
		{name: "canceled is a miss", err: context.Canceled, wantStatus: http.StatusNotFound},
		{name: "refused is a miss", err: errors.New("connection refused"), wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, herr := probeFailure(context.Background(), tt.err, "http://backend/path", time.Second)
			assert.Equal(t, tt.wantStatus, status)
			require.Error(t, herr)
			assert.ErrorIs(t, herr, tt.err)
		})
	}
}

func TestProxyLifecycle(t *testing.T) {
	initLogger()

	p := NewProxy(config.StaticPagesConfig{})
	assert.Error(t, p.Shutdown(), "shutting down a proxy that never started")

	p.ServeAsync("127.0.0.1:0")
	assert.NoError(t, p.Shutdown())
}

func TestProxyServeOnBusyPort(t *testing.T) {
	initLogger()

	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = busy.Close() })

	herr := NewProxy(config.StaticPagesConfig{}).Serve(busy.Addr().String())
	require.Error(t, herr)
	assert.Contains(t, herr.Error(), "Unable to start reverse proxy")
}
