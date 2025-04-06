package drift

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/priyanshujain/infrasync/internal/providers/google"
)

// ResourceState represents a resource's state in Terraform
type ResourceState struct {
	Type       string         `json:"type"`
	Name       string         `json:"name"`
	Provider   string         `json:"provider"`
	ID         string         `json:"id"`
	Attributes map[string]any `json:"attributes"`
}

// DriftResult represents the result of drift detection
type DriftResult struct {
	ResourceType string
	ResourceName string
	ResourceID   string
	HasDrift     bool
	Changes      map[string]Change
}

// Change represents a change in a resource attribute
type Change struct {
	OldValue any
	NewValue any
}

// Detector detects drift between cloud resources and Terraform state
type Detector struct {
	logger *slog.Logger
}

// NewDetector creates a new drift detector
func NewDetector(logger *slog.Logger) *Detector {
	return &Detector{
		logger: logger,
	}
}

// DetectDrift detects drift between a cloud resource and its Terraform state
func (d *Detector) DetectDrift(ctx context.Context, resource *google.Resource, state *ResourceState) (*DriftResult, error) {
	if resource == nil || state == nil {
		return nil, fmt.Errorf("resource or state is nil")
	}

	// Create drift result
	result := &DriftResult{
		ResourceType: string(resource.Type),
		ResourceName: resource.Name,
		ResourceID:   resource.ID,
		HasDrift:     false,
		Changes:      make(map[string]Change),
	}

	// Compare resource attributes with state
	// For now, just compare IAM bindings for PubSub topics
	if resource.Type == google.ResourceTypePubSubTopicIAM {
		// Extract IAM bindings from resource and state
		// This is simplified and would need to be expanded for actual implementation
		resourceBindings, ok := resource.Attributes["members"].([]string)
		if !ok {
			return nil, fmt.Errorf("invalid resource bindings format")
		}

		stateBindings, ok := state.Attributes["members"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid state bindings format")
		}

		// Convert state bindings to []string
		stateBindingsStr := make([]string, len(stateBindings))
		for i, binding := range stateBindings {
			stateBindingsStr[i] = binding.(string)
		}

		// Compare bindings
		if !reflect.DeepEqual(resourceBindings, stateBindingsStr) {
			result.HasDrift = true
			result.Changes["members"] = Change{
				OldValue: stateBindingsStr,
				NewValue: resourceBindings,
			}
		}
	}

	return result, nil
}

// DetectResourceDrift detects drift for a specific resource type
func (d *Detector) DetectResourceDrift(ctx context.Context, resources []*google.Resource, stateData []byte) ([]*DriftResult, error) {
	var results []*DriftResult

	// Parse state data
	var state map[string]any
	if err := json.Unmarshal(stateData, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state data: %w", err)
	}

	// Extract resources from state
	resourcesState, ok := state["resources"].([]any)
	if !ok {
		return nil, fmt.Errorf("invalid state format: resources not found")
	}

	// Create map of state resources by ID
	stateResourcesByID := make(map[string]*ResourceState)
	for _, res := range resourcesState {
		resMap, ok := res.(map[string]any)
		if !ok {
			continue
		}

		id, ok := resMap["id"].(string)
		if !ok {
			continue
		}

		stateResource := &ResourceState{
			Type:       resMap["type"].(string),
			Name:       resMap["name"].(string),
			Provider:   resMap["provider"].(string),
			ID:         id,
			Attributes: resMap["attributes"].(map[string]any),
		}

		stateResourcesByID[id] = stateResource
	}

	// Compare cloud resources with state
	for _, resource := range resources {
		stateResource, ok := stateResourcesByID[resource.ID]
		if !ok {
			// Resource exists in cloud but not in state
			results = append(results, &DriftResult{
				ResourceType: string(resource.Type),
				ResourceName: resource.Name,
				ResourceID:   resource.ID,
				HasDrift:     true,
				Changes:      map[string]Change{"existence": {OldValue: false, NewValue: true}},
			})
			continue
		}

		// Compare resource with state
		result, err := d.DetectDrift(ctx, resource, stateResource)
		if err != nil {
			d.logger.Warn("Failed to detect drift", "resource", resource.ID, "error", err)
			continue
		}

		if result.HasDrift {
			results = append(results, result)
		}
	}

	return results, nil
}