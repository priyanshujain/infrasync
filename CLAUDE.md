# CLAUDE.md - Development Guidelines

## Project Overview
Infrasync is a Go-based CLI tool that converts existing cloud infrastructure to Terraform code (Infrastructure as Code). The tool discovers cloud resources and generates appropriate Terraform import blocks and resource definitions.

## Current Implementation
- GCP PubSub implementation using iterator pattern
- Hierarchical directory structure (provider/project/service)
- XDG-compliant configuration management
- Structured logging using slog

## Build/Test Commands
- Build: `go build ./cmd/infrasync`
- Run: `go run ./cmd/infrasync/main.go`
- Test all: `go test ./...`
- Test specific: `go test ./[package-path]`
- Test with verbose: `go test -v ./...`
- Lint: `golangci-lint run`
- Format: `gofmt -s -w .`

## Code Style
- **Formatting**: Use `gofmt` or `go fmt` for consistent styling
- **Imports**: Group standard library, external, and internal imports
- **Naming**: Use CamelCase for exported names; otherwise use camelCase
- **Error Handling**: Always check errors; use meaningful error messages
- **Documentation**: Document all exported functions, types, and packages
- **Tests**: Write table-driven tests when possible
- **Types**: Prefer explicit types; avoid interface{} when possible
- **Organization**: Follow Go project standard layout (cmd/, internal/, pkg/)
- **Constants**: Group related constants; use UPPERCASE for unchanging values
- **Logging**: Use structured logging with slog (no fmt.Printf)

## Design Patterns
- Use iterator pattern for efficient resource processing
- Implement ResourceIterator interface with Next() and Close() methods
- Separate provider logic from terraform generation
- Properly handle resource cleanup with defer and Close()

## Pending Tasks
- Implementation of additional GCP services beyond PubSub
- Support for other cloud providers (AWS, Azure)
- Conflict resolution for existing resources
- Testing framework and test cases
- Resource validation and error handling improvements