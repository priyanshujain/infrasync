# InfraSync

InfraSync is a tool for converting existing cloud infrastructure to Terraform code and maintaining synchronization between Infrastructure-as-Code and actual cloud resources.

## Features

- Initialize new IaC repositories
- Import existing GCP resources
- Generate Terraform import blocks and resource definitions
- Detect drift between cloud state and Terraform state
- Create PRs with changes when drift is detected

## Structure Approach
- Separated imports and resources into different directories:
  ```
  project/
  ├── resources/
  │   └── [provider]/
  │       └── [project]/
  │           └── [service]/
  │               └── resource.tf
  ├── main.tf
  ├── variables.tf
  ├── providers.tf
  └── outputs.tf
  ```

## Supported Providers

- Google Cloud Platform (GCP)
  - PubSub (Topics, Subscriptions, IAM bindings)
  - CloudSQL (Instances, Databases, Users)
  - More services coming soon!

## Usage

### Initialize a new IaC repository

```bash
infrasync init
```

This creates a new repository with:
- Basic Terraform configuration
- GCS backend configuration
- GitHub Actions workflow for drift detection
- Git repository initialization (optional)

### Import existing resources

```bash
infrasync import
```

This discovers existing resources and generates:
- Import blocks for Terraform
- Directory structure for resources
- Provider configurations

## GitHub Actions Integration

InfraSync includes GitHub Actions workflow templates for:
- Automated drift detection
- PR creation when changes are detected
- Seamless integration with existing CI/CD pipelines

## Development

```bash
# Build
go build ./cmd/infrasync

# Test
go test ./...

# Run locally
go run ./cmd/infrasync/main.go
```

## Roadmap
1. Support for additional GCP services
2. Support for other cloud providers (AWS, Azure)
3. Comprehensive drift detection and reconciliation
4. Enhanced resource templating and customization
