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
  - Storage (Buckets, IAM bindings)
  - More services coming soon!

## Usage

### As a CLI Tool

#### Initialize a new IaC repository

```bash
infrasync init
```

This creates a new repository with:
- Basic Terraform configuration
- GCS backend configuration
- GitHub Actions workflow for drift detection
- Git repository initialization (optional)

#### Import existing resources

```bash
infrasync import
```

This discovers existing resources and generates:
- Import blocks for Terraform
- Directory structure for resources
- Provider configurations

### As a Go Package

InfraSync can also be used as a Go package in your own applications:

```go
import (
	"context"
	"log"

	"github.com/priyanshujain/infrasync/internal/config"
	"github.com/priyanshujain/infrasync/pkg/infrasync"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Create a new client
	client := infrasync.NewClient(cfg)

	// Initialize a project
	ctx := context.Background()
	if err := client.Initialize(ctx); err != nil {
		log.Fatalf("Error initializing project: %v", err)
	}

	// Import all configured resources
	if err := client.Import(ctx); err != nil {
		log.Fatalf("Error importing resources: %v", err)
	}

	// Or import specific services
	if err := client.ImportPubSub(ctx); err != nil {
		log.Fatalf("Error importing PubSub resources: %v", err)
	}

	// Import a specific resource
	// Note: Currently imports all resources of the specified service
	// Future updates will support importing individual resources
	if err := client.ImportSingleResource(ctx, "storage", "google_storage_bucket", "my-bucket"); err != nil {
		log.Fatalf("Error importing specific bucket: %v", err)
	}
}
```

See the `examples/` directory for more detailed usage examples.

## GitHub Actions Integration

InfraSync includes GitHub Actions workflow templates for:
- Automated drift detection
- PR creation when changes are detected
- Seamless integration with existing CI/CD pipelines

## Development

```bash
# Build
make build

# Install
go install

# Test
make test

# Run locally
make run
```

## Roadmap
1. Support for additional GCP services
2. Support for other cloud providers (AWS, Azure)
3. Comprehensive drift detection and reconciliation
4. Enhanced resource templating and customization