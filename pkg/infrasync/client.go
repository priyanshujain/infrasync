package infrasync

import (
	"context"
	"fmt"

	"github.com/priyanshujain/infrasync/internal/auth"
	"github.com/priyanshujain/infrasync/internal/sync"
)

// Options contains configuration for the InfraSync client
type Options struct {
	// ProjectID is the Google Cloud project ID
	ProjectID string
	// StateBackend specifies the state backend to use (e.g. "gcs")
	StateBackend string
	// StateBucket is the bucket name for the state backend
	StateBucket string
	// StateKey is the key/path for the state file
	StateKey string
	// OutputDir is the directory to write generated Terraform files
	OutputDir string
	// Auth options for Google Cloud
	CredentialsJSON []byte
	// Alternative to CredentialsJSON - path to credentials file
	CredentialsFile string
	// Services to sync (e.g. "pubsub", "storage")
	Services []string
	// DryRun if true, will not modify any files
	DryRun bool
}

// Client is the main InfraSync client
type Client struct {
	options Options
}

// NewClient creates a new InfraSync client
func NewClient(options Options) (*Client, error) {
	// Validate options
	if options.ProjectID == "" {
		return nil, fmt.Errorf("project ID is required")
	}
	
	if options.StateBackend == "" {
		return nil, fmt.Errorf("state backend is required")
	}
	
	if options.StateBackend == "gcs" && options.StateBucket == "" {
		return nil, fmt.Errorf("state bucket is required for GCS backend")
	}
	
	if len(options.Services) == 0 {
		return nil, fmt.Errorf("at least one service is required")
	}
	
	return &Client{options: options}, nil
}

// Sync syncs the infrastructure with Terraform
func (c *Client) Sync(ctx context.Context) (*sync.SyncResult, error) {
	// Create internal sync options
	syncOptions := sync.Options{
		ProjectID:    c.options.ProjectID,
		StateBackend: c.options.StateBackend,
		StateBucket:  c.options.StateBucket,
		StateKey:     c.options.StateKey,
		OutputDir:    c.options.OutputDir,
		Services:     c.options.Services,
		DryRun:       c.options.DryRun,
		Auth: auth.GoogleAuthOptions{
			CredentialsJSON: c.options.CredentialsJSON,
			CredentialsFile: c.options.CredentialsFile,
		},
	}
	
	// Run sync
	return sync.Run(ctx, syncOptions)
}