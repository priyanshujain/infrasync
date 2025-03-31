package state

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	"github.com/priyanshujain/infrasync/internal/auth"
	"google.golang.org/api/option"
)

// GCSStateBackend manages Terraform state in Google Cloud Storage
type GCSStateBackend struct {
	client     *storage.Client
	bucketName string
	projectID  string
}

// NewGCSStateBackend creates a new GCS state backend
func NewGCSStateBackend(ctx context.Context, opts auth.GoogleAuthOptions, bucketName, projectID string) (*GCSStateBackend, error) {
	var credsJSON []byte
	var err error

	// Get credentials
	if len(opts.CredentialsJSON) > 0 {
		credsJSON = opts.CredentialsJSON
	} else if opts.CredentialsFile != "" {
		credsJSON, err = ioutil.ReadFile(opts.CredentialsFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read credentials file: %w", err)
		}
	} else if opts.CredentialsEnvVar != "" {
		envPath := os.Getenv(opts.CredentialsEnvVar)
		if envPath != "" {
			credsJSON, err = ioutil.ReadFile(envPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read credentials from env var path: %w", err)
			}
		}
	} else {
		return nil, fmt.Errorf("no credentials provided")
	}

	// Create client
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(credsJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}

	return &GCSStateBackend{
		client:     client,
		bucketName: bucketName,
		projectID:  projectID,
	}, nil
}

// GetState retrieves the Terraform state file from GCS
func (b *GCSStateBackend) GetState(ctx context.Context, statePath string) ([]byte, error) {
	// Get state from GCS
	bucket := b.client.Bucket(b.bucketName)
	obj := bucket.Object(statePath)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read state from GCS: %w", err)
	}
	defer reader.Close()

	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read state data: %w", err)
	}

	return data, nil
}

// ParseState parses the Terraform state file
func (b *GCSStateBackend) ParseState(stateData []byte) (map[string]interface{}, error) {
	var state map[string]interface{}
	if err := json.Unmarshal(stateData, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state data: %w", err)
	}
	return state, nil
}

// SaveState saves updated Terraform state to a local file
func (b *GCSStateBackend) SaveState(stateData []byte, outputPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write state file
	if err := ioutil.WriteFile(outputPath, stateData, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// Close closes the GCS client
func (b *GCSStateBackend) Close() error {
	return b.client.Close()
}