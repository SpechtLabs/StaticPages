package config

import (
	"fmt"
	"strings"

	"github.com/sierrasoftworks/humane-errors-go"
)

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
	Provider   string `yaml:"provider"`
	Repository string `yaml:"repository"`
	MainBranch string `yaml:"mainBranch"`
}

func (g *GitConfig) GetAudience() (string, humane.Error) {
	switch g.Provider {
	case "github":
		return fmt.Sprintf("https://github.com/%s", strings.Split(g.Repository, "/")[0]), nil

	default:
		return "", humane.New("Invalid Provider is provided", "Make sure to provide a Authorization Provider in pages[].auth.provider (currently only github is supported)")
	}
}
