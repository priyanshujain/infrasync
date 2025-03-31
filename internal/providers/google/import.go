package google

import "context"

type ResourceImporter interface {
	Import(context.Context) ([]Resource, error)
	Close()
}
