package es

import (
	"context"
)

type Client interface {
	Create(ctx context.Context, index, id string, document any, refresh bool) error
	Update(ctx context.Context, index, id string, document any, refresh bool) error
	Delete(ctx context.Context, index, id string, refresh bool) error
	UpdateByQuery(ctx context.Context, index string, query *Query, script *Script, refresh bool) error
	DeleteByQuery(ctx context.Context, index string, query *Query, refresh bool) error
	Search(ctx context.Context, index string, req *Request) (*Response, error)
	Exists(ctx context.Context, index string) (bool, error)
	Count(ctx context.Context, index string, query *Query) (int64, error)
	CreateIndex(ctx context.Context, index string, properties map[string]any) error
	DeleteIndex(ctx context.Context, index string) error
	Types() Types
	NewBulkIndexer(index string) (BulkIndexer, error)
}

type Types interface {
	NewLongNumberProperty() any
	NewTextProperty() any
	NewUnsignedLongNumberProperty() any
}

type BulkIndexer interface {
	Add(ctx context.Context, item BulkIndexerItem) error
	Close(ctx context.Context) error
}
