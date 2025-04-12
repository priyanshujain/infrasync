package google

import "context"

type ResourceIterator interface {
	Next(context.Context) (*Resource, error)

	Close() error
}

type ResourceImporter interface {
	Import(context.Context) (ResourceIterator, error)
	Close()
}
