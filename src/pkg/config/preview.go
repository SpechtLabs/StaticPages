package config

type PreviewConfig struct {
	Enabled      bool `yaml:"enabled"`
	CommitSha    bool `yaml:"sha"`
	Environments bool `yaml:"environment"`
	Branch       bool `yaml:"branch"`
}
