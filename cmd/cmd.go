package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/priyanshujain/infrasync/internal/config"
	"github.com/priyanshujain/infrasync/pkg/infrasync"
	"github.com/spf13/cobra"
)

var cfg config.Config

func Execute() {
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
	client := infrasync.NewClient(cfg)
	
	if err := client.Import(ctx); err != nil {
		return fmt.Errorf("import failed: %w", err)
	}
	
	return nil
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	client := infrasync.NewClient(cfg)
	
	if err := client.Initialize(ctx); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}
	
	slog.Info("Next steps:")
	slog.Info("1. Review and edit the generated files")
	slog.Info("2. Run 'infrasync import' to import existing resources")
	slog.Info("3. Run 'terraform init' and 'terraform apply' to apply the configuration")
	
	return nil
}