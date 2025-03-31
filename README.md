# infrasync - convert existing infra to IaC


Step 1
Creating an organized file structure for Terraform configurations with import blocks, focusing on scalability for multiple cloud providers and projects. Key points covered:

## Structure Approach
- Separated imports and resources into different directories:
  ```
  project/
  ├── imports/
  │   └── [provider]/
  │       └── [project]/
  │           └── [service]/
  │               └── resource.tf
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

## Organization Strategy
- Categorized by cloud provider (GCP, AWS, Azure)
- Further divided by project/account
- Then by service type (pubsub, storage, compute, etc.)
- Individual resource files with matching names in both imports and resources directories

## Root Configuration
- Single `main.tf` as orchestration point for SMEs
- Providers configured with appropriate aliases
- Organized variables and outputs by provider and project

## Go Implementation
- Created a Go program to generate this structure, that will be pushed to a github repository
- Supports multiple cloud providers and projects
- cloud provider name and project name will be provided as input config
- It will create import blocks and resource definitions by fetching it from cloud provider APIs
- Follows best practices for Terraform organization

```
infrasync/
├── cmd/
│   └── infrasync/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   └── generator/
│       └── terraform.go
├── providers/
│   ├── gcp/
│       └── pubsub.go
```

## Roadmap
1. Support reading configuration from external sources
2. for now we will only support gcp
3. Generate accurate import blocks by querying existing resources
4. Add more resource templates and service types
