package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/priyanshujain/infrasync/internal/providers"
	"github.com/priyanshujain/infrasync/internal/providers/google"
	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type cfg struct {
	Providers map[string]struct {
		Projects []struct {
			ID       string   `yaml:"id"`
			Region   string   `yaml:"region"`
			Services []string `yaml:"services"`
		} `yaml:"projects"`
		Credentials string `yaml:"credentials,omitempty"`
	} `yaml:"providers"`
}

type Config struct {
	Providers []providers.Provider
	cfg       cfg
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(path string) (Config, error) {
	if path == "" {
		p, err := defaultConfigPath()
		if err != nil {
			return Config{}, fmt.Errorf("failed to get default config path: %w", err)
		}
		path = p
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("error reading config file: %w", err)
	}

	var config cfg
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("error parsing config file: %w", err)
	}

	if err := validateConfig(&config); err != nil {
		return Config{}, err
	}

	var ps []providers.Provider
	for name, provider := range config.Providers {
		if providers.ProviderTypeGoogle.String() != name {
			return Config{}, fmt.Errorf("unsupported provider: %s", name)
		}
		for _, project := range provider.Projects {
			ps = append(ps, providers.Provider{
				Type:      providers.ProviderTypeGoogle,
				ProjectID: project.ID,
			})
		}
	}

	return Config{
		Providers: ps,
		cfg:       config,
	}, nil
}

// validateConfig ensures the configuration is valid
func validateConfig(config *cfg) error {
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

func defaultConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	path := filepath.Join(homeDir, ".config", "infrasync", "config.yaml")

	// if the file does not exist, create it
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return "", fmt.Errorf("failed to create config directory: %w", err)
		}
		if _, err := os.Create(path); err != nil {
			return "", fmt.Errorf("failed to create config file: %w", err)
		}

		// write default config
		defaultConfig := `providers:`
		err = os.WriteFile(path, []byte(defaultConfig), 0644)
		if err != nil {
			return "", fmt.Errorf("failed to write default config: %w", err)
		}
	}
	return path, nil
}

func (c *Config) GoogleServices(p providers.Provider) []google.Service {
	var services []google.Service
	for _, project := range c.cfg.Providers[p.Type.String()].Projects {
		for _, service := range project.Services {
			services = append(services, google.Service(service))
		}
	}
	return services
}
