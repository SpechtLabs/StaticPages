package config

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
