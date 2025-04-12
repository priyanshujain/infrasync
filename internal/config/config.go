package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/priyanshujain/infrasync/internal/providers"
	"github.com/priyanshujain/infrasync/internal/providers/google"
	"gopkg.in/yaml.v3"
)

type cfg struct {
	Name      string `yaml:"name"`
	Path      string `yaml:"path"`
	Providers map[string]struct {
		Projects []struct {
			ID       string   `yaml:"id"`
			Region   string   `yaml:"region"`
			Services []string `yaml:"services"`
		} `yaml:"projects"`
		Credentials string `yaml:"credentials,omitempty"`
	} `yaml:"providers"`
	Backend struct {
		Type       string `yaml:"type"`
		BucketName string `yaml:"bucket"`
	} `yaml:"backend"`
}

type Config struct {
	Name      string
	Path      string
	Providers []providers.Provider
	cfg       cfg
}

func Load() (Config, error) {
	path, err := defaultConfigPath()
	if err != nil {
		return Config{}, fmt.Errorf("failed to get default config path: %w", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return Config{}, fmt.Errorf("failed to create config directory: %w", err)
		}

		if _, err := os.Create(path); err != nil {
			return Config{}, fmt.Errorf("failed to create config file: %w", err)
		}

		fmt.Printf("Config file created at %s. Please fill in the required fields.\n", path)
		fmt.Println("Template:")
		fmt.Print(Template)
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
				Region:    project.Region,
			})
		}
	}

	c := Config{
		Name:      config.Name,
		Path:      config.Path,
		Providers: ps,
		cfg:       config,
	}

	if err := c.validateGoogleCredentials(); err != nil {
		return Config{}, fmt.Errorf("failed to validate google credentials: %w", err)
	}

	return c, nil
}

func validateConfig(config *cfg) error {
	if config.Name == "" {
		return fmt.Errorf("name is required")
	}
	if config.Path == "" {
		return fmt.Errorf("path is required")
	}
	if _, err := os.Stat(config.Path); os.IsNotExist(err) {
		return fmt.Errorf("path %s does not exist", config.Path)
	}
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

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return "", fmt.Errorf("failed to create config directory: %w", err)
		}
		if _, err := os.Create(path); err != nil {
			return "", fmt.Errorf("failed to create config file: %w", err)
		}

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

func (c *Config) ProjectPath() string {
	return filepath.Join(c.Path, c.Name)
}

func (c *Config) DefaultProvider() providers.Provider {
	if len(c.Providers) == 0 {
		return providers.Provider{}
	}
	return c.Providers[0]
}

func (c *Config) DefaultBackend() providers.Backend {
	if c.cfg.Backend.Type == "" {
		return providers.Backend{}
	}

	return providers.Backend{
		Type:   providers.BackendTypeGCS,
		Bucket: c.cfg.Backend.BucketName,
	}
}

func (c *Config) validateGoogleCredentials() error {

	path := c.cfg.Providers[providers.ProviderTypeGoogle.String()].Credentials
	if path != "" {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			return fmt.Errorf("credentials file does not exist: %s", absPath)
		}

		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", absPath)
	}

	err := google.ValidateCredentials()
	if err != nil {
		return fmt.Errorf("failed to validate credentials: %w", err)
	}

	bucketName := c.DefaultBackend().Bucket
	if err := google.ValidateBackend(bucketName); err != nil {
		return fmt.Errorf("failed to validate backend: %w", err)
	}

	return nil
}
