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

type Page struct {
	Domain  DomainScope   `yaml:"domain"`
	Bucket  BucketConfig  `yaml:"bucket"`
	Proxy   PageProxy     `yaml:"proxy"`
	History int           `yaml:"history"`
	Git     GitConfig     `yaml:"git"`
	Preview PreviewConfig `yaml:"preview"`
}

type BucketConfig struct {
	URL           EnvValue `yaml:"url"`
	Name          EnvValue `yaml:"name"`
	ApplicationID EnvValue `yaml:"applicationId"`
	Secret        EnvValue `yaml:"secret"`
	Region        EnvValue `yaml:"region"`
}

type PageProxy struct {
	URL        EnvValue `yaml:"url"`
	Path       EnvValue `yaml:"path"`
	SearchPath []string `yaml:"searchPath"`
	NotFound   string   `yaml:"notFound"`
}

type SubDomain struct {
	Pattern string `yaml:"pattern"`
	History int    `yaml:"history"`
}

type GitConfig struct {
	Provider   string      `yaml:"provider"`
	Repository string      `yaml:"repository"`
	MainBranch string      `yaml:"mainBranch"`
	Oidc       GitProvider `yaml:"oidc"`
}

type GitProvider struct {
	Issuer        string      `yaml:"issuer"`
	ClaimMappings ClaimMapRaw `yaml:"claimMappings"`
}

type Claim string
type ClaimMapRaw map[string]string
type ClaimMap map[Claim]string

const (
	RepositoryClaim  Claim = "repository"
	CommitClaim      Claim = "commit"
	BranchClaim      Claim = "branch"
	EnvironmentClaim Claim = "environment"
)

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

func (cm ClaimMapRaw) AsTyped() ClaimMap {
	out := make(map[Claim]string, len(cm))
	for k, v := range cm {
		out[Claim(k)] = v
	}
	return out
}

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
