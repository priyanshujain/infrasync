package google

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/pubsub"
	"github.com/priyanshujain/infrasync/internal/providers"
	"google.golang.org/api/iterator"
)

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

// pubSubIterator implements the ResourceIterator interface
type pubSubIterator struct {
	ctx           context.Context
	pubsub        *pubSub
	topicIter     *pubsub.TopicIterator
	currentTopic  *pubsub.Topic
	resourceQueue []Resource // Queue for dependent resources (IAM bindings, subscriptions)
	err           error
	isClosed      bool
}

// Next returns the next resource or nil when done
func (it *pubSubIterator) Next(ctx context.Context) (*Resource, error) {
	// Use the iterator's stored context instead of the passed one
	// to ensure consistent state throughout iteration
	ctx = it.ctx
	if it.isClosed {
		return nil, fmt.Errorf("iterator is closed")
	}

	// If we have resources in the queue, return the next one
	if len(it.resourceQueue) > 0 {
		resource := it.resourceQueue[0]
		it.resourceQueue = it.resourceQueue[1:]
		return &resource, nil
	}

	// If we encountered an error previously, return it
	if it.err != nil {
		return nil, it.err
	}

	// Try to get the next topic
	topic, err := it.topicIter.Next()
	if err == iterator.Done {
		// No more topics, we're done
		return nil, nil
	}
	if err != nil {
		it.err = fmt.Errorf("error iterating topics: %w", err)
		return nil, it.err
	}

	// Process the topic
	topicName := topic.ID()
	topicResource := Resource{
		Provider: it.pubsub.provider,
		Type:     ResourceTypePubSubTopic,
		Service:  ServicePubSub,
		Name:     sanitizeName(topicName),
		ID:       fmt.Sprintf("projects/%s/topics/%s", it.pubsub.provider.ProjectID, topicName),
	}

	// Get IAM bindings
	iamBindings, err := it.pubsub.getTopicIAMBindings(it.ctx, topicName)
	if err != nil {
		it.err = fmt.Errorf("error getting IAM bindings for topic %s: %w", topicName, err)
		return nil, it.err
	}
	if len(iamBindings) > 0 {
		topicResource.Dependents = append(topicResource.Dependents, iamBindings...)
	}

	// Get subscriptions
	subscriptions, err := it.pubsub.topicSubscriptions(it.ctx, topicName)
	if err != nil {
		it.err = fmt.Errorf("error getting subscriptions for topic %s: %w", topicName, err)
		return nil, it.err
	}
	if len(subscriptions) > 0 {
		topicResource.Dependents = append(topicResource.Dependents, subscriptions...)
	}

	return &topicResource, nil
}

func (it *pubSubIterator) Close() error {
	if it.isClosed {
		return nil
	}
	it.isClosed = true
	return nil
}

func (ps *pubSub) Import(ctx context.Context) (ResourceIterator, error) {
	topicIter := ps.client.Topics(ctx)

	return &pubSubIterator{
		ctx:           ctx,
		pubsub:        ps,
		topicIter:     topicIter,
		resourceQueue: make([]Resource, 0),
	}, nil
}

func (c *pubSub) getTopicIAMBindings(ctx context.Context, topicName string) ([]Resource, error) {
	var resources []Resource

	// Get IAM policy for the topic
	topic := c.client.Topic(topicName)
	policy, err := topic.IAM().Policy(ctx)
	if err != nil {
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
				// Store attributes for drift detection
				Attributes: map[string]any{
					"topic":   topicName,
					"role":    role,
					"members": members,
				},
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
				// Store attributes for drift detection
				Attributes: map[string]any{
					"subscription": subName,
					"role":         role,
					"members":      members,
				},
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