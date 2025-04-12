package main

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
	"github.com/spf13/cobra"
)

var cfg config.Config

func main() {
	rootCmd := &cobra.Command{
		Use:   "infrasync",
		Short: "InfraSync - Convert existing infrastructure to IaC",
		Long:  `InfraSync is a tool for converting existing cloud infrastructure to Terraform code.`,
	}

	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import cloud resources and generate Terraform code",
		RunE:  runImport,
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new IaC repository",
		Long:  `Initialize a new Infrastructure as Code repository with Terraform configurations.`,
		RunE:  runInit,
	}

	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(initCmd)

	var err error
	cfg, err = config.Load()
	if err != nil {
		fmt.Printf("Error loading config file: %v\n", err)

		fmt.Println("Please format the config file as per the template.")
		fmt.Println("Template:")
		fmt.Print(config.Template)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runImport(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	var absOutputPath = cfg.ProjectPath()
	var provider = cfg.DefaultProvider()

	provider = cfg.DefaultProvider()

	resourcesDir := filepath.Join(absOutputPath, "resources", provider.Type.String(), provider.ProjectID)

	for _, dir := range []string{resourcesDir} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}
	}

	services := cfg.GoogleServices(provider)

	for _, service := range services {
		serviceResourcesDir := filepath.Join(resourcesDir, service.String())

		for _, dir := range []string{serviceResourcesDir} {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create service directory: %w", err)
				}
			}
		}

		if err := importService(ctx, service); err != nil {
			return fmt.Errorf("failed to process Google services: %w", err)
		}
	}

	return nil
}

func setupDirectoryStructure(outputPath string) error {
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
	slog.Info("3. Run 'terraform init' and 'terraform apply' to apply the configuration")

	return nil
}

func importService(ctx context.Context, service google.Service) error {
	path := cfg.ProjectPath()
	provider := cfg.DefaultProvider()

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
