package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the configuration file
type Config struct {
	BaseBranch      string   `yaml:"base_branch"`
	UpstreamRemote  string   `yaml:"upstream_remote"`
	OriginRemote    string   `yaml:"origin_remote"`
	ExcludePatterns []string `yaml:"exclude_patterns"`
}

// LoadConfig loads config from .gitsync.yaml or returns defaults
func LoadConfig() (*Config, error) {
	config := &Config{
		BaseBranch:      "",
		UpstreamRemote:  "",
		OriginRemote:    "origin",
		ExcludePatterns: []string{},
	}
	
	// Try to load from file
	data, err := os.ReadFile(".gitsync.yaml")
	if err == nil {
		yaml.Unmarshal(data, config)
	}
	
	// Auto-detect if not set
	if config.BaseBranch == "" {
		if branch, err := DetectBaseBranch(); err == nil {
			config.BaseBranch = branch
		}
	}
	
	if config.UpstreamRemote == "" {
		if remote, err := DetectUpstreamRemote(); err == nil {
			config.UpstreamRemote = remote
		} else if err != nil { // Propagate error from DetectUpstreamRemote
			return nil, err
		}
	}
	
	return config, nil
}

// SaveConfig saves config to .gitsync.yaml
func SaveConfig(config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(".gitsync.yaml", data, 0644)
}
