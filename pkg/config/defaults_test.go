package config_test

import (
	"strings"
	"testing"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// load parses the YAML, applies page defaults, and unmarshals into the config.
func load(t *testing.T, yamlCfg string) config.StaticPagesConfig {
	t.Helper()
	v := viper.New()
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(strings.NewReader(yamlCfg)))

	config.ApplyPageDefaults(v)

	var cfg config.StaticPagesConfig
	require.NoError(t, v.Unmarshal(&cfg))
	return cfg
}

func TestApplyPageDefaults_InheritsAndDeepMerges(t *testing.T) {
	cfg := load(t, `
pageDefaults:
  bucket:
    region: eu-central-003
    name: cedi-testing
    applicationId: ENV(APP_ID)
    secret: ENV(S3_SECRET)
  proxy:
    url: https://cdn.specht-labs.de
    path: file/cedi-testing
    notFound: 404.html
    searchPath: [.html, .htm, /index.html]
  git:
    provider: github
    mainBranch: main
  preview:
    enabled: true
    branch: true

pages:
  - domain: prose.specht-labs.de
    git:
      repository: SpechtLabs/prose
`)

	require.Len(t, cfg.Pages, 1)
	p := cfg.Pages[0]

	assert.Equal(t, config.FromString("prose.specht-labs.de"), p.Domain)
	// git: page set repository, inherited provider + mainBranch (deep merge)
	assert.Equal(t, "SpechtLabs/prose", p.Git.Repository)
	assert.Equal(t, "github", p.Git.Provider)
	assert.Equal(t, "main", p.Git.MainBranch)
	// whole blocks inherited
	assert.Equal(t, "https://cdn.specht-labs.de", p.Proxy.URL.String())
	assert.Equal(t, "404.html", p.Proxy.NotFound)
	assert.Equal(t, []string{".html", ".htm", "/index.html"}, p.Proxy.SearchPath)
	assert.Equal(t, "cedi-testing", p.Bucket.Name.String())
	assert.True(t, p.Preview.Enabled)
	assert.True(t, p.Preview.Branch)
}

func TestApplyPageDefaults_PageOverridesWin(t *testing.T) {
	cfg := load(t, `
pageDefaults:
  proxy:
    url: https://cdn.specht-labs.de
    notFound: 404.html
    searchPath: [.html, .htm, /index.html]
  preview:
    enabled: true
    branch: true

pages:
  - domain: dev.specht-labs.de
    proxy:
      searchPath: [/index.html]
    preview:
      enabled: false
`)

	p := cfg.Pages[0]
	// list is replaced, not merged
	assert.Equal(t, []string{"/index.html"}, p.Proxy.SearchPath)
	// sibling proxy fields still inherited (deep merge of the proxy block)
	assert.Equal(t, "https://cdn.specht-labs.de", p.Proxy.URL.String())
	assert.Equal(t, "404.html", p.Proxy.NotFound)
	// explicit false overrides a true default; unspecified branch stays inherited
	assert.False(t, p.Preview.Enabled)
	assert.True(t, p.Preview.Branch)
}

func TestApplyPageDefaults_NoCrossContamination(t *testing.T) {
	cfg := load(t, `
pageDefaults:
  git:
    provider: github
    mainBranch: main

pages:
  - domain: a.specht-labs.de
    git:
      repository: org/a
  - domain: b.specht-labs.de
    git:
      repository: org/b
      mainBranch: develop
`)

	require.Len(t, cfg.Pages, 2)
	assert.Equal(t, "org/a", cfg.Pages[0].Git.Repository)
	assert.Equal(t, "main", cfg.Pages[0].Git.MainBranch)
	assert.Equal(t, "org/b", cfg.Pages[1].Git.Repository)
	assert.Equal(t, "develop", cfg.Pages[1].Git.MainBranch) // override does not leak to page a
}

func TestApplyPageDefaults_NoDefaultsIsUnchanged(t *testing.T) {
	cfg := load(t, `
pages:
  - domain: solo.specht-labs.de
    git:
      provider: github
      repository: org/solo
      mainBranch: main
    proxy:
      url: https://cdn.specht-labs.de
`)

	require.Len(t, cfg.Pages, 1)
	assert.Equal(t, "org/solo", cfg.Pages[0].Git.Repository)
	assert.Equal(t, "https://cdn.specht-labs.de", cfg.Pages[0].Proxy.URL.String())
}
