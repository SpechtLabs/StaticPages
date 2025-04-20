package config

import (
	"fmt"
	humane "github.com/sierrasoftworks/humane-errors-go"
	"github.com/spf13/viper"
	"net/url"
)

type Page struct {
	Domain     string               `yaml:"domain"`
	Bucket     BucketConfig         `yaml:"bucket"`
	Proxy      ProxyConfig          `yaml:"proxy"`
	History    int                  `yaml:"history"`
	Repository string               `yaml:"repository"`
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

	return nil
}

func (p *Page) Parse() humane.Error {
	if err := p.Bucket.Parse(); err != nil {
		return humane.Wrap(err, "Invalid bucket configuration", err.Advice()...)
	}

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

func (bc *BucketConfig) Parse() humane.Error {
	if err := bc.URL.Parse(); err != nil {
		return err
	}

	if err := bc.Name.Parse(); err != nil {
		return err
	}

	if err := bc.ApplicationID.Parse(); err != nil {
		return err
	}

	if err := bc.Secret.Parse(); err != nil {
		return err
	}

	return nil
}

type ProxyConfig struct {
	URL        EnvValue `yaml:"url"`
	Path       EnvValue `yaml:"path"`
	SearchPath []string `yaml:"searchPath"`
}

func (pc *ProxyConfig) Validate() humane.Error {
	if pc.URL == "" {
		return humane.New("No Reverse-Proxy URL is provided", "Make sure to provide a valid URL for which the reverse proxy is created in in pages[].proxy.url")
	}

	if _, err := url.Parse(pc.URL.String()); err != nil {
		return humane.New("No Reverse-Proxy URL is provided", "Make sure to provide a valid URL for which the reverse proxy is created in in pages[].proxy.url")
	}

	if pc.Path == "" {
		return humane.New("No Reverse-Proxy path is provided", "Make sure to provide a URL path for which the reverse proxy is created in in pages[].proxy.path")
	}

	if len(pc.SearchPath) == 0 {
		return humane.New("No search paths specified", "Make sure to provide a list of search paths in pages[].proxy.searchPath")
	}

	return nil
}

func (pc *ProxyConfig) Parse() humane.Error {

	if err := pc.URL.Parse(); err != nil {
		return err
	}

	if err := pc.Path.Parse(); err != nil {
		return err
	}

	if len(pc.SearchPath) == 0 {
		pc.SearchPath = []string{
			"/index.html",
			"/index.htm",
		}
	}

	return nil
}

type SubDomain struct {
	Pattern string `yaml:"pattern"`
	History int    `yaml:"history"`
}

func ParsePages() ([]*Page, humane.Error) {
	pages := make([]*Page, 0)

	err := viper.UnmarshalKey("pages", &pages)
	if err != nil {
		return nil, humane.Wrap(err, "Unable to parse pages pages", "Make sure the config file is valid.")
	}

	// Parse each page configuration
	for i, page := range pages {
		if err := page.Parse(); err != nil {
			return nil, humane.Wrap(err, "Invalid page configuration",
				fmt.Sprintf("Ensure Page %d (%s) configuration is valid: %s", i, page.Domain, err.Advice()))
		}

		// Validate page config
		if err := page.Validate(); err != nil {
			return nil, humane.Wrap(err, "Invalid page configuration",
				fmt.Sprintf("Ensure Page %d (%s) configuration is valid: %s", i, page.Domain, err.Advice()))
		}
	}

	return pages, nil
}
