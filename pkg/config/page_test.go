package config_test

import (
	"testing"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestGitConfig_GetOidcIssuer(t *testing.T) {
	tests := []struct {
		name          string
		gitConfig     config.GitConfig
		expectedValue string
		expectError   bool
		errorContains string
	}{
		{
			name: "github provider",
			gitConfig: config.GitConfig{
				Provider: "github",
			},
			expectedValue: "https://token.actions.githubusercontent.com",
			expectError:   false,
		},
		{
			name: "custom provider with issuer",
			gitConfig: config.GitConfig{
				Provider: "custom",
				Oidc: config.GitProvider{
					Issuer: "https://custom.example.com",
				},
			},
			expectedValue: "https://custom.example.com",
			expectError:   false,
		},
		{
			name: "custom provider without issuer",
			gitConfig: config.GitConfig{
				Provider: "custom",
				Oidc: config.GitProvider{
					Issuer: "",
				},
			},
			expectError:   true,
			errorContains: "Invalid Git-Provider 'custom'",
		},
		{
			name: "unsupported provider",
			gitConfig: config.GitConfig{
				Provider: "gitlab",
			},
			expectError:   true,
			errorContains: "Invalid Git-Provider configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issuer, err := tc.gitConfig.GetOidcIssuer()

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
				assert.Empty(t, issuer)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedValue, issuer)
			}
		})
	}
}

func TestGitConfig_GetOidcClaimMapping(t *testing.T) {
	tests := []struct {
		name          string
		gitConfig     config.GitConfig
		expectedKeys  []config.Claim
		expectError   bool
		errorContains string
	}{
		{
			name: "github provider",
			gitConfig: config.GitConfig{
				Provider: "github",
			},
			expectedKeys: []config.Claim{
				config.RepositoryClaim,
				config.CommitClaim,
				config.BranchClaim,
				config.EnvironmentClaim,
			},
			expectError: false,
		},
		{
			name: "custom provider with valid claim mappings",
			gitConfig: config.GitConfig{
				Provider: "custom",
				Oidc: config.GitProvider{
					ClaimMappings: config.ClaimMapRaw{
						"repository":  "repo",
						"commit":      "commitSha",
						"branch":      "branchName",
						"environment": "env",
					},
				},
			},
			expectedKeys: []config.Claim{
				config.RepositoryClaim,
				config.CommitClaim,
				config.BranchClaim,
				config.EnvironmentClaim,
			},
			expectError: false,
		},
		{
			name: "custom provider with empty claim mappings",
			gitConfig: config.GitConfig{
				Provider: "custom",
				Oidc: config.GitProvider{
					ClaimMappings: config.ClaimMapRaw{},
				},
			},
			expectError:   true,
			errorContains: "Invalid Git-Provider 'custom'",
		},
		{
			name: "custom provider with incomplete claim mappings",
			gitConfig: config.GitConfig{
				Provider: "custom",
				Oidc: config.GitProvider{
					ClaimMappings: config.ClaimMapRaw{
						"repository": "repo",
						"commit":     "commitSha",
						// Missing branch
					},
				},
			},
			expectError:   true,
			errorContains: "Invalid ClaimMapping",
		},
		{
			name: "unsupported provider",
			gitConfig: config.GitConfig{
				Provider: "gitlab",
			},
			expectError:   true,
			errorContains: "Invalid Git-Provider configured",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			claimMap, err := tc.gitConfig.GetOidcClaimMapping()

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
				assert.Empty(t, claimMap)
			} else {
				assert.NoError(t, err)
				for _, key := range tc.expectedKeys {
					assert.Contains(t, claimMap, key, "Expected claim mapping to contain key %s", key)
				}
			}
		})
	}
}

func TestClaimMapRaw_AsTyped(t *testing.T) {
	tests := []struct {
		name     string
		raw      config.ClaimMapRaw
		expected map[config.Claim]string
	}{
		{
			name:     "empty map",
			raw:      config.ClaimMapRaw{},
			expected: map[config.Claim]string{
				// Empty map
			},
		},
		{
			name: "map with values",
			raw: config.ClaimMapRaw{
				"repository":  "repo",
				"commit":      "sha",
				"branch":      "ref",
				"environment": "env",
			},
			expected: map[config.Claim]string{
				config.RepositoryClaim:  "repo",
				config.CommitClaim:      "sha",
				config.BranchClaim:      "ref",
				config.EnvironmentClaim: "env",
			},
		},
		{
			name: "map with extra values",
			raw: config.ClaimMapRaw{
				"repository":  "repo",
				"commit":      "sha",
				"branch":      "ref",
				"environment": "env",
				"extra":       "value",
			},
			expected: map[config.Claim]string{
				config.RepositoryClaim:  "repo",
				config.CommitClaim:      "sha",
				config.BranchClaim:      "ref",
				config.EnvironmentClaim: "env",
				"extra":                 "value",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.raw.AsTyped()

			assert.Equal(t, len(tc.expected), len(result), "Maps should have the same length")

			for key, expectedValue := range tc.expected {
				actualValue, exists := result[key]
				assert.True(t, exists, "Key %s should exist in the result", key)
				assert.Equal(t, expectedValue, actualValue, "Values for key %s should match", key)
			}
		})
	}
}
