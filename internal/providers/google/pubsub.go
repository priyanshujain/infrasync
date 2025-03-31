package google

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"cloud.google.com/go/pubsub"
	"github.com/priyanshujain/infrasync/internal/providers"
	"google.golang.org/api/iterator"
)

type PubSubResource struct {
	Type       string
	Name       string
	ID         string
	Attributes map[string]interface{}
	Dependents []PubSubResource
}

type pubSub struct {
	client   *pubsub.Client
	provider providers.Provider
}

func NewPubsub(ctx context.Context, provider providers.Provider) (*pubSub, error) {
	client, err := pubsub.NewClient(ctx, provider.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create pubsub client: %w", err)
	}
	return &pubSub{
		client:   client,
		provider: provider,
	}, nil
}

func (ps *pubSub) Close() {
	ps.client.Close()
}

func (ps *pubSub) Import(ctx context.Context) ([]Resource, error) {
	var resources []Resource

	topicIter := ps.client.Topics(ctx)
	for {
		topic, err := topicIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error iterating topics: %w", err)
		}

		topicName := topic.ID()
		topicResource := Resource{
			Provider: ps.provider,
			Type:     ResourceTypePubSubTopic,
			Service:  ServicePubSub,
			Name:     sanitizeName(topicName),
			ID:       fmt.Sprintf("projects/%s/topics/%s", ps.provider.ProjectID, topicName),
			Attributes: map[string]any{
				"name": topicName,
			},
		}

		// Get IAM bindings for the topic
		iamBindings, err := ps.getTopicIAMBindings(ctx, topicName)
		if err != nil {
			return nil, fmt.Errorf("error getting IAM bindings for topic %s: %w", topicName, err)
		}
		if len(iamBindings) > 0 {
			topicResource.Dependents = append(topicResource.Dependents, iamBindings...)
		}

		subscriptions, err := ps.topicSubscriptions(ctx, topicName)
		if err != nil {
			return nil, fmt.Errorf("error getting subscriptions for topic %s: %w", topicName, err)
		}
		if len(subscriptions) > 0 {
			topicResource.Dependents = append(topicResource.Dependents, subscriptions...)
		}

		resources = append(resources, topicResource)
	}

	return resources, nil
}

func (c *pubSub) getTopicIAMBindings(ctx context.Context, topicName string) ([]Resource, error) {
	var resources []Resource

	// Get IAM policy for the topic
	topic := c.client.Topic(topicName)
	policy, err := topic.IAM().Policy(ctx)
	if err != nil {
		slog.Info("topic iam error", "err", err)
		return []Resource{}, fmt.Errorf("error getting IAM policy for topic %s: %w", topicName, err)
	}

	// Create resource for each binding
	for _, role := range policy.Roles() {
		members := policy.Members(role)
		if len(members) > 0 {
			roleSuffix := strings.Replace(string(role), "/", "_", -1)
			roleSuffix = strings.Replace(roleSuffix, ".", "_", -1)

			// Create a binding resource for each role
			iamResource := Resource{
				Provider: c.provider,
				Type:     ResourceTypePubSubTopicIAM,
				Name: fmt.Sprintf("google_pubsub_topic_iam_binding.%s_%s",
					sanitizeName(topicName), sanitizeName(roleSuffix)),
				ID: fmt.Sprintf("projects/%s/topics/%s %s",
					c.provider.ProjectID, topicName, role),
			}
			resources = append(resources, iamResource)
		}
	}

	return resources, nil
}

func (c *pubSub) topicSubscriptions(ctx context.Context, topicName string) ([]Resource, error) {
	var resources []Resource

	topic := c.client.Topic(topicName)
	subIter := topic.Subscriptions(ctx)

	for {
		sub, err := subIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error iterating subscriptions for topic %s: %w", topicName, err)
		}

		subName := sub.ID()
		subResource := Resource{
			Provider: c.provider,
			Type:     ResourceTypePubSubSubscription,
			Service:  ServicePubSub,
			Name:     sanitizeName(subName),
			ID:       fmt.Sprintf("projects/%s/subscriptions/%s", c.provider.ProjectID, subName),
		}

		// Get IAM bindings for the subscription
		iamBindings, err := c.getSubscriptionIAMBindings(ctx, subName)
		if err != nil {
			return nil, fmt.Errorf("error getting IAM bindings for subscription %s: %w", subName, err)
		}
		if len(iamBindings) > 0 {
			subResource.Dependents = append(subResource.Dependents, iamBindings...)
		}

		resources = append(resources, subResource)
	}

	return resources, nil
}

func (ps *pubSub) getSubscriptionIAMBindings(ctx context.Context, subName string) ([]Resource, error) {
	var resources []Resource

	// Get IAM policy for the subscription
	subscription := ps.client.Subscription(subName)
	policy, err := subscription.IAM().Policy(ctx)
	if err != nil {
		slog.Info("subscription iam error", "err", err)
		return resources, fmt.Errorf("error getting IAM policy for subscription %s: %w", subName, err)
	}

	// Create resource for each binding
	for _, role := range policy.Roles() {
		members := policy.Members(role)
		if len(members) > 0 {
			roleSuffix := strings.Replace(string(role), "/", "_", -1)
			roleSuffix = strings.Replace(roleSuffix, ".", "_", -1)

			iamResource := Resource{
				Type: ResourceTypePubSubSubscriptionIAM,
				Name: fmt.Sprintf("google_pubsub_subscription_iam_binding.%s_%s",
					sanitizeName(subName), sanitizeName(roleSuffix)),
				ID: fmt.Sprintf("projects/%s/subscriptions/%s %s",
					ps.provider.ProjectID, subName, role),
			}
			resources = append(resources, iamResource)
		}
	}

	return resources, nil
}

func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "/", "_")
	return name
}
