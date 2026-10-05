package api

import (
	"net"
	"testing"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestApiLifecycle(t *testing.T) {
	api := NewRestApi(config.StaticPagesConfig{})
	assert.Error(t, api.Shutdown(), "shutting down a server that never started")

	api.ServeAsync("127.0.0.1:0")
	assert.NoError(t, api.Shutdown())
}

func TestRestApiServeOnBusyPort(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = busy.Close() })

	herr := NewRestApi(config.StaticPagesConfig{}).Serve(busy.Addr().String())
	require.Error(t, herr)
	assert.Contains(t, herr.Error(), "Unable to start API server")
}

func TestCollectUniqueIssuers(t *testing.T) {
	tests := []struct {
		name        string
		git         config.GitConfig
		wantIssuers []string
		wantErr     string
	}{
		{
			name:        "github",
			git:         config.GitConfig{Provider: "github"},
			wantIssuers: []string{"https://token.actions.githubusercontent.com"},
		},
		{
			name:    "unknown provider",
			git:     config.GitConfig{Provider: "gitea"},
			wantErr: "failed to get OIDC issuer",
		},
		{
			name:    "custom provider without claim mappings",
			git:     config.GitConfig{Provider: "custom", Oidc: config.GitProvider{Issuer: "https://issuer.example"}},
			wantErr: "failed to get OIDC claim mapping",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issuers, err := collectUniqueIssuers([]*config.Page{{Git: tt.git}, {Git: tt.git}})
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Len(t, issuers, len(tt.wantIssuers), "issuers are deduplicated")
			for _, issuer := range tt.wantIssuers {
				assert.Contains(t, issuers, issuer)
			}
		})
	}
}
