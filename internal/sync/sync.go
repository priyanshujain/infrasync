package sync

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/priyanshujain/infrasync/internal/drift"
	"github.com/priyanshujain/infrasync/internal/providers"
	"github.com/priyanshujain/infrasync/internal/providers/google"
	"github.com/priyanshujain/infrasync/internal/state"
	"github.com/priyanshujain/infrasync/internal/tfimport"
)

// SyncResult contains the results of a sync operation
type SyncResult struct {
	DriftDetected    bool
	ResourcesDrifted int
	ResourcesAdded   int
	ResourcesRemoved int
	OutputDir        string
}

// Service is the main sync service
type Service struct {
	options Options
	logger  *slog.Logger
}

// NewService creates a new sync service
func NewService(options Options, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}

	return &Service{
		options: options,
		logger:  logger,
	}
}

// Run executes the sync process
func (s *Service) Run(ctx context.Context) (*SyncResult, error) {
	s.logger.Info("Starting sync process",
		"project", s.options.ProjectID,
		"stateBackend", s.options.StateBackend,
		"services", s.options.Services)

	// Initialize result
	result := &SyncResult{
		DriftDetected:    false,
		ResourcesDrifted: 0,
		ResourcesAdded:   0,
		ResourcesRemoved: 0,
		OutputDir:        s.options.OutputDir,
	}

	// Initialize state backend
	var stateBackend *state.GCSStateBackend
	var err error

	if s.options.StateBackend == "gcs" {
		stateBackend, err = state.NewGCSStateBackend(
			ctx,
			s.options.Auth,
			s.options.StateBucket,
			s.options.ProjectID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize state backend: %w", err)
		}
		defer stateBackend.Close()
	} else {
		return nil, fmt.Errorf("unsupported state backend: %s", s.options.StateBackend)
	}

	// Get state from backend
	stateData, err := stateBackend.GetState(ctx, s.options.StateKey)
	if err != nil {
		s.logger.Warn("Failed to get state from backend", "error", err)
		// Continue with empty state
		stateData = []byte("{}")
	}

	// Initialize provider
	provider := providers.Provider{
		Type:      providers.ProviderTypeGoogle,
		ProjectID: s.options.ProjectID,
	}

	// Initialize drift detector
	detector := drift.NewDetector(s.logger)

	// Process each service
	for _, serviceName := range s.options.Services {
		switch serviceName {
		case "pubsub":
			err = s.processPubSub(ctx, provider, stateData, detector, result)
			if err != nil {
				s.logger.Error("Failed to process PubSub service", "error", err)
				continue
			}
		default:
			s.logger.Warn("Unsupported service", "service", serviceName)
		}
	}

	// If drift detected, mark environment variable for GitHub Actions
	if result.DriftDetected && !s.options.DryRun {
		os.Setenv("DRIFT_DETECTED", "true")
	}

	s.logger.Info("Sync completed",
		"driftDetected", result.DriftDetected,
		"resourcesDrifted", result.ResourcesDrifted,
		"resourcesAdded", result.ResourcesAdded,
		"resourcesRemoved", result.ResourcesRemoved)

	return result, nil
}

// processPubSub handles PubSub service sync
func (s *Service) processPubSub(
	ctx context.Context,
	provider providers.Provider,
	stateData []byte,
	detector *drift.Detector,
	result *SyncResult,
) error {
	// Initialize PubSub importer
	pubsub, err := google.NewPubsub(ctx, provider)
	if err != nil {
		return fmt.Errorf("failed to initialize PubSub importer: %w", err)
	}
	defer pubsub.Close()

	// Get resource iterator
	iter, err := pubsub.Import(ctx)
	if err != nil {
		return fmt.Errorf("failed to import PubSub resources: %w", err)
	}
	defer iter.Close()

	// Collect all resources
	var resources []google.Resource
	for {
		resource, err := iter.Next(ctx)
		if err != nil {
			return fmt.Errorf("error iterating resources: %w", err)
		}
		if resource == nil {
			break
		}

		// Store the value, not the pointer
		resources = append(resources, *resource)
	}

	// For drift detection, we need pointers, so create pointer slice
	var resourcePointers []*google.Resource
	for i := range resources {
		resourcePointers = append(resourcePointers, &resources[i])
	}

	// Detect drift
	driftResults, err := detector.DetectResourceDrift(ctx, resourcePointers, stateData)
	if err != nil {
		return fmt.Errorf("failed to detect drift: %w", err)
	}

	if len(driftResults) > 0 {
		result.DriftDetected = true
		result.ResourcesDrifted = len(driftResults)

		// Log drift details
		for _, dr := range driftResults {
			s.logger.Info("Drift detected",
				"resourceType", dr.ResourceType,
				"resourceName", dr.ResourceName,
				"resourceID", dr.ResourceID,
				"changes", len(dr.Changes))
		}
	}

	// Generate Terraform configurations if not dry run
	if !s.options.DryRun {
		outputDir := s.options.OutputDir
		if outputDir == "" {
			outputDir = filepath.Join("terraform", provider.ProjectID)
		}

		// Initialize Terraform generator
		generator, err := tfimport.New(outputDir, []string{provider.ProjectID})
		if err != nil {
			s.logger.Error("Failed to create Terraform generator", "error", err)
			return err
		}

		// Generate Terraform files
		for _, resource := range resources {
			// Pass resources directly (they're already values)
			err = generator.SaveImportBlock(resource)
			if err != nil {
				s.logger.Error("Failed to generate import",
					"resource", resource.ID,
					"error", err)
				continue
			}

			// For drift detection, we would need to add resource block generation
			// This is a placeholder for future implementation
			for _, dr := range driftResults {
				if dr.ResourceID == resource.ID {
					// TODO: Implement resource block generation
					s.logger.Info("Drift detected, would generate resource",
						"resource", resource.ID)
					break
				}
			}
		}
	}

	return nil
}

// Run is a convenience function to create and run a sync service
func Run(ctx context.Context, options Options) (*SyncResult, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	service := NewService(options, logger)
	return service.Run(ctx)
}
