package graph

import "context"

// Neo4jExtensions defines a set of advanced graph algorithms and functionalities specific to Neo4j,
// often leveraging the Graph Data Science (GDS) library.
type Neo4jExtensions interface {
	// CommunityDetection runs a community detection algorithm.
	// 'algorithm' can be "louvain", "labelPropagation", etc.
	// 'config' holds algorithm-specific parameters.
	CommunityDetection(ctx context.Context, algorithm string, config map[string]any) (map[string]string, error)

	// Similarity calculates similarity scores between nodes.
	// 'algorithm' can be "jaccard", "cosine", "pearson", etc.
	// 'config' holds algorithm-specific parameters.
	Similarity(ctx context.Context, algorithm string, config map[string]any) (map[string]float64, error)

	// Centrality runs a centrality algorithm.
	// 'algorithm' can be "degree", "closeness", etc.
	// 'config' holds algorithm-specific parameters.
	Centrality(ctx context.Context, algorithm string, config map[string]any) (map[string]float64, error)

	// LinkPrediction predicts the existence of a relationship between two nodes.
	// 'algorithm' can be "adamicAdar", "commonNeighbors", etc.
	// 'config' holds algorithm-specific parameters.
	LinkPrediction(ctx context.Context, algorithm string, config map[string]any) ([]*LinkPredictionResult, error)

	// NodeEmbedding creates a vector representation of nodes.
	// 'algorithm' can be "node2vec", "fastRP", "graphSage", etc.
	// 'config' holds algorithm-specific parameters.
	NodeEmbedding(ctx context.Context, algorithm string, config map[string]any) (map[string][]float64, error)
}

// LinkPredictionResult represents the result of a link prediction algorithm.
type LinkPredictionResult struct {
	SourceNodeID string  `json:"source_node_id"`
	TargetNodeID string  `json:"target_node_id"`
	Score        float64 `json:"score"`
}
