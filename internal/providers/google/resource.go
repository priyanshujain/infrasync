package google

import "github.com/priyanshujain/infrasync/internal/providers"

type ResourceType string

var (
	ResourceTypePubSubTopic                  ResourceType = "google_pubsub_topic"
	ResourceTypePubSubTopicIAMBinding        ResourceType = "google_pubsub_topic_iam_binding"
	ResourceTypePubSubSubscription           ResourceType = "google_pubsub_subscription"
	ResourceTypePubSubSubscriptionIAMBinding ResourceType = "google_pubsub_subscription_iam_binding"
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
	Dependents []Resource
	Attributes map[string]any
}
