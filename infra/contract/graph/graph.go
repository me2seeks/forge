package graph

import (
	"context"
)

// Client defines the interface for a graph database.
type Client interface {
	// --- Node Operations ---
	CreateNode(ctx context.Context, node *Node) (*Node, error)
	GetNode(ctx context.Context, nodeID string) (*Node, error)
	UpdateNode(ctx context.Context, nodeID string, properties Properties) error
	DeleteNode(ctx context.Context, nodeID string) error

	// --- Edge Operations ---
	CreateEdge(ctx context.Context, edge *Edge) (*Edge, error)
	GetEdge(ctx context.Context, edgeID string) (*Edge, error)
	UpdateEdge(ctx context.Context, edgeID string, properties Properties) error
	DeleteEdge(ctx context.Context, edgeID string) error

	// --- Bulk Operations ---
	NewBulkWriter() BulkWriter

	// --- Query Operations ---
	Query(ctx context.Context, query *Query) (*QueryResult, error)
	// FindNodes is a convenience method to find and return nodes directly.
	// It is a wrapper around the generic Query method.
	FindNodes(ctx context.Context, query *Query) ([]*Node, error)
	// FindEdges is a convenience method to find and return edges directly.
	// It is a wrapper around the generic Query method.
	FindEdges(ctx context.Context, query *Query) ([]*Edge, error)
	// Count executes a query and returns the number of results.
	Count(ctx context.Context, query *Query) (int64, error)

	// --- Schema Operations ---
	CreateNodeIndex(ctx context.Context, label string, properties []string) error
	CreateEdgeIndex(ctx context.Context, label string, properties []string) error
	CreateConstraint(ctx context.Context, label, property string, constraintType ConstraintType) error
	DropNodeIndex(ctx context.Context, label string, properties []string) error
	DropEdgeIndex(ctx context.Context, label string, properties []string) error
	DropConstraint(ctx context.Context, label, property string, constraintType ConstraintType) error

	// --- Bulk Update/Delete Operations (based on Query) ---
	// UpdateNodesByQuery updates properties of all nodes matching the query.
	UpdateNodesByQuery(ctx context.Context, query *Query, properties Properties) (int, error)
	// UpdateEdgesByQuery updates properties of all edges matching the query.
	UpdateEdgesByQuery(ctx context.Context, query *Query, properties Properties) (int, error)
	// DeleteNodesByQuery deletes all nodes matching the query.
	DeleteNodesByQuery(ctx context.Context, query *Query) (int, error)
	// DeleteEdgesByQuery deletes all edges matching the query.
	DeleteEdgesByQuery(ctx context.Context, query *Query) (int, error)
}

// BulkWriter provides an interface for efficient bulk data ingestion.
type BulkWriter interface {
	AddNode(ctx context.Context, node *Node) error
	AddEdge(ctx context.Context, edge *Edge) error
	// Close finalizes the bulk operation and reports any errors.
	Close(ctx context.Context) error
}
