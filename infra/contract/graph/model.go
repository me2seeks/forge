package graph

// Properties represents a key-value map for attributes of nodes and edges.
type Properties map[string]any

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
}
