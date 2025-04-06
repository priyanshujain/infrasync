package tfimport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/priyanshujain/infrasync/internal/providers/google"
)

// OpenTofuRunner handles OpenTofu operations
type OpenTofuRunner struct {
	workingDir string
	logger     *slog.Logger
}

// NewOpenTofuRunner creates a new OpenTofu runner
func NewOpenTofuRunner(workingDir string, logger *slog.Logger) (*OpenTofuRunner, error) {
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}

	// Ensure OpenTofu is installed
	if err := checkTofuInstalled(); err != nil {
		return nil, fmt.Errorf("OpenTofu not installed: %w", err)
	}

	return &OpenTofuRunner{
		workingDir: workingDir,
		logger:     logger,
	}, nil
}

// checkTofuInstalled checks if OpenTofu is installed
func checkTofuInstalled() error {
	cmd := exec.Command("tofu", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("OpenTofu is not installed or not in PATH: %w", err)
	}
	return nil
}

// ImportResource imports a resource into OpenTofu state
func (r *OpenTofuRunner) ImportResource(ctx context.Context, resource google.Resource) error {
	r.logger.Info("Importing resource",
		"type", resource.Type,
		"name", resource.Name,
		"id", resource.ID)

	// Build the import command
	cmd := exec.CommandContext(ctx, "tofu", "import",
		fmt.Sprintf("%s.%s", resource.Type, resource.Name),
		resource.ID)
	
	cmd.Dir = r.workingDir
	
	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	// Run the command
	err := cmd.Run()
	if err != nil {
		r.logger.Error("Import failed",
			"error", err,
			"stderr", stderr.String())
		return fmt.Errorf("failed to import resource: %w, stderr: %s", err, stderr.String())
	}
	
	r.logger.Info("Import succeeded", 
		"resource", resource.ID, 
		"output", stdout.String())
	
	return nil
}

// GenerateResourceConfig generates resource configuration from state
func (r *OpenTofuRunner) GenerateResourceConfig(ctx context.Context) error {
	r.logger.Info("Generating resource configuration from state")
	
	// Export state to JSON
	cmd := exec.CommandContext(ctx, "tofu", "show", "-json")
	cmd.Dir = r.workingDir
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	if err := cmd.Run(); err != nil {
		r.logger.Error("Failed to show state",
			"error", err,
			"stderr", stderr.String())
		return fmt.Errorf("failed to show state: %w", err)
	}
	
	// Parse the state
	var state map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &state); err != nil {
		return fmt.Errorf("failed to parse state JSON: %w", err)
	}
	
	// Extract resources from state
	resources, ok := state["values"].(map[string]any)["root_module"].(map[string]any)["resources"].([]any)
	if !ok {
		return fmt.Errorf("failed to extract resources from state")
	}
	
	// Generate configuration for each resource
	for _, res := range resources {
		resource := res.(map[string]any)
		if err := r.generateResourceFile(resource); err != nil {
			r.logger.Error("Failed to generate resource file",
				"resource", resource["address"],
				"error", err)
		}
	}
	
	return nil
}

// generateResourceFile generates a .tf file for a resource
func (r *OpenTofuRunner) generateResourceFile(resource map[string]any) error {
	address := resource["address"].(string)
	parts := strings.Split(address, ".")
	resourceType := parts[0]
	resourceName := strings.Join(parts[1:], ".")
	
	// Create directory if needed
	resourceDir := filepath.Join(r.workingDir, "resources")
	if err := os.MkdirAll(resourceDir, 0755); err != nil {
		return fmt.Errorf("failed to create resource directory: %w", err)
	}
	
	// Create resource file
	filePath := filepath.Join(resourceDir, fmt.Sprintf("%s.tf", resourceName))
	
	// Build resource configuration
	values := resource["values"].(map[string]any)
	
	// Convert values to HCL
	var config strings.Builder
	config.WriteString(fmt.Sprintf("resource \"%s\" \"%s\" {\n", resourceType, resourceName))
	
	// Write attributes
	for key, value := range values {
		// Skip computed values
		if key == "id" || strings.HasPrefix(key, "%") {
			continue
		}
		
		valueStr, err := formatHCLValue(value)
		if err != nil {
			r.logger.Warn("Failed to format value",
				"key", key,
				"value", value,
				"error", err)
			continue
		}
		
		config.WriteString(fmt.Sprintf("  %s = %s\n", key, valueStr))
	}
	
	config.WriteString("}\n")
	
	// Write the file
	if err := os.WriteFile(filePath, []byte(config.String()), 0644); err != nil {
		return fmt.Errorf("failed to write resource file: %w", err)
	}
	
	r.logger.Info("Generated resource file",
		"resource", address,
		"file", filePath)
	
	return nil
}

// formatHCLValue formats a value for HCL
func formatHCLValue(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("\"%s\"", v), nil
	case bool, int, float64:
		return fmt.Sprintf("%v", v), nil
	case []any:
		if len(v) == 0 {
			return "[]", nil
		}
		var elements []string
		for _, elem := range v {
			elemStr, err := formatHCLValue(elem)
			if err != nil {
				return "", err
			}
			elements = append(elements, elemStr)
		}
		return fmt.Sprintf("[%s]", strings.Join(elements, ", ")), nil
	case map[string]any:
		var pairs []string
		for key, val := range v {
			valStr, err := formatHCLValue(val)
			if err != nil {
				return "", err
			}
			pairs = append(pairs, fmt.Sprintf("%s = %s", key, valStr))
		}
		return fmt.Sprintf("{\n    %s\n  }", strings.Join(pairs, "\n    ")), nil
	case nil:
		return "null", nil
	default:
		return "", fmt.Errorf("unsupported type: %T", v)
	}
}

// CleanupImportBlocks removes import blocks after successful import
func (r *OpenTofuRunner) CleanupImportBlocks(ctx context.Context) error {
	r.logger.Info("Cleaning up import blocks")
	
	// Find import block files
	importDir := filepath.Join(r.workingDir, "imports")
	if _, err := os.Stat(importDir); os.IsNotExist(err) {
		r.logger.Info("No import directory found, nothing to clean up")
		return nil
	}
	
	// Remove import directory
	if err := os.RemoveAll(importDir); err != nil {
		return fmt.Errorf("failed to remove import directory: %w", err)
	}
	
	r.logger.Info("Import blocks cleaned up")
	return nil
}

// Initialize initializes a new Terraform directory
func (r *OpenTofuRunner) Initialize(ctx context.Context) error {
	r.logger.Info("Initializing OpenTofu")
	
	cmd := exec.CommandContext(ctx, "tofu", "init")
	cmd.Dir = r.workingDir
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	if err := cmd.Run(); err != nil {
		r.logger.Error("Initialization failed",
			"error", err,
			"stderr", stderr.String())
		return fmt.Errorf("failed to initialize: %w", err)
	}
	
	r.logger.Info("Initialization succeeded", 
		"output", stdout.String())
	
	return nil
}