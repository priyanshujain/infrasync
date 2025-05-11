package infrasync

import (
	"context"

	"github.com/priyanshujain/infrasync/internal/providers/google"
)

// ImportPubSub imports all PubSub resources for the configured project
func (c *Client) ImportPubSub(ctx context.Context) error {
	return c.ImportService(ctx, "pubsub")
}

// ImportCloudSQL imports all CloudSQL resources for the configured project
func (c *Client) ImportCloudSQL(ctx context.Context) error {
	return c.ImportService(ctx, "cloudsql")
}

// ImportStorage imports all Storage resources for the configured project
func (c *Client) ImportStorage(ctx context.Context) error {
	return c.ImportService(ctx, "storage")
}

// ImportSingleResource imports a single resource with the given type and ID.
// TODO: Currently this is a placeholder that ignores resourceType and resourceID parameters
// and imports all resources of the specified service. Future implementation will:
// 1. Create a filtered resource iterator that returns only the specified resource
// 2. Use the terraform importer to import only that specific resource
// 3. Support proper error handling for non-existent resources
func (c *Client) ImportSingleResource(ctx context.Context, service google.Service, resourceType string, resourceID string) error {
	// IMPORTANT: This implementation currently ignores resourceType and resourceID
	// It will be properly implemented in a future update
	return c.ImportService(ctx, service)
}