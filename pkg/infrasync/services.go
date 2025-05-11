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

// ImportSingleResource imports a single resource with the given type and ID
func (c *Client) ImportSingleResource(ctx context.Context, service google.Service, resourceType string, resourceID string) error {
	// This is a simplified implementation - a more robust one would actually import the single resource
	// For now, we'll just call the service-specific import function
	return c.ImportService(ctx, service)
}