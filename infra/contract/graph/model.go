package graph

// Properties represents a key-value map for attributes of nodes and edges.
type Properties map[string]any

// NodeSelector defines a way to select a node by its labels and properties.
// It is used when creating an edge without knowing the node's elementId.
// Note: This is primarily intended for use in CreateEdge to simplify a common pattern
// of matching two nodes by their properties and then creating a relationship between them.
// For more complex queries or other operations (like update/delete), it's recommended to use
// the Query object with UpdateNodesByQuery/DeleteNodesByQuery methods.
type NodeSelector struct {
	Labels     []string   `json:"labels"`
	Properties Properties `json:"properties"`
}

// Node represents a node (or vertex) in the graph.
type Node struct {
	ID         string     `json:"id"`
	Labels     []string   `json:"labels"`
	Properties Properties `json:"properties"`
}

// Path represents a sequence of nodes and edges, typically as the result of a pathfinding algorithm.
type Path struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"edges"`
}

// Edge represents an edge (or relationship) between two nodes.
type Edge struct {
	ID string `json:"id"`
	// Label represents the type of the relationship (e.g., "KNOWS", "FRIEND_OF").
	// In Neo4j, this corresponds to the Relationship Type.
	Label        string     `json:"label"`
	SourceNodeID string     `json:"source_node_id"`
	TargetNodeID string     `json:"target_node_id"`
	Properties   Properties `json:"properties"`

	// New fields for matching nodes by properties.
	// If the selector is provided, it will be used for matching.
	// Otherwise, the corresponding NodeID (Source/Target) will be used.
	SourceNodeSelector *NodeSelector `json:"source_node_selector,omitempty"`
	TargetNodeSelector *NodeSelector `json:"target_node_selector,omitempty"`
}
