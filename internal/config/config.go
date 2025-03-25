package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Providers map[string]*Provider `yaml:"providers"`
}

// Provider represents a cloud provider configuration
type Provider struct {
	Projects    []*Project `yaml:"projects"`
	Credentials string     `yaml:"credentials,omitempty"`
}

// Project represents a cloud project configuration
type Project struct {
	ID       string   `yaml:"id"`
	Region   string   `yaml:"region"`
	Services []string `yaml:"services"`
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// validateConfig ensures the configuration is valid
func validateConfig(config *Config) error {
	if len(config.Providers) == 0 {
		return fmt.Errorf("no providers configured")
	}

	for name, provider := range config.Providers {
		if len(provider.Projects) == 0 {
			return fmt.Errorf("provider %s has no projects configured", name)
		}

		for _, project := range provider.Projects {
			if project.ID == "" {
				return fmt.Errorf("project in provider %s has no ID", name)
			}
			if len(project.Services) == 0 {
				return fmt.Errorf("project %s in provider %s has no services configured", project.ID, name)
			}
		}
	}

	return nil
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".infrasync", "config.yaml")
}