package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/sierrasoftworks/humane-errors-go"
)

type Page struct {
	Domain     string               `yaml:"domain"`
	Bucket     BucketConfig         `yaml:"bucket"`
	Proxy      PageProxy            `yaml:"proxy"`
	History    int                  `yaml:"history"`
	Auth       AuthorizationConfig  `yaml:"auth"`
	SubDomains map[string]SubDomain `yaml:"subDomains"`
}

func (p *Page) Validate() humane.Error {
	if err := p.Bucket.Validate(); err != nil {
		return humane.Wrap(err, "Invalid bucket configuration", err.Advice()...)
	}

	if err := p.Proxy.Validate(); err != nil {
		return humane.Wrap(err, "Invalid proxy configuration", err.Advice()...)
	}

	if p.History < 0 {
		return humane.New("Invalid number of revisions to keep ", "history must be a non-negative value")
	}

	if err := p.Auth.Validate(); err != nil {
		return humane.Wrap(err, "Invalid authorization configuration", err.Advice()...)
	}

	return nil
}

func (p *Page) Parse() humane.Error {
	if err := p.Proxy.Parse(); err != nil {
		return humane.Wrap(err, "Invalid proxy configuration", err.Advice()...)
	}

	return nil
}

type BucketConfig struct {
	URL           EnvValue `yaml:"url"`
	Name          EnvValue `yaml:"name"`
	ApplicationID EnvValue `yaml:"applicationId"`
	Secret        EnvValue `yaml:"secret"`
	Region        EnvValue `yaml:"region"`
}

func (bc *BucketConfig) Validate() humane.Error {
	if bc.URL == "" {
		return humane.New("No S3 Bucket URL is provided", "Make sure to provide a S3 bucket URL in pages[].bucket.url")
	}

	if _, err := url.Parse(bc.URL.String()); err != nil {
		return humane.Wrap(err, "Invalid S3 Endpoint", "Make sure the S3 endpoint URL is valid")
	}

	if bc.Name == "" {
		return humane.New("No S3 Bucket Name is provided", "Make sure to provide a S3 bucket Name in pages[].bucket.name")
	}

	if bc.ApplicationID == "" {
		return humane.New("No S3 Application ID is provided", "Make sure to provide a S3 Application ID in pages[].bucket.application_id")
	}

	if bc.Secret == "" {
		return humane.New("No S3 Secret is provided", "Make sure to provide a S3 Secret in pages[].bucket.secret")
	}

	return nil
}

type PageProxy struct {
	URL        EnvValue `yaml:"url"`
	Path       EnvValue `yaml:"path"`
	SearchPath []string `yaml:"searchPath"`
	NotFound   string   `yaml:"notFound"`
}

func (pc *PageProxy) Validate() humane.Error {
	if pc.URL == "" {
		return humane.New("No Reverse-Proxy URL is provided", "Make sure to provide a valid URL for which the reverse proxy is created in in pages[].proxy.url")
	}

	if pc.URL.Validate() != nil {
		return humane.New("Invalid Reverse-Proxy URL is provided", "Make sure to provide a valid URL for which the reverse proxy is created in in pages[].proxy.url")
	}

	if _, err := url.Parse(pc.URL.String()); err != nil {
		return humane.New("Invalid Reverse-Proxy URL is provided", "Make sure to provide a valid URL for which the reverse proxy is created in in pages[].proxy.url")
	}

	if pc.Path == "" {
		return humane.New("No Reverse-Proxy Path is provided", "Make sure to provide a URL path for which the reverse proxy is created in in pages[].proxy.path")
	}

	if pc.Path.Validate() != nil {
		return humane.New("Invalid Reverse-Proxy Path is provided", "Make sure to provide a URL path for which the reverse proxy is created in in pages[].proxy.path")
	}

	if len(pc.SearchPath) == 0 {
		return humane.New("No search paths specified", "Make sure to provide a list of search paths in pages[].proxy.searchPath")
	}

	return nil
}

func (pc *PageProxy) Parse() humane.Error {
	if len(pc.SearchPath) == 0 {
		pc.SearchPath = []string{
			"/index.html",
			"/index.htm",
		}
	}

	if pc.NotFound == "" {
		pc.NotFound = "404.html"
	}

	return nil
}

type SubDomain struct {
	Pattern string `yaml:"pattern"`
	History int    `yaml:"history"`
}

type AuthorizationConfig struct {
	Provider   string `yaml:"provider"`
	Repository string `yaml:"repository"`
}

func (ac *AuthorizationConfig) Validate() humane.Error {
	if ac.Provider == "" {
		return humane.New("No Provider is provided", "Make sure to provide a Authorization Provider in pages[].auth.provider (currently only github is supported)")
	}

	if ac.Provider != "github" {
		return humane.New("Invalid Provider is provided", "Make sure to provide a Authorization Provider in pages[].auth.provider (currently only github is supported)")
	}

	if ac.Repository == "" {
		return humane.New("No Repository is provided", "Make sure to provide a Repository in pages[].auth.repository")
	}

	return nil
}

func (ac *AuthorizationConfig) GetAudience() (string, humane.Error) {
	switch ac.Provider {
	case "github":
		return fmt.Sprintf("https://github.com/%s", strings.Split(ac.Repository, "/")[0]), nil

	default:
		return "", humane.New("Invalid Provider is provided", "Make sure to provide a Authorization Provider in pages[].auth.provider (currently only github is supported)")
	}
}
