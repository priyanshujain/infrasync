package google

import "github.com/priyanshujain/infrasync/internal/providers"

type ResourceType string

var (
	ResourceTypePubSubSchema          ResourceType = "pubsub_schema"
	ResourceTypePubSubTopic           ResourceType = "pubsub_topic"
	ResourceTypePubSubTopicIAM        ResourceType = "pubsub_topic_iam"
	ResourceTypePubSubSubscription    ResourceType = "pubsub_subscription"
	ResourceTypePubSubSubscriptionIAM ResourceType = "pubsub_subscription_iam"
)

type Service string

var (
	ServicePubSub Service = "pubsub"
)

func (s Service) String() string {
	return string(s)
}

type Resource struct {
	Provider   providers.Provider
	Type       ResourceType
	Service    Service
	Name       string
	ID         string
	Attributes map[string]any
	Dependents []Resource
}
