package providers

type ProviderType string

var (
	ProviderTypeGoogle ProviderType = "google"
)

type BackendType string

var (
	BackendTypeGCS BackendType = "gcs"
)

func (p ProviderType) String() string {
	return string(p)
}

type Provider struct {
	Type      ProviderType
	ProjectID string
	Region    string
}

type Backend struct {
	Type   BackendType
	Bucket string
}
