package google

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/priyanshujain/infrasync/internal/providers"
	"github.com/priyanshujain/infrasync/internal/providers/google/gcloudclient/cloudsql"
	"google.golang.org/api/option"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

// NOTE: Google Cloud SQL access by using the Google Cloud client library is broken
// It does not provide correct data.
// Google has two types of client libraries
// 1. auto-generated Go libraries
// 2.  Cloud Client Libraries for Go
// They recommend using the Cloud Client Libraries for Go for accessing gcp resources
// but it does not support cloud sql as of now
// So We will use gcloud (google cloud sdk) to access cloud sql resources

type cloudSQL struct {
	service      *sqladmin.Service
	provider     providers.Provider
	gcloudClient *cloudsql.Client
}

func NewCloudSQL(ctx context.Context, provider providers.Provider) (*cloudSQL, error) {
	service, err := sqladmin.NewService(ctx, option.WithScopes(sqladmin.CloudPlatformScope))
	if err != nil {
		return nil, fmt.Errorf("failed to create cloudsql service: %w", err)
	}

	return &cloudSQL{
		service:      service,
		provider:     provider,
		gcloudClient: cloudsql.NewClient(),
	}, nil
}

func (cs *cloudSQL) Close() {
	// No close method for the service
}

type cloudSQLIterator struct {
	ctx           context.Context
	cloudsql      *cloudSQL
	instances     []*sqladmin.DatabaseInstance
	instanceIndex int
	resourceQueue []Resource
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

	// Check if we have processed all instances
	if it.instanceIndex >= len(it.instances) {
		return nil, nil
	}

	// Process next instance
	instance := it.instances[it.instanceIndex]
	it.instanceIndex++

	if err := isImportable(instance); err != nil {
		// Skip this instance and try the next one
		//
		slog.Info("Skipping instance due to terraform pre-check", "instance", instance.Name, "error", err)
		return it.Next(ctx)
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
		users, err := it.cloudsql.getUsers(it.ctx, instance)
		if err != nil {
			it.err = fmt.Errorf("error getting users for instance %s: %w", instanceName, err)
			return nil, it.err
		}
		if len(users) > 0 {
			instanceResource.Dependents = append(instanceResource.Dependents, users...)
		}
	}

	return &instanceResource, nil
}

func (it *cloudSQLIterator) Close() error {
	if it.isClosed {
		return nil
	}
	it.isClosed = true
	return nil
}

func (cs *cloudSQL) Import(ctx context.Context) (ResourceIterator, error) {
	// Fetch all instances upfront
	instances, err := cs.gcloudClient.ListInstances(cs.provider.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("error listing SQL instances: %w", err)
	}

	return &cloudSQLIterator{
		ctx:           ctx,
		cloudsql:      cs,
		instances:     instances,
		instanceIndex: 0,
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

func (cs *cloudSQL) getUsers(ctx context.Context, instance *sqladmin.DatabaseInstance) ([]Resource, error) {
	var resources []Resource

	resp, err := cs.service.Users.List(cs.provider.ProjectID, instance.Name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("error listing users for instance %s: %w", instance.Name, err)
	}

	for _, user := range resp.Items {
		userName := user.Name
		// Skip system users like replication, mysql.infoschema, etc.
		if strings.HasPrefix(userName, "mysql.") || strings.HasPrefix(userName, "cloudsqlsuperuser") {
			continue
		}

		/*
			SQL users for MySQL databases can be imported using the project, instance, host and name, e.g.
				{{project_id}}/{{instance}}/{{host}}/{{name}}

			SQL users for PostgreSQL databases can be imported using the project, instance and name, e.g.
				{{project_id}}/{{instance}}/{{name}}
		*/

		var id string
		if strings.HasPrefix(instance.DatabaseVersion, "POSTGRES") {
			id = fmt.Sprintf("%s/%s/%s", cs.provider.ProjectID, instance.Name, userName)
		} else if strings.HasPrefix(instance.DatabaseVersion, "MYSQL") {
			id = fmt.Sprintf("%s/%s/%s/%s", cs.provider.ProjectID, instance.Name, user.Host, userName)
		} else {
			return nil, fmt.Errorf("unsupported database version %s", instance.DatabaseVersion)
		}

		userResource := Resource{
			Provider: cs.provider,
			Type:     ResourceTypeSQLUser,
			Service:  ServiceCloudSQL,
			Name:     fmt.Sprintf("%s_%s", sanitizeName(instance.Name), sanitizeName(userName)),
			ID:       id,
			Attributes: map[string]any{
				"project":  cs.provider.ProjectID,
				"instance": instance.Name,
				"name":     userName,
				"host":     user.Host,
			},
		}
		resources = append(resources, userResource)
	}

	return resources, nil
}

func isImportable(instance *sqladmin.DatabaseInstance) error {
	if instance.Settings == nil {
		return fmt.Errorf("instance settings are nil instance")
	}

	// NOTE: terraform expected settings.0.maintenance_window.0.day to be in the range (1 - 7), got 0
	if instance.Settings.MaintenanceWindow != nil && instance.Settings.MaintenanceWindow.Day == 0 &&
		instance.Settings.MaintenanceWindow.Hour == 0 {
		return fmt.Errorf("instance maintenance window is invalid instance(Any Window is not supported)")
	}

	// terraform expected settings.0.insights_config.0.query_string_length to be in the range (256 - 4500), got 0
	if instance.Settings.InsightsConfig != nil &&
		instance.Settings.InsightsConfig.QueryStringLength == 0 {
		return fmt.Errorf("instance insights query string length is zero")
	}

	return nil
}

func isRunning(instance *sqladmin.DatabaseInstance) bool {
	return instance.State == "RUNNABLE"
}
