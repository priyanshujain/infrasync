package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/priyanshujain/infrasync/internal/config"
	"github.com/priyanshujain/infrasync/pkg/infrasync"
)

func main() {
	// Load a custom config
	cfg := config.Config{
		// Set your custom configuration here
		// Or load from a file
	}

	// Or load the default config
	var err error
	cfg, err = config.Load()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Create a new client
	client := infrasync.NewClient(cfg)

	// Or use the default client (loads config from default location)
	defaultClient, err := infrasync.DefaultClient()
	if err != nil {
		log.Fatalf("Error creating default client: %v", err)
	}

	// Context for operations
	ctx := context.Background()

	// Initialize a new project
	if err := client.Initialize(ctx); err != nil {
		log.Fatalf("Error initializing project: %v", err)
	}

	// Import all configured services
	if err := client.Import(ctx); err != nil {
		log.Fatalf("Error importing resources: %v", err)
	}

	// Or import specific services
	if err := defaultClient.ImportPubSub(ctx); err != nil {
		log.Fatalf("Error importing PubSub resources: %v", err)
	}

	if err := defaultClient.ImportCloudSQL(ctx); err != nil {
		log.Fatalf("Error importing CloudSQL resources: %v", err)
	}

	if err := defaultClient.ImportStorage(ctx); err != nil {
		log.Fatalf("Error importing Storage resources: %v", err)
	}

	// Import a specific resource
	// Note: Individual resource import is not fully implemented yet
	// Currently this will import all resources of the specified service
	bucketName := "my-bucket"
	if err := defaultClient.ImportSingleResource(ctx, "storage", "google_storage_bucket", bucketName); err != nil {
		log.Fatalf("Error importing specific bucket: %v", err)
	}
	
	// TODO: In the future, this will only import the specified resource

	fmt.Println("All resources imported successfully!")
	os.Exit(0)
}