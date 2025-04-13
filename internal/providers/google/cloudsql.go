package google

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/priyanshujain/infrasync/internal/providers"
	"google.golang.org/api/option"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

type cloudSQL struct {
	service  *sqladmin.Service
	provider providers.Provider
}

func NewCloudSQL(ctx context.Context, provider providers.Provider) (*cloudSQL, error) {
	service, err := sqladmin.NewService(ctx, option.WithScopes(sqladmin.CloudPlatformScope))
	if err != nil {
		return nil, fmt.Errorf("failed to create cloudsql service: %w", err)
	}

	return &cloudSQL{
		service:  service,
		provider: provider,
	}, nil
}

func (cs *cloudSQL) Close() {
	// No close method for the service
}

type cloudSQLIterator struct {
	ctx           context.Context
	cloudsql      *cloudSQL
	resourceQueue []Resource
	pageToken     string
	finished      bool
	err           error
	isClosed      bool
}

func (it *cloudSQLIterator) Next(ctx context.Context) (*Resource, error) {
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

	// Check if we already finished listing instances
	if it.finished {
		return nil, nil
	}

	// Fetch next batch of instances
	call := it.cloudsql.service.Instances.List(it.cloudsql.provider.ProjectID)
	if it.pageToken != "" {
		call = call.PageToken(it.pageToken)
	}

	resp, err := call.Context(it.ctx).Do()
	if err != nil {
		it.err = fmt.Errorf("error listing SQL instances: %w", err)
		return nil, it.err
	}

	// Update pagination state
	it.pageToken = resp.NextPageToken
	if it.pageToken == "" {
		it.finished = true
	}

	// Process instances
	for _, instance := range resp.Items {
		if !isImportable(instance) {
			continue
		}

		instanceName := instance.Name
		id := fmt.Sprintf("projects/%s/instances/%s", it.cloudsql.provider.ProjectID, instanceName)
		instanceResource := Resource{
			Provider: it.cloudsql.provider,
			Type:     ResourceTypeSQLInstance,
			Service:  ServiceCloudSQL,
			Name:     sanitizeName(instanceName),
			ID:       id,
			Attributes: map[string]any{
				"project":          it.cloudsql.provider.ProjectID,
				"name":             instanceName,
				"database_version": instance.DatabaseVersion,
				"region":           instance.Region,
			},
		}

		if isRunning(instance) {
			// Get databases for this instance
			databases, err := it.cloudsql.getDatabases(it.ctx, instanceName)
			if err != nil {
				it.err = fmt.Errorf("error getting databases for instance %s: %w", instanceName, err)
				return nil, it.err
			}
			if len(databases) > 0 {
				instanceResource.Dependents = append(instanceResource.Dependents, databases...)
			}

			// Get users for this instance
			users, err := it.cloudsql.getUsers(it.ctx, instanceName)
			if err != nil {
				it.err = fmt.Errorf("error getting users for instance %s: %w", instanceName, err)
				return nil, it.err
			}
			if len(users) > 0 {
				instanceResource.Dependents = append(instanceResource.Dependents, users...)
			}
		}

		// Add to the queue
		it.resourceQueue = append(it.resourceQueue, instanceResource)
	}

	// If queue is still empty, we have no more instances
	if len(it.resourceQueue) == 0 {
		return nil, nil
	}

	// Return the first resource from the queue
	resource := it.resourceQueue[0]
	it.resourceQueue = it.resourceQueue[1:]
	return &resource, nil
}

func (it *cloudSQLIterator) Close() error {
	if it.isClosed {
		return nil
	}
	it.isClosed = true
	return nil
}

func (cs *cloudSQL) Import(ctx context.Context) (ResourceIterator, error) {
	return &cloudSQLIterator{
		ctx:           ctx,
		cloudsql:      cs,
		resourceQueue: make([]Resource, 0),
	}, nil
}

func (cs *cloudSQL) getDatabases(ctx context.Context, instanceName string) ([]Resource, error) {
	var resources []Resource

	resp, err := cs.service.Databases.List(cs.provider.ProjectID, instanceName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("error listing databases for instance %s: %w", instanceName, err)
	}

	for _, database := range resp.Items {
		dbName := database.Name

		id := fmt.Sprintf("projects/%s/instances/%s/databases/%s", cs.provider.ProjectID, instanceName, dbName)

		dbResource := Resource{
			Provider: cs.provider,
			Type:     ResourceTypeSQLDatabase,
			Service:  ServiceCloudSQL,
			Name:     fmt.Sprintf("%s_%s", sanitizeName(instanceName), sanitizeName(dbName)),
			ID:       id,
			Attributes: map[string]any{
				"project":   cs.provider.ProjectID,
				"instance":  instanceName,
				"name":      dbName,
				"charset":   database.Charset,
				"collation": database.Collation,
			},
		}
		resources = append(resources, dbResource)
	}

	return resources, nil
}

func (cs *cloudSQL) getUsers(ctx context.Context, instanceName string) ([]Resource, error) {
	var resources []Resource

	resp, err := cs.service.Users.List(cs.provider.ProjectID, instanceName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("error listing users for instance %s: %w", instanceName, err)
	}

	for _, user := range resp.Items {
		userName := user.Name
		// Skip system users like replication, mysql.infoschema, etc.
		if strings.HasPrefix(userName, "mysql.") || strings.HasPrefix(userName, "cloudsqlsuperuser") {
			continue
		}

		userResource := Resource{
			Provider: cs.provider,
			Type:     ResourceTypeSQLUser,
			Service:  ServiceCloudSQL,
			Name:     fmt.Sprintf("%s_%s", sanitizeName(instanceName), sanitizeName(userName)),
			ID:       fmt.Sprintf("%s:%s:%s", cs.provider.ProjectID, instanceName, userName),
			Attributes: map[string]any{
				"project":  cs.provider.ProjectID,
				"instance": instanceName,
				"name":     userName,
				"host":     user.Host,
			},
		}
		resources = append(resources, userResource)
	}

	return resources, nil
}

func isImportable(instance *sqladmin.DatabaseInstance) bool {
	if instance.Settings == nil {
		slog.Error("instance settings are nil", "instance", instance.Name)
		return false
	}

	if instance.Settings.MaintenanceWindow == nil {
		slog.Error("instance maintenance window is nil", "instance", instance.Name)
		return false
	}

	// NOTE: terraform expected settings.0.maintenance_window.0.day to be in the range (1 - 7), got 0
	if instance.Settings.MaintenanceWindow.Day == 0 && instance.Settings.MaintenanceWindow.Hour == 0 &&
		instance.Settings.MaintenanceWindow.UpdateTrack != "stable" {
		slog.Error("instance maintenance window is invalid", "instance", instance.Name)
		return false
	}

	// terraform expected settings.0.insights_config.0.query_string_length to be in the range (256 - 4500), got 0
	if instance.Settings.InsightsConfig == nil {
		slog.Error("instance insights config is nil", "instance", instance.Name)
		return false
	}

	if instance.Settings.InsightsConfig.QueryStringLength == 0 {
		slog.Error("instance insights config is invalid", "instance", instance.Name)
		return false
	}

	slog.Info("instance is importable", "instance", instance.Name, "insights_config", instance.Settings.InsightsConfig.QueryStringLength)

	return true
}

func isRunning(instance *sqladmin.DatabaseInstance) bool {
	slog.Info("instance is running", "instance", instance.Name, "state", instance.State, "disk_size", instance.CurrentDiskSize, "max_disk_size", instance.MaxDiskSize)
	return instance.State == "RUNNABLE" && instance.CurrentDiskSize > 0 && instance.MaxDiskSize > 0
}
