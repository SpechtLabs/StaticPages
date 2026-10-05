package api

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"maps"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeIssuer is an OIDC provider that signs whatever claims a test asks for.
type fakeIssuer struct {
	server *httptest.Server
	key    *rsa.PrivateKey
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		want    string
		wantErr bool
	}{
		{name: "bearer token", header: "Bearer abc.def.ghi", want: "abc.def.ghi"},
		{name: "empty header", header: "", wantErr: true},
		{name: "other scheme", header: "Basic dXNlcjpwYXNz", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractBearerToken(tt.header)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMetadataFromClaims(t *testing.T) {
	date := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		claims          map[string]any
		wantErr         string
		wantRepository  string
		wantCommit      string
		wantBranch      string
		wantEnvironment string
	}{
		{
			name: "deployment",
			claims: map[string]any{
				"repository":  "spechtlabs/site",
				"sha":         "abc123",
				"ref":         "refs/heads/feature",
				"environment": "preview",
			},
			wantRepository:  "spechtlabs/site",
			wantCommit:      "abc123",
			wantBranch:      "feature",
			wantEnvironment: "preview",
		},
		{
			name: "no environment",
			claims: map[string]any{
				"repository": "spechtlabs/site",
				"sha":        "abc123",
				"ref":        "main",
			},
			wantRepository: "spechtlabs/site",
			wantCommit:     "abc123",
			wantBranch:     "main",
		},
		{
			name:    "no repository",
			claims:  map[string]any{"sha": "abc123", "ref": "main"},
			wantErr: "failed to extract repository claim",
		},
		{
			name:    "commit is not a string",
			claims:  map[string]any{"repository": "spechtlabs/site", "sha": 42, "ref": "main"},
			wantErr: "failed to extract commit claim",
		},
		{
			name:    "no branch",
			claims:  map[string]any{"repository": "spechtlabs/site", "sha": "abc123"},
			wantErr: "failed to extract branch claim",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata, err := metadataFromClaims(rawClaims(t, tt.claims), githubClaims(), date)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantRepository, metadata.Repository())
			assert.Equal(t, tt.wantCommit, metadata.SHA())
			assert.Equal(t, tt.wantBranch, metadata.Branch)
			assert.Equal(t, tt.wantEnvironment, metadata.Environment)
			assert.Equal(t, date, metadata.Date)
		})
	}
}

func TestVerifyAgainstIssuers(t *testing.T) {
	date := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	trusted := newFakeIssuer(t)
	other := newFakeIssuer(t)
	unreachable := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(unreachable.Close)

	claims := map[string]any{"repository": "spechtlabs/site", "sha": "abc123", "ref": "refs/heads/main"}

	tests := []struct {
		name    string
		issuers []string
		token   string
		wantErr string
	}{
		{
			name:    "the only issuer accepts",
			issuers: []string{trusted.server.URL},
			token:   trusted.token(t, claims),
		},
		{
			name:    "one of several accepts",
			issuers: []string{trusted.server.URL, other.server.URL, unreachable.URL},
			token:   trusted.token(t, claims),
		},
		{
			name:    "token from an unconfigured issuer",
			issuers: []string{other.server.URL, unreachable.URL},
			token:   trusted.token(t, claims),
			wantErr: "none of the configured OIDC providers accepted the token",
		},
		{
			name:    "accepted token without a commit",
			issuers: []string{trusted.server.URL},
			token:   trusted.token(t, map[string]any{"repository": "spechtlabs/site", "ref": "main"}),
			wantErr: "none of the configured OIDC providers accepted the token",
		},
		{
			name:    "no issuers",
			wantErr: "no OIDC issuer is configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issuerSet := make(map[string]config.ClaimMap, len(tt.issuers))
			for _, issuer := range tt.issuers {
				issuerSet[issuer] = githubClaims()
			}

			metadata, err := verifyAgainstIssuers(context.Background(), tt.token, issuerSet, date)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "spechtlabs/site", metadata.Repository())
			assert.Equal(t, "abc123", metadata.SHA())
			assert.Equal(t, "main", metadata.Branch)
		})
	}
}

// A client that goes away stops the verification rather than leaving it to
// the timeout.
func TestVerifyAgainstIssuersCanceled(t *testing.T) {
	hanging := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(hanging.Close)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(50*time.Millisecond, cancel)

	_, err := verifyAgainstIssuers(ctx, "token", map[string]config.ClaimMap{hanging.URL: githubClaims()}, time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled while verifying token")
}

func newFakeIssuer(t *testing.T) *fakeIssuer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	issuer := &fakeIssuer{key: key}
	mux := http.NewServeMux()
	issuer.server = httptest.NewServer(mux)
	t.Cleanup(issuer.server.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"issuer":                                issuer.server.URL,
			"jwks_uri":                              issuer.server.URL + "/jwks",
			"authorization_endpoint":                issuer.server.URL + "/authorize",
			"token_endpoint":                        issuer.server.URL + "/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{"keys": []map[string]string{{
			"kty": "RSA",
			"alg": "RS256",
			"use": "sig",
			"kid": "test",
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})

	return issuer
}

// token returns an ID token for claims, issued by this issuer and valid for an
// hour.
func (f *fakeIssuer) token(t *testing.T, claims map[string]any) string {
	t.Helper()

	payload := map[string]any{
		"iss": f.server.URL,
		"aud": "staticpages",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	maps.Copy(payload, claims)

	header, err := json.Marshal(map[string]string{"alg": "RS256", "kid": "test", "typ": "JWT"})
	require.NoError(t, err)
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	require.NoError(t, err)

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func githubClaims() config.ClaimMap {
	return config.ClaimMap{
		config.RepositoryClaim:  "repository",
		config.CommitClaim:      "sha",
		config.BranchClaim:      "ref",
		config.EnvironmentClaim: "environment",
	}
}

func rawClaims(t *testing.T, claims map[string]any) map[string]json.RawMessage {
	t.Helper()

	raw := make(map[string]json.RawMessage, len(claims))
	for k, v := range claims {
		b, err := json.Marshal(v)
		require.NoError(t, err)
		raw[k] = b
	}
	return raw
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	assert.NoError(t, json.NewEncoder(w).Encode(v))
}
