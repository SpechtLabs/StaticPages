package config

type PreviewConfig struct {
	Enabled bool            `yaml:"enabled"`
	History bool            `yaml:"history"`
	Domains []PreviewDomain `yaml:"domains"`
}

type PreviewDomain struct {
	Pattern string `yaml:"pattern"`
	When    string `yaml:"when"`
	History int    `yaml:"history"`
}
