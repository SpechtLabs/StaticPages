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
	History    int                  `yaml:"history"`
	Allowed    []string             `yaml:"allowed"`
	SubDomains map[string]SubDomain `yaml:"subDomains"`
}

func (p *Page) Validate() humane.Error {
	if err := p.Bucket.Validate(); err != nil {
		return humane.Wrap(err, "Invalid bucket configuration", err.Advice()...)
	}

	if p.History < 0 {
		return humane.New("Invalid number of revisions to keep ", "history must be a non-negative value")
	}

	return nil
}

type BucketConfig struct {
	URL           EnvValue `yaml:"url"`
	Name          EnvValue `yaml:"name"`
	Page          EnvValue `yaml:"page"`
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

	if bc.Page == "" {
		return humane.New("No Page Name is provided", "Make sure to provide the name of the web-page in the S3 bucket in pages[].bucket.page")
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

	if err := bc.Page.Parse(); err != nil {
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

type SubDomain struct {
	Pattern string `yaml:"pattern"`
	History int    `yaml:"history"`
}

func ParsePages() ([]Page, humane.Error) {
	pages := make([]Page, 0)

	err := viper.UnmarshalKey("pages", &pages)
	if err != nil {
		return nil, humane.Wrap(err, "Unable to parse pages pages", "Make sure the config file is valid.")
	}

	// Parse each page configuration
	for i, page := range pages {
		if err := page.Bucket.Parse(); err != nil {
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
