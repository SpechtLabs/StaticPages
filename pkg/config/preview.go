package config

// PreviewConfig selects which preview subdomains a page serves besides its main
// branch: one per commit, per environment, or per branch.
type PreviewConfig struct {
	Enabled      bool `yaml:"enabled"`
	CommitSha    bool `yaml:"sha"`
	Environments bool `yaml:"environment"`
	Branch       bool `yaml:"branch"`
}
