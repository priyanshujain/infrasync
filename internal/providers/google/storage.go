package google

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/priyanshujain/infrasync/internal/providers"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type gcsStorage struct {
	client   *storage.Client
	provider providers.Provider
}

func NewStorage(ctx context.Context, provider providers.Provider) (*gcsStorage, error) {
	client, err := storage.NewClient(ctx, option.WithScopes(storage.ScopeReadOnly))
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}
	return &gcsStorage{
		client:   client,
		provider: provider,
	}, nil
}

func (gs *gcsStorage) Close() {
	gs.client.Close()
}

type storageIterator struct {
	ctx           context.Context
	storage       *gcsStorage
	bucketIter    *storage.BucketIterator
	resourceQueue []Resource
	err           error
	isClosed      bool
}

func (it *storageIterator) Next(ctx context.Context) (*Resource, error) {
	it.ctx = ctx

	if it.isClosed {
		return nil, fmt.Errorf("iterator is closed")
	}

	if it.err != nil {
		return nil, it.err
	}

	// Return resources from the queue if available
	if len(it.resourceQueue) > 0 {
		resource := it.resourceQueue[0]
		it.resourceQueue = it.resourceQueue[1:]
		return &resource, nil
	}

	// Get the next bucket
	attrs, err := it.bucketIter.Next()
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		it.err = fmt.Errorf("error iterating buckets: %w", err)
		return nil, it.err
	}

	bucketName := attrs.Name

	// Create bucket resource
	bucketResource := Resource{
		Provider: it.storage.provider,
		Type:     ResourceTypeStorageBucket,
		Service:  ServiceStorage,
		Name:     sanitizeName(bucketName),
		ID:       bucketName, // Import ID for GCS bucket is just the bucket name
		Attributes: map[string]any{
			"name":          bucketName,
			"project":       it.storage.provider.ProjectID,
			"location":      attrs.Location,
			"storage_class": attrs.StorageClass,
		},
	}

	// Get IAM bindings for this bucket
	iamBindings, err := it.storage.getBucketIAMBindings(it.ctx, bucketName)
	if err != nil {
		// Log error but continue with the bucket
		slog.Info("Error getting IAM bindings", "bucket", bucketName, "error", err)
	} else if len(iamBindings) > 0 {
		bucketResource.Dependents = append(bucketResource.Dependents, iamBindings...)
	}

	return &bucketResource, nil
}

func (it *storageIterator) Close() error {
	if it.isClosed {
		return nil
	}
	it.isClosed = true
	return nil
}

func (gs *gcsStorage) Import(ctx context.Context) (ResourceIterator, error) {
	// Create a bucket iterator
	bucketIter := gs.client.Buckets(ctx, gs.provider.ProjectID)

	return &storageIterator{
		ctx:           ctx,
		storage:       gs,
		bucketIter:    bucketIter,
		resourceQueue: make([]Resource, 0),
	}, nil
}

func (gs *gcsStorage) getBucketIAMBindings(ctx context.Context, bucketName string) ([]Resource, error) {
	var resources []Resource

	bucket := gs.client.Bucket(bucketName)
	policy, err := bucket.IAM().Policy(ctx)
	if err != nil {
		return resources, fmt.Errorf("error getting IAM policy for bucket %s: %w", bucketName, err)
	}

	for _, role := range policy.Roles() {
		members := policy.Members(role)
		if len(members) > 0 {
			roleSuffix := strings.Replace(string(role), "/", "_", -1)
			roleSuffix = strings.Replace(roleSuffix, ".", "_", -1)

			iamResource := Resource{
				Provider: gs.provider,
				Type:     ResourceTypeStorageBucketIAMBinding,
				Service:  ServiceStorage,
				Name:     fmt.Sprintf("%s_%s", sanitizeName(bucketName), sanitizeName(roleSuffix)),
				ID:       fmt.Sprintf("%s %s", bucketName, role),
				Attributes: map[string]any{
					"bucket":  bucketName,
					"role":    role,
					"members": members,
				},
			}
			resources = append(resources, iamResource)
		}
	}

	return resources, nil
}