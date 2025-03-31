package google

import "context"

// ResourceIterator provides a streaming interface for imported resources
type ResourceIterator interface {
	// Next returns the next resource or an error
	// When iteration is complete, Next will return (nil, nil)
	Next(context.Context) (*Resource, error)
	
	// Close releases any resources used by the iterator
	Close() error
}

type ResourceImporter interface {
	// Import returns a resource iterator that streams resources as they're discovered
	Import(context.Context) (ResourceIterator, error)
	Close()
}
