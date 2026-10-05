package config

import (
	"fmt"

	"github.com/sierrasoftworks/humane-errors-go"
)

const exampleGitProviderOidcConfig = `
provider:
  oidc:
    issuer: https://token.actions.githubusercontent.com
    claimMappings:
      Repository: repository
      Commit: sha
      Branch: ref
      Environment: environment
`

// Page is one site StaticPages serves: the domain it answers on, the bucket
// its uploads live in, the backend it proxies to, and the repository allowed
// to publish it.
type Page struct {
	Bucket  BucketConfig  `yaml:"bucket"`
	Git     GitConfig     `yaml:"git"`
	Domain  DomainScope   `yaml:"domain"`
	Proxy   PageProxy     `yaml:"proxy"`
	History int           `yaml:"history"`
	Preview PreviewConfig `yaml:"preview"`
}

// BucketConfig locates the S3-compatible bucket a page is uploaded to.
type BucketConfig struct {
	URL           EnvValue `yaml:"url"`
	Name          EnvValue `yaml:"name"`
	ApplicationID EnvValue `yaml:"applicationId"`
	Secret        EnvValue `yaml:"secret"`
	Region        EnvValue `yaml:"region"`
}

// PageProxy configures where the proxy fetches a page's objects from and how it
// resolves request paths to them.
type PageProxy struct {
	URL        EnvValue `yaml:"url"`
	Path       EnvValue `yaml:"path"`
	NotFound   string   `yaml:"notFound"`
	SearchPath []string `yaml:"searchPath"`
}

// SubDomain configures a subdomain pattern and how much history it keeps.
type SubDomain struct {
	Pattern string `yaml:"pattern"`
	History int    `yaml:"history"`
}

// GitConfig names the repository allowed to publish a page and the provider
// whose OIDC tokens prove it.
type GitConfig struct {
	Oidc       GitProvider `yaml:"oidc"`
	Provider   string      `yaml:"provider"`
	Repository string      `yaml:"repository"`
	MainBranch string      `yaml:"mainBranch"`
}

// GitProvider configures a custom OIDC issuer and how its token claims map to
// the claims StaticPages reads.
type GitProvider struct {
	ClaimMappings ClaimMapRaw `yaml:"claimMappings"`
	Issuer        string      `yaml:"issuer"`
}

// Claim names a piece of information StaticPages reads from an OIDC token.
type Claim string

// ClaimMapRaw maps claim names, as written in the configuration, to token claims.
type ClaimMapRaw map[string]string

// ClaimMap maps each Claim to the name of the token claim that carries it.
type ClaimMap map[Claim]string

const (
	// RepositoryClaim is the repository that published the upload.
	RepositoryClaim Claim = "repository"
	// CommitClaim is the commit the upload was built from.
	CommitClaim Claim = "commit"
	// BranchClaim is the branch the commit is on.
	BranchClaim Claim = "branch"
	// EnvironmentClaim is the deployment environment, if any.
	EnvironmentClaim Claim = "environment"
)

// AllClaims lists every claim a custom provider has to map.
var AllClaims = []Claim{
	RepositoryClaim,
	CommitClaim,
	BranchClaim,
	EnvironmentClaim,
}

var githubClaimMap = ClaimMap{
	RepositoryClaim:  "repository",
	CommitClaim:      "sha",
	BranchClaim:      "ref",
	EnvironmentClaim: "environment",
}

// AsTyped converts the configured mapping to a ClaimMap.
func (cm ClaimMapRaw) AsTyped() ClaimMap {
	out := make(map[Claim]string, len(cm))
	for k, v := range cm {
		out[Claim(k)] = v
	}
	return out
}

// GetOidcIssuer returns the OIDC issuer of the configured provider.
func (g *GitConfig) GetOidcIssuer() (string, humane.Error) {
	switch g.Provider {
	case "github":
		return "https://token.actions.githubusercontent.com", nil

	case "custom":
		if g.Oidc.Issuer == "" {
			return "", humane.New("Invalid Git-Provider 'custom'",
				"Please provide 'pages[].git.provider.oidc.issuer'",
				fmt.Sprintf("Example:\n%s", exampleGitProviderOidcConfig),
			)
		}
		return g.Oidc.Issuer, nil

	default:
		return "", humane.New("Invalid Git-Provider configured",
			"Please configure a valid Git-Provider in pages[].git.provider",
			"You can use a 'custom' provider to use your own Git-Provider and provide 'pages[].git.provider.oidc.issuer' and 'pages[].git.provider.oidc.claimMappings'")
	}
}

// GetOidcClaimMapping returns how the configured provider's token claims map to
// the claims StaticPages reads.
func (g *GitConfig) GetOidcClaimMapping() (ClaimMap, humane.Error) {
	switch g.Provider {
	case "github":
		return githubClaimMap, nil

	case "custom":
		if len(g.Oidc.ClaimMappings) == 0 {
			return ClaimMap{}, humane.New("Invalid Git-Provider 'custom'",
				"Please provide 'pages[].git.provider.oidc.claimMappings'",
				fmt.Sprintf("Example:\n%s", exampleGitProviderOidcConfig),
			)
		}

		for _, claim := range AllClaims {
			if _, ok := g.Oidc.ClaimMappings[string(claim)]; !ok {
				return ClaimMap{}, humane.New("Invalid ClaimMapping",
					fmt.Sprintf("Please provide 'pages[].git.provider.oidc.claimMappings.%s'", claim),
					fmt.Sprintf("Example:\n%s", exampleGitProviderOidcConfig),
				)
			}
		}

		return g.Oidc.ClaimMappings.AsTyped(), nil

	default:
		return ClaimMap{}, humane.New("Invalid Git-Provider configured",
			"Please configure a valid Git-Provider in pages[].git.provider",
			"You can use a 'custom' provider to use your own Git-Provider and provide 'pages[].git.provider.oidc.issuer' and 'pages[].git.provider.oidc.claimMappings'")
	}
}
