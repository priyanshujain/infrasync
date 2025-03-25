package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/priyanshujain/infrasync/internal/config"
	"github.com/priyanshujain/infrasync/internal/generator"
	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	outputPath string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "infrasync",
		Short: "InfraSync - Convert existing infrastructure to IaC",
		Long:  `InfraSync is a tool for converting existing cloud infrastructure to Terraform code.`,
	}

	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate Terraform templates",
		Long:  `Generate Terraform templates based on the configuration file.`,
		RunE:  runGenerate,
	}

	generateCmd.Flags().StringVarP(&cfgFile, "config", "c", "", "Path to configuration file (default is $HOME/.infrasync/config.yaml)")
	generateCmd.Flags().StringVarP(&outputPath, "output", "o", "./terraform", "Path to output directory")

	rootCmd.AddCommand(generateCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runGenerate(cmd *cobra.Command, args []string) error {
	// If config file not specified, use default
	if cfgFile == "" {
		cfgFile = config.GetDefaultConfigPath()
		if cfgFile == "" {
			return fmt.Errorf("no config file specified and could not determine default location")
		}
	}

	// Ensure config file exists
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", cfgFile)
	}

	// Load configuration
	cfg, err := config.LoadFromFile(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create absolute path for output
	absOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output: %w", err)
	}

	// Create generator
	gen := generator.NewGenerator(cfg, absOutputPath)

	// Generate Terraform files
	if err := gen.Generate(); err != nil {
		return fmt.Errorf("failed to generate terraform files: %w", err)
	}

	fmt.Printf("Successfully generated Terraform templates in %s\n", absOutputPath)
	return nil
}