package tfimport

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/priyanshujain/infrasync/internal/providers/google"
)

// THINK: Should this generator be via docker?

type generator struct {
	workingDir string
}

var ErrAlreadyExists = fmt.Errorf("resource_already_exists")

func New(workingDir string) (*generator, error) {
	if err := checkIfRunnerInstalled(); err != nil {
		return nil, fmt.Errorf("generator not installed: %w", err)
	}

	return &generator{
		workingDir: workingDir,
	}, nil
}

func checkIfRunnerInstalled() error {
	cmd := exec.Command("terraform", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("terraform is not installed or not in PATH: %w", err)
	}
	return nil
}

func (r *generator) Import(ctx context.Context, resource google.Resource) error {
	slog.Info("Importing resource",
		"type", resource.Type,
		"name", resource.Name,
		"id", resource.ID)

	resourceDir := filepath.Join(r.workingDir, "resources", resource.Provider.Type.String(), resource.Provider.ProjectID, resource.Service.String())
	resourceFilePath := filepath.Join(resourceDir, fmt.Sprintf("%s.tf", resource.Name))

	if _, err := os.Stat(resourceFilePath); err == nil {
		return ErrAlreadyExists
	}

	if _, err := os.Stat(resourceDir); os.IsNotExist(err) {
		if err := os.MkdirAll(resourceDir, 0755); err != nil {
			return fmt.Errorf("failed to create resource directory: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, "terraform", "plan",
		fmt.Sprintf("-generate-config-out=%s", resourceFilePath))
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		slog.Error("Import failed",
			"stderr", stderr.String())
		return fmt.Errorf("failed to import resource: %w", err)
	}

	slog.Info("Import succeeded",
		"resource", resource.ID)

	return nil
}

func (r *generator) CleanupImportBlocks(resource google.Resource) error {
	importBlockPath := filepath.Join(r.workingDir, fmt.Sprintf("%s.tf", resource.Name))
	if err := os.Remove(importBlockPath); err != nil {
		return fmt.Errorf("failed to remove import block file: %w", err)
	}
	return nil
}

func (r *generator) Initialize(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "terraform", "init")
	cmd.Dir = r.workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}
	return nil
}
