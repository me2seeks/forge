package graph

import (
	"context"
)

// Client defines the interface for a graph database.
type Client interface {
	GraphAlgorithms

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

// GraphAlgorithms defines a set of common graph algorithms.
type GraphAlgorithms interface {
	// ShortestPath calculates the shortest path between two nodes.
	// config can be used to pass algorithm-specific parameters, e.g., {"relationshipWeightProperty": "cost"}.
	ShortestPath(ctx context.Context, sourceNodeID, targetNodeID string, config map[string]any) ([]*Path, error)

	// PageRank calculates the PageRank for nodes in the graph.
	// config can be used to pass algorithm-specific parameters, e.g., {"dampingFactor": 0.85, "maxIterations": 20}.
	PageRank(ctx context.Context, config map[string]any) (map[string]float64, error)

	// ConnectedComponents finds sets of connected nodes in the graph.
	// It returns a map of nodeID to its componentID.
	// config can be used to pass algorithm-specific parameters, e.g., {"relationshipTypes": ["FRIENDS_WITH"]}.
	ConnectedComponents(ctx context.Context, config map[string]any) (map[string]string, error)

	// BetweennessCentrality calculates the betweenness centrality for nodes in the graph.
	// config can be used to pass algorithm-specific parameters, e.g., {"relationshipTypes": ["HAS_CONNECTION"]}.
	BetweennessCentrality(ctx context.Context, config map[string]any) (map[string]float64, error)
}

// BulkWriter provides an interface for efficient bulk data ingestion.
type BulkWriter interface {
	AddNode(ctx context.Context, node *Node) error
	AddEdge(ctx context.Context, edge *Edge) error
	// Close finalizes the bulk operation and reports any errors.
	Close(ctx context.Context) error
}
