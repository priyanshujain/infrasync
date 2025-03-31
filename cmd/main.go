package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/priyanshujain/infrasync/internal/config"
	"github.com/priyanshujain/infrasync/internal/providers"
	"github.com/priyanshujain/infrasync/internal/providers/google"
	"github.com/priyanshujain/infrasync/internal/tfimport"
	"github.com/spf13/cobra"
)

var (
	provider   string
	services   string
	projectID  string
	outputPath string
	cfgFile    string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "infrasync",
		Short: "InfraSync - Convert existing infrastructure to IaC",
		Long:  `InfraSync is a tool for converting existing cloud infrastructure to Terraform code.`,
		RunE:  run,
	}

	rootCmd.Flags().StringVar(&provider, "provider", "google", "Cloud provider (google, aws, azure)")
	rootCmd.Flags().StringVar(&services, "services", "pubsub", "Comma-separated list of services to import")
	rootCmd.Flags().StringVar(&projectID, "project", "", "Cloud project ID")
	rootCmd.Flags().StringVarP(&outputPath, "output", "o", "./terraform", "Path to output directory")
	rootCmd.Flags().StringVarP(&cfgFile, "config", "c", "", "Path to configuration file (default is $HOME/.config/infrasync/config.yaml)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Create absolute path for output
	absOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output: %w", err)
	}

	// Load configuration if specified
	cfg, err := config.LoadFromFile(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create base directory structure
	if err := setupDirectoryStructure(absOutputPath); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}

	cfgProviders := cfg.Providers
	if len(cfgProviders) == 0 {
		return fmt.Errorf("no providers found in configuration")
	}
	if len(cfgProviders) > 1 {
		return fmt.Errorf("multiple providers configuration not supported")
	}
	if provider != "" && provider != cfgProviders[0].Type.String() {
		return fmt.Errorf("unsupported provider: %s", provider)
	}
	if projectID == "" {
		projectID = cfgProviders[0].ProjectID
	}
	if provider == "" {
		provider = cfgProviders[0].Type.String()
		projectID = cfgProviders[0].ProjectID
	}
	cfgServices := cfg.GoogleServices(cfgProviders[0])

	// Parse services
	serviceList := strings.Split(services, ",")
	if len(serviceList) == 0 && len(cfgServices) == 0 {
		return fmt.Errorf("at least one service must be specified")
	}

	ctx := context.Background()

	tf, err := tfimport.New(outputPath, []string{projectID})
	if err != nil {
		return fmt.Errorf("failed to create Terraform generator: %w", err)
	}

	for _, p := range cfg.Providers {
		if projectID == "" || p.ProjectID == projectID {
			if err := processGoogleServices(ctx, serviceList, p.ProjectID, absOutputPath, tf); err != nil {
				return fmt.Errorf("failed to process Google Cloud services: %w", err)
			}
		}
	}

	return nil
}

func setupDirectoryStructure(outputPath string) error {
	// Create minimal terraform structure
	baseDirs := []string{
		outputPath,
	}

	for _, dir := range baseDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func processGoogleServices(ctx context.Context, services []string, projectID, outputPath string, tf tfimport.TerraformGenerator) error {
	slog.Info("Processing Google Cloud services", "project", projectID)

	// create google directory
	gcpDir := filepath.Join(outputPath, "resources", "gcp", projectID)
	// check if directory exists
	if _, err := os.Stat(gcpDir); os.IsNotExist(err) {
		if err := os.MkdirAll(gcpDir, 0755); err != nil {
			return fmt.Errorf("failed to create GCP directory: %w", err)
		}
	}

	for _, service := range services {
		slog.Info("Importing service", "service", service)

		serviceDir := filepath.Join(gcpDir, service)
		if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
			if err := os.MkdirAll(serviceDir, 0755); err != nil {
				return fmt.Errorf("failed to create service directory: %w", err)
			}
		}

		var s google.ResourceImporter
		var err error
		switch service {
		case "pubsub":
			s, err = google.NewPubsub(ctx, providers.Provider{Type: providers.ProviderTypeGoogle, ProjectID: projectID})
			if err != nil {
				return fmt.Errorf("failed to create PubSub client: %w", err)
			}
		default:
			slog.Warn("Service not yet implemented", "service", service)
			continue
		}

		// Get resource iterator
		resourceIter, err := s.Import(ctx)
		if err != nil {
			return fmt.Errorf("failed to create resource iterator: %w", err)
		}
		defer resourceIter.Close()

		// Process resources as they're generated
		var count int
		for {
			resource, err := resourceIter.Next(ctx)
			if err != nil {
				return fmt.Errorf("error getting next resource: %w", err)
			}

			// No more resources
			if resource == nil {
				break
			}

			// Save the import block immediately
			if err := tf.SaveImportBlock(*resource); err != nil {
				return fmt.Errorf("failed to save import block: %w", err)
			}

			count++
			if count%10 == 0 {
				slog.Info("Import progress", "service", service, "resourcesImported", count)
			}
			if count == 1 {
				break
			}
		}

		slog.Info("Import complete", "service", service, "resourcesImported", count)
	}

	return nil
}
