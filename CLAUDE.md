# CLAUDE.md - Development Guidelines

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
- **Logging**: Use structured logging with levels (info, warn, error)