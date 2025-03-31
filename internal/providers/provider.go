package providers

type ProviderType string

var (
	ProviderTypeGoogle ProviderType = "google"
)

func (p ProviderType) String() string {
	return string(p)
}

type Provider struct {
	Type      ProviderType
	ProjectID string
}
