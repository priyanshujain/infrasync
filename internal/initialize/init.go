package initialize

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/priyanshujain/infrasync/internal/config"
	"github.com/priyanshujain/infrasync/internal/providers"
)

func Init(cfg config.Config) error {
	slog.Info("Initializing new IaC repository", "outputDir", cfg.Path)

	path := cfg.ProjectPath()

	if err := createDirectoryStructure(path); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}

	if err := createTerraformDefaultFiles(cfg); err != nil {
		return fmt.Errorf("failed to create Terraform files: %w", err)
	}

	if err := initGitRepo(path); err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	if err := setupGitHubActions(path); err != nil {
		return fmt.Errorf("failed to setup GitHub Actions: %w", err)
	}

	return nil
}

func createDirectoryStructure(path string) error {
	dirs := []string{
		path,
		filepath.Join(path, "modules"),
		filepath.Join(path, ".github", "workflows"),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); err == nil {
			continue
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

func createTerraformDefaultFiles(cfg config.Config) error {
	provider := cfg.DefaultProvider()

	// Create provider.tf
	providerTmpl := `# Generated by InfraSync
terraform {
  {{if eq .StateBackend "gcs"}}
  backend "gcs" {
    bucket = "{{.StateBucket}}"
    prefix = "terraform/state"
  }
  {{end}}

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 4.0"
    }
  }
}

provider "google" {
  project = "{{.ProjectID}}"
}
`

	// Create variables.tf
	variablesTmpl := `# Generated by InfraSync
variable "project_id" {
  description = "The Google Cloud project ID"
  type        = string
  default     = "{{.ProjectID}}"
}

variable "region" {
  description = "The default region for resources"
  type        = string
  default     = "{{.Region}}"
}
`

	// Create main.tf
	mainTmpl := `# Generated by InfraSync
# Main Terraform configuration
`

	// Create .gitignore
	gitignoreTmpl := `# Generated by InfraSync
.terraform/
.terraform.lock.hcl
terraform.tfstate
terraform.tfstate.backup
*.tfvars
`

	path := cfg.ProjectPath()
	backend := cfg.DefaultBackend()

	// Define template data
	data := struct {
		ProjectID    string
		Region       string
		StateBackend providers.BackendType
		StateBucket  string
	}{
		ProjectID:    provider.ProjectID,
		Region:       provider.Region,
		StateBackend: backend.Type,
		StateBucket:  backend.Bucket,
	}

	// Create provider.tf
	if err := createFileFromTemplate(filepath.Join(path, "provider.tf"), providerTmpl, data); err != nil {
		return err
	}

	// Create variables.tf
	if err := createFileFromTemplate(filepath.Join(path, "variables.tf"), variablesTmpl, data); err != nil {
		return err
	}

	// Create main.tf
	if err := createFileFromTemplate(filepath.Join(path, "main.tf"), mainTmpl, data); err != nil {
		return err
	}

	// Create .gitignore
	if err := createFileFromTemplate(filepath.Join(path, ".gitignore"), gitignoreTmpl, data); err != nil {
		return err
	}

	// Create README.md
	readmeTmpl := `# {{.RepoName}}

Infrastructure as Code repository managed with [InfraSync](https://github.com/priyanshujain/infrasync).

## Structure

- environments/: Environment-specific configurations
- modules/: Reusable Terraform modules
- main.tf: Main Terraform configuration

## Usage

To import existing resources:

    infrasync import --project={{.ProjectID}} --services=pubsub

To detect drift and update configurations:

    infrasync sync --project={{.ProjectID}} --state-bucket={{.StateBucket}}
`

	readmeData := struct {
		RepoName    string
		ProjectID   string
		StateBucket string
	}{
		RepoName:    cfg.Name,
		ProjectID:   cfg.DefaultProvider().ProjectID,
		StateBucket: cfg.DefaultBackend().Bucket,
	}

	if err := createFileFromTemplate(filepath.Join(path, "README.md"), readmeTmpl, readmeData); err != nil {
		return err
	}

	return nil
}

func createFileFromTemplate(filePath, tmplStr string, data any) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer file.Close()

	// Parse template
	tmpl, err := template.New(filepath.Base(filePath)).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func initGitRepo(path string) error {
	if err := os.Chdir(path); err != nil {
		return fmt.Errorf("failed to change to output directory: %w", err)
	}

	if err := runCommandHelper("git", "init"); err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	if err := runCommandHelper("git", "add", "."); err != nil {
		return fmt.Errorf("failed to add files: %w", err)
	}

	if err := runCommandHelper("git", "commit", "-m", "Initial commit by InfraSync"); err != nil {
		return fmt.Errorf("failed to commit files: %w", err)
	}

	return nil
}

func setupGitHubActions(path string) error {
	workflowTmpl := `# Generated by InfraSync
name: InfraSync - Infrastructure Drift Detection

on:
  schedule:
    - cron: "0 0 * * *"  # Run daily at midnight
  workflow_dispatch:     # Allow manual triggering

jobs:
  sync-infrastructure:
    runs-on: ubuntu-latest

    permissions:
      contents: write
      pull-requests: write

    steps:
      - name: Checkout code
        uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.20'

      - name: Install InfraSync
        run: |
          go install github.com/priyanshujain/infrasync@latest

      - name: Auth to Google Cloud
        id: auth
        uses: google-github-actions/auth@v1
        with:
          credentials_json: ${{ "{{" }} secrets.GCP_SA_KEY {{ "}}" }}
          export_environment_variables: true

      - name: Setup OpenTofu
        uses: opentofu/setup-opentofu@v1

      - name: Run InfraSync Sync
        run: |
          infrasync sync \
            --project=${{ "{{" }} secrets.GCP_PROJECT {{ "}}" }} \
            --services=pubsub \
            --state-bucket=${{ "{{" }} secrets.GCS_STATE_BUCKET {{ "}}" }} \
            --state-key=terraform/state \
            --output=.

      - name: Create PR if drift detected
        if: ${{ "{{" }} env.DRIFT_DETECTED == 'true' {{ "}}" }}
        uses: peter-evans/create-pull-request@v5
        with:
          title: "Infrastructure drift detected"
          body: |
            This PR was automatically created by the InfraSync drift detection workflow.

            ## Detected Changes

            Infrastructure drift was detected between Terraform state and actual cloud resources.
            The Terraform configuration has been updated to reflect the current state of your infrastructure.

            ## Review Instructions

            Please review the changes carefully before merging to ensure they match your intended infrastructure state.

            Generated with InfraSync
          branch: "infrasync-drift-${{ "{{" }} github.run_id {{ "}}" }}"
          commit-message: "Update Terraform configurations to match cloud state"
          base: main
`

	return createFileFromTemplate(
		filepath.Join(path, ".github", "workflows", "infrasync.yml"),
		workflowTmpl,
		nil,
	)
}

func runCommandHelper(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
