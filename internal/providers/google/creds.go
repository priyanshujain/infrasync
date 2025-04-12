package google

import (
	"context"
	"fmt"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/storage/v1"
)

func ValidateCredentials() error {
	const ReadOnly = "https://www.googleapis.com/auth/cloud-platform.read-only"

	_, err := google.FindDefaultCredentials(context.Background(), ReadOnly)
	if err != nil {
		return err
	}

	return nil
}

func ValidateBackend(bucketName string) error {
	if bucketName == "" {
		return fmt.Errorf("bucket name is empty")
	}

	ctx := context.Background()
	service, err := storage.NewService(ctx)
	if err != nil {
		return err
	}

	bucket, err := service.Buckets.Get(bucketName).Do()
	if err != nil {
		return fmt.Errorf("failed to get bucket %s: %w", bucketName, err)
	}

	if bucket == nil {
		return fmt.Errorf("bucket %s does not exist", bucketName)
	}

	if bucket.Name != bucketName {
		return fmt.Errorf("bucket name mismatch: expected %s, got %s", bucketName, bucket.Name)
	}

	return nil
}
