package sync

import (
	"github.com/priyanshujain/infrasync/internal/auth"
)

// Options contains configuration for sync operations
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
	Auth auth.GoogleAuthOptions
	// Services to sync (e.g. "pubsub", "storage")
	Services []string
	// DryRun if true, will not modify any files
	DryRun bool
}