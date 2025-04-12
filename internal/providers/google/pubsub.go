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

type pubSubIterator struct {
	ctx           context.Context
	pubsub        *pubSub
	topicIter     *pubsub.TopicIterator
	currentTopic  *pubsub.Topic
	resourceQueue []Resource // Queue for dependent resources (IAM bindings, subscriptions)
	err           error
	isClosed      bool
}

func (it *pubSubIterator) Next(ctx context.Context) (*Resource, error) {
	ctx = it.ctx
	if it.isClosed {
		return nil, fmt.Errorf("iterator is closed")
	}

	if len(it.resourceQueue) > 0 {
		resource := it.resourceQueue[0]
		it.resourceQueue = it.resourceQueue[1:]
		return &resource, nil
	}

	if it.err != nil {
		return nil, it.err
	}

	topic, err := it.topicIter.Next()
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		it.err = fmt.Errorf("error iterating topics: %w", err)
		return nil, it.err
	}

	topicName := topic.ID()
	topicResource := Resource{
		Provider: it.pubsub.provider,
		Type:     ResourceTypePubSubTopic,
		Service:  ServicePubSub,
		Name:     sanitizeName(topicName),
		ID:       fmt.Sprintf("projects/%s/topics/%s", it.pubsub.provider.ProjectID, topicName),
	}

	iamBindings, err := it.pubsub.getTopicIAMBindings(it.ctx, topicName)
	if err != nil {
		it.err = fmt.Errorf("error getting IAM bindings for topic %s: %w", topicName, err)
		return nil, it.err
	}
	if len(iamBindings) > 0 {
		topicResource.Dependents = append(topicResource.Dependents, iamBindings...)
	}

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

	topic := c.client.Topic(topicName)
	policy, err := topic.IAM().Policy(ctx)
	if err != nil {
		return []Resource{}, fmt.Errorf("error getting IAM policy for topic %s: %w", topicName, err)
	}

	for _, role := range policy.Roles() {
		members := policy.Members(role)
		if len(members) > 0 {
			roleSuffix := strings.Replace(string(role), "/", "_", -1)
			roleSuffix = strings.Replace(roleSuffix, ".", "_", -1)

			iamResource := Resource{
				Provider: c.provider,
				Type:     ResourceTypePubSubTopicIAMBinding,
				Name: fmt.Sprintf("%s_%s",
					sanitizeName(topicName), sanitizeName(roleSuffix)),
				ID: fmt.Sprintf("projects/%s/topics/%s %s",
					c.provider.ProjectID, topicName, role),
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

	subscription := ps.client.Subscription(subName)
	policy, err := subscription.IAM().Policy(ctx)
	if err != nil {
		return resources, fmt.Errorf("error getting IAM policy for subscription %s: %w", subName, err)
	}

	for _, role := range policy.Roles() {
		members := policy.Members(role)
		if len(members) > 0 {
			roleSuffix := strings.Replace(string(role), "/", "_", -1)
			roleSuffix = strings.Replace(roleSuffix, ".", "_", -1)

			iamResource := Resource{
				Type: ResourceTypePubSubSubscriptionIAMBinding,
				Name: fmt.Sprintf("%s_%s",
					sanitizeName(subName), sanitizeName(roleSuffix)),
				ID: fmt.Sprintf("projects/%s/subscriptions/%s %s",
					ps.provider.ProjectID, subName, role),
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
