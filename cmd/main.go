package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/priyanshujain/infrasync/internal/config"
	"github.com/priyanshujain/infrasync/internal/initialize"
	"github.com/priyanshujain/infrasync/internal/providers"
	"github.com/priyanshujain/infrasync/internal/providers/google"
	"github.com/priyanshujain/infrasync/internal/sync"
	"github.com/priyanshujain/infrasync/internal/tfimport"
	"github.com/spf13/cobra"
)

var (
	provider        string
	services        string
	projectID       string
	outputPath      string
	cfgFile         string
	credentialsFile string
	stateBucket     string
	stateKey        string
	dryRun          bool
	repoName        string
	gitInit         bool
	setupGHAction   bool
	useTofu         bool
	cleanupImports  bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "infrasync",
		Short: "InfraSync - Convert existing infrastructure to IaC",
		Long:  `InfraSync is a tool for converting existing cloud infrastructure to Terraform code.`,
	}

	// Import command (existing functionality)
	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import cloud resources and generate Terraform code",
		RunE:  runImport,
	}

	importCmd.Flags().StringVar(&provider, "provider", "google", "Cloud provider (google, aws, azure)")
	importCmd.Flags().StringVar(&services, "services", "pubsub", "Comma-separated list of services to import")
	importCmd.Flags().StringVar(&projectID, "project", "", "Cloud project ID")
	importCmd.Flags().StringVarP(&outputPath, "output", "o", "./terraform", "Path to output directory")
	importCmd.Flags().StringVarP(&cfgFile, "config", "c", "", "Path to configuration file (default is $HOME/.config/infrasync/config.yaml)")
	importCmd.Flags().BoolVar(&useTofu, "use-tofu", true, "Use OpenTofu to generate resource configurations")
	importCmd.Flags().BoolVar(&cleanupImports, "cleanup-imports", true, "Clean up import blocks after successful import")

	// Sync command (new functionality)
	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Terraform with cloud resources",
		Long:  `Sync detects drift between Terraform state and cloud resources and updates configurations accordingly.`,
		RunE:  runSync,
	}

	syncCmd.Flags().StringVar(&provider, "provider", "google", "Cloud provider (currently only 'google' is supported)")
	syncCmd.Flags().StringVar(&services, "services", "pubsub", "Comma-separated list of services to sync")
	syncCmd.Flags().StringVar(&projectID, "project", "", "Cloud project ID")
	syncCmd.Flags().StringVarP(&outputPath, "output", "o", "./terraform", "Path to output directory")
	syncCmd.Flags().StringVar(&credentialsFile, "credentials", "", "Path to credentials file")
	syncCmd.Flags().StringVar(&stateBucket, "state-bucket", "", "GCS bucket for Terraform state")
	syncCmd.Flags().StringVar(&stateKey, "state-key", "terraform.tfstate", "Key/path for Terraform state in bucket")
	syncCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Don't modify any files, just detect drift")

	// Init command (new functionality)
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new IaC repository",
		Long:  `Initialize a new Infrastructure as Code repository with Terraform/OpenTofu configurations.`,
		RunE:  runInit,
	}

	initCmd.Flags().StringVar(&provider, "provider", "google", "Cloud provider (currently only 'google' is supported)")
	initCmd.Flags().StringVar(&projectID, "project", "", "Cloud project ID")
	initCmd.Flags().StringVarP(&outputPath, "output", "o", "./terraform", "Path to output directory")
	initCmd.Flags().StringVar(&repoName, "name", "terraform-infra", "Name of the repository")
	initCmd.Flags().StringVar(&stateBucket, "state-bucket", "", "GCS bucket for Terraform state")
	initCmd.Flags().BoolVar(&gitInit, "git-init", true, "Initialize git repository")
	initCmd.Flags().BoolVar(&setupGHAction, "setup-gh-action", true, "Setup GitHub Actions workflow")

	// Add commands to root
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(initCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runImport(cmd *cobra.Command, args []string) error {
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

func runSync(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate required parameters
	if projectID == "" {
		return fmt.Errorf("project ID is required")
	}

	if provider != "google" {
		return fmt.Errorf("only 'google' provider is currently supported")
	}

	if stateBucket == "" {
		return fmt.Errorf("state-bucket is required")
	}

	// Parse services
	serviceList := strings.Split(services, ",")
	if len(serviceList) == 0 {
		return fmt.Errorf("at least one service must be specified")
	}

	// Create sync options
	options := sync.Options{
		ProjectID:    projectID,
		StateBackend: "gcs",
		StateBucket:  stateBucket,
		StateKey:     stateKey,
		OutputDir:    outputPath,
		Services:     serviceList,
		DryRun:       dryRun,
	}

	// Set up auth options
	if credentialsFile != "" {
		options.Auth.CredentialsFile = credentialsFile
	} else {
		// Use default credentials
		options.Auth.CredentialsEnvVar = "GOOGLE_APPLICATION_CREDENTIALS"
	}

	// Create logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Run sync
	service := sync.NewService(options, logger)
	result, err := service.Run(ctx)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	// Display summary
	fmt.Printf("Sync completed:\n")
	fmt.Printf("- Drift detected: %v\n", result.DriftDetected)
	fmt.Printf("- Resources with drift: %d\n", result.ResourcesDrifted)
	fmt.Printf("- Resources added: %d\n", result.ResourcesAdded)
	fmt.Printf("- Resources removed: %d\n", result.ResourcesRemoved)
	fmt.Printf("- Output directory: %s\n", result.OutputDir)

	if result.DriftDetected && !dryRun {
		fmt.Printf("\nDrift detected! Updated Terraform configurations have been generated.\n")
	} else if result.DriftDetected && dryRun {
		fmt.Printf("\nDrift detected! Run without --dry-run to generate updated configurations.\n")
	}

	return nil
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate required parameters
	if projectID == "" {
		return fmt.Errorf("project ID is required")
	}

	if provider != "google" {
		return fmt.Errorf("only 'google' provider is currently supported")
	}

	// Create absolute path for output
	absOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output: %w", err)
	}

	// Check if output directory already contains files
	if _, err := os.Stat(absOutputPath); err == nil {
		entries, err := os.ReadDir(absOutputPath)
		if err != nil {
			return fmt.Errorf("failed to read output directory: %w", err)
		}
		if len(entries) > 0 {
			return fmt.Errorf("output directory is not empty: %s", absOutputPath)
		}
	}

	// Create options
	options := initialize.Options{
		ProjectID:     projectID,
		StateBackend:  "gcs",
		StateBucket:   stateBucket,
		OutputDir:     absOutputPath,
		RepoName:      repoName,
		GitInit:       gitInit,
		SetupGHAction: setupGHAction,
	}

	// Create logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Run initialization
	service := initialize.NewService(options, logger)
	result, err := service.Run(ctx)
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	// Display summary
	fmt.Printf("Initialization completed:\n")
	fmt.Printf("- Repository created at: %s\n", result.OutputDir)
	if result.RepoInit {
		fmt.Printf("- Git repository initialized\n")
	}
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("1. Review and edit the generated files\n")
	fmt.Printf("2. Run 'infrasync import' to import existing resources\n")
	fmt.Printf("3. Run 'tofu init' and 'tofu apply' to apply the configuration\n")

	return nil
}

func processGoogleServices(ctx context.Context, services []string, projectID, outputPath string, tf tfimport.TerraformGenerator) error {
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

		serviceImportsDir := filepath.Join(importsDir, service)
		serviceResourcesDir := filepath.Join(resourcesDir, service)
		
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
	
	// If useTofu is enabled, run OpenTofu to generate actual resources
	if useTofu {
		logger.Info("Using OpenTofu to generate resource configurations")
		
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
		
		// Clean up import blocks if requested
		if cleanupImports {
			if err := runner.CleanupImportBlocks(ctx); err != nil {
				logger.Error("Failed to clean up import blocks", "error", err)
			}
		}
	}

	return nil
}
