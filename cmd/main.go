package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/priyanshujain/infrasync/internal/config"
	"github.com/priyanshujain/infrasync/internal/initialize"
	"github.com/priyanshujain/infrasync/internal/providers"
	"github.com/priyanshujain/infrasync/internal/providers/google"
	"github.com/priyanshujain/infrasync/internal/tfimport"
	"github.com/spf13/cobra"
)

var cfg config.Config
var cfgFile string

func main() {
	rootCmd := &cobra.Command{
		Use:   "infrasync",
		Short: "InfraSync - Convert existing infrastructure to IaC",
		Long:  `InfraSync is a tool for converting existing cloud infrastructure to Terraform code.`,
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Path to configuration file (default is $HOME/.config/infrasync/config.yaml)")

	// Import command (existing functionality)
	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import cloud resources and generate Terraform code",
		RunE:  runImport,
	}

	// Init command (new functionality)
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new IaC repository",
		Long:  `Initialize a new Infrastructure as Code repository with Terraform/OpenTofu configurations.`,
		RunE:  runInit,
	}

	// Add commands to root
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(initCmd)

	// read config file
	var err error
	cfg, err = config.LoadFromFile(cfgFile)
	if err != nil {
		fmt.Printf("Error loading config file: %v\n", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runImport(cmd *cobra.Command, args []string) error {
	var absOutputPath = cfg.ProjectPath()
	var provider = cfg.DefaultProvider()

	services := cfg.GoogleServices(provider)

	ctx := context.Background()

	tf, err := tfimport.New(absOutputPath, []string{provider.ProjectID})
	if err != nil {
		return fmt.Errorf("failed to create Terraform generator: %w", err)
	}

	if err := processGoogleServices(ctx, services, provider.ProjectID, absOutputPath, tf); err != nil {
		return fmt.Errorf("failed to process Google services: %w", err)
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

func runInit(cmd *cobra.Command, args []string) error {

	outputPath := cfg.ProjectPath()

	absOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output: %w", err)
	}

	if _, err := os.Stat(absOutputPath); err == nil {
		entries, err := os.ReadDir(absOutputPath)
		if err != nil {
			return fmt.Errorf("failed to read output directory: %w", err)
		}
		if len(entries) > 0 {
			return fmt.Errorf("output directory is not empty: %s", absOutputPath)
		}
	}

	err = initialize.Init(cfg)
	if err != nil {
		panic(err)
	}

	slog.Info("Initialization completed.")
	slog.Info("Next steps:")
	slog.Info("1. Review and edit the generated files")
	slog.Info("2. Run 'infrasync import' to import existing resources")
	slog.Info("3. Run 'tofu init' and 'tofu apply' to apply the configuration")

	return nil
}

func processGoogleServices(ctx context.Context, services []google.Service, projectID, outputPath string, tf tfimport.TerraformGenerator) error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("Processing Google Cloud services", "project", projectID)

	// Create directories
	importsDir := filepath.Join(outputPath, "imports", "gcp", projectID)
	resourcesDir := filepath.Join(outputPath, "resources", "gcp", projectID)

	// Create both directories
	for _, dir := range []string{importsDir, resourcesDir} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}
	}

	// Collect all resources for later processing
	var allResources []google.Resource

	for _, service := range services {
		logger.Info("Discovering resources", "service", service)

		serviceImportsDir := filepath.Join(importsDir, service.String())
		serviceResourcesDir := filepath.Join(resourcesDir, service.String())

		// Create service directories
		for _, dir := range []string{serviceImportsDir, serviceResourcesDir} {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create service directory: %w", err)
				}
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
			logger.Warn("Service not yet implemented", "service", service)
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

			// Save resource for later OpenTofu processing
			allResources = append(allResources, *resource)

			count++
			if count%10 == 0 {
				logger.Info("Import progress", "service", service, "resourcesImported", count)
			}
		}

		logger.Info("Import discovery complete", "service", service, "resourcesImported", count)
	}

	// Initialize OpenTofu runner
	runner, err := tfimport.NewOpenTofuRunner(outputPath, logger)
	if err != nil {
		return fmt.Errorf("failed to create OpenTofu runner: %w", err)
	}

	// Initialize OpenTofu
	if err := runner.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize OpenTofu: %w", err)
	}

	// Import resources
	for i, resource := range allResources {
		logger.Info("Importing resource with OpenTofu",
			"resource", resource.ID,
			"progress", fmt.Sprintf("%d/%d", i+1, len(allResources)))

		if err := runner.ImportResource(ctx, resource); err != nil {
			logger.Error("Failed to import resource",
				"resource", resource.ID,
				"error", err)
			continue
		}
	}

	// Generate resource configurations
	if err := runner.GenerateResourceConfig(ctx); err != nil {
		logger.Error("Failed to generate resource configurations", "error", err)
	}

	if err := runner.CleanupImportBlocks(ctx); err != nil {
		logger.Error("Failed to clean up import blocks", "error", err)
	}

	return nil
}
