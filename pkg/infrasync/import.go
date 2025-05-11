package infrasync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/priyanshujain/infrasync/internal/config"
	"github.com/priyanshujain/infrasync/internal/initialize"
	"github.com/priyanshujain/infrasync/internal/providers"
	"github.com/priyanshujain/infrasync/internal/providers/google"
	"github.com/priyanshujain/infrasync/internal/tfimport"
)

// Client represents the InfraSync client
type Client struct {
	Config config.Config
}

// NewClient creates a new InfraSync client with the provided configuration
func NewClient(cfg config.Config) *Client {
	return &Client{
		Config: cfg,
	}
}

// DefaultClient creates a client with configuration loaded from the default path
func DefaultClient() (*Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return NewClient(cfg), nil
}

// Initialize creates a new IaC repository with Terraform configurations
func (c *Client) Initialize(ctx context.Context) error {
	outputPath := c.Config.ProjectPath()

	absOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output: %w", err)
	}

	// Check if output directory is empty
	if _, err := os.Stat(absOutputPath); err == nil {
		entries, err := os.ReadDir(absOutputPath)
		if err != nil {
			return fmt.Errorf("failed to read output directory: %w", err)
		}
		if len(entries) > 0 {
			return fmt.Errorf("output directory is not empty: %s", absOutputPath)
		}
	}

	err = initialize.Init(c.Config)
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	slog.Info("Initialization completed.")
	return nil
}

// Import imports cloud resources and generates Terraform code
func (c *Client) Import(ctx context.Context) error {
	absOutputPath := c.Config.ProjectPath()
	provider := c.Config.DefaultProvider()

	resourcesDir := filepath.Join(absOutputPath, "resources", provider.Type.String(), provider.ProjectID)

	for _, dir := range []string{resourcesDir} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}
	}

	services := c.Config.GoogleServices(provider)

	for _, service := range services {
		serviceResourcesDir := filepath.Join(resourcesDir, service.String())

		for _, dir := range []string{serviceResourcesDir} {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create service directory: %w", err)
				}
			}
		}

		if err := c.ImportService(ctx, service); err != nil {
			return fmt.Errorf("failed to process service: %w", err)
		}
	}

	return nil
}

// ImportService imports resources for a specific service
func (c *Client) ImportService(ctx context.Context, service google.Service) error {
	path := c.Config.ProjectPath()
	provider := c.Config.DefaultProvider()

	absOutputPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output: %w", err)
	}

	tf, err := tfimport.NewImporter(absOutputPath)
	if err != nil {
		return fmt.Errorf("failed to create Terraform generator: %w", err)
	}

	runner, err := tfimport.New(absOutputPath)
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	if err := runner.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize runner: %w", err)
	}

	var s google.ResourceImporter
	switch service {
	case "pubsub":
		s, err = google.NewPubsub(ctx, providers.Provider{
			Type: providers.ProviderTypeGoogle, ProjectID: provider.ProjectID})
		if err != nil {
			return fmt.Errorf("failed to create PubSub client: %w", err)
		}
	case "cloudsql":
		s, err = google.NewCloudSQL(ctx, providers.Provider{
			Type: providers.ProviderTypeGoogle, ProjectID: provider.ProjectID})
		if err != nil {
			return fmt.Errorf("failed to create CloudSQL client: %w", err)
		}
	case "storage":
		s, err = google.NewStorage(ctx, providers.Provider{
			Type: providers.ProviderTypeGoogle, ProjectID: provider.ProjectID})
		if err != nil {
			return fmt.Errorf("failed to create Storage client: %w", err)
		}
	default:
		slog.Info("Service is not supported", "service", service)
		return nil
	}

	resourceIter, err := s.Import(ctx)
	if err != nil {
		return fmt.Errorf("failed to create resource iterator: %w", err)
	}
	defer resourceIter.Close()

	var count int
	for {
		resource, err := resourceIter.Next(ctx)
		if err != nil {
			return fmt.Errorf("error getting next resource: %w", err)
		}

		if resource == nil {
			break
		}

		if err := tf.SaveImportBlock(*resource); err != nil {
			return fmt.Errorf("failed to save import block: %w", err)
		}

		if err := runner.Import(ctx, *resource); err != nil {
			if errors.Is(err, tfimport.ErrAlreadyExists) {
				slog.Info("Resource already exists", "resource", resource.ID)
			} else {
				return fmt.Errorf("failed to import resource: %w", err)
			}
		}

		if err := runner.CleanupImportBlocks(*resource); err != nil {
			return fmt.Errorf("failed to cleanup import blocks: %w", err)
		}

		count++
		slog.Info("Imported resource", "count", count, "resource", resource.ID)
	}

	return nil
}