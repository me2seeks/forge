package neo4j

import (
	"context"
	"os"
	"testing"

	"github.com/me2seeks/forge/infra/contract/graph"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/require"
)

var (
	testURI      = os.Getenv("NEO4J_URI")
	testUsername = os.Getenv("NEO4J_USERNAME")
	testPassword = os.Getenv("NEO4J_PASSWORD")
)

func setup(t *testing.T) (graph.Client, func()) {
	ctx := context.Background()
	client, err := New(ctx, testURI, WithBasicAuth(testUsername, testPassword, ""))
	require.NoError(t, err)

	teardown := func() {
		// Clean up the database after each test
		session := client.(*neo4jClient).driver.NewSession(ctx, neo4j.SessionConfig{})
		defer session.Close(ctx)
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx, "MATCH (n) DETACH DELETE n", nil)
			return nil, err
		})
		require.NoError(t, err)
	}

	return client, teardown
}

func TestNodeCRUD(t *testing.T) {
	client, teardown := setup(t)
	defer teardown()

	ctx := context.Background()

	// 1. Create Node
	nodeToCreate := &graph.Node{
		Labels: []string{"TestUser"},
		Properties: graph.Properties{
			"name":  "Alice",
			"email": "alice@example.com",
		},
	}
	createdNode, err := client.CreateNode(ctx, nodeToCreate)
	require.NoError(t, err)
	require.NotNil(t, createdNode)
	require.NotEmpty(t, createdNode.ID)
	require.Equal(t, "Alice", createdNode.Properties["name"])

	t.Logf("createdNode: %v", createdNode)

	// 2. Get Node
	retrievedNode, err := client.GetNode(ctx, createdNode.ID)
	require.NoError(t, err)
	require.NotNil(t, retrievedNode)
	require.Equal(t, createdNode.ID, retrievedNode.ID)
	require.Equal(t, "alice@example.com", retrievedNode.Properties["email"])

	t.Logf("retrievedNode: %v", retrievedNode)

	// 3. Update Node
	err = client.UpdateNode(ctx, createdNode.ID, graph.Properties{"name": "Alice Smith"})
	require.NoError(t, err)

	updatedNode, err := client.GetNode(ctx, createdNode.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedNode)
	require.Equal(t, "Alice Smith", updatedNode.Properties["name"])
	require.Equal(t, "alice@example.com", updatedNode.Properties["email"]) // Email should be unchanged

	t.Logf("updatedNode: %v", updatedNode)

	// // 4. Delete Node
	err = client.DeleteNode(ctx, createdNode.ID)
	require.NoError(t, err)

	deletedNode, err := client.GetNode(ctx, createdNode.ID)
	require.NoError(t, err)
	require.Nil(t, deletedNode)
}

func TestEdgeCRUD(t *testing.T) {
	client, teardown := setup(t)
	defer teardown()

	ctx := context.Background()

	// 1. Create source and target nodes
	sourceNode, err := client.CreateNode(ctx, &graph.Node{Labels: []string{"Source"}})
	require.NoError(t, err)
	targetNode, err := client.CreateNode(ctx, &graph.Node{Labels: []string{"Target"}})
	require.NoError(t, err)

	// 2. Create Edge
	edgeToCreate := &graph.Edge{
		Label:        "CONNECTS_TO",
		SourceNodeID: sourceNode.ID,
		TargetNodeID: targetNode.ID,
		Properties:   graph.Properties{"weight": 1.0},
	}
	createdEdge, err := client.CreateEdge(ctx, edgeToCreate)
	require.NoError(t, err)
	require.NotNil(t, createdEdge)
	require.NotEmpty(t, createdEdge.ID)
	require.Equal(t, "CONNECTS_TO", createdEdge.Label)
	require.Equal(t, 1.0, createdEdge.Properties["weight"])

	// 3. Get Edge
	retrievedEdge, err := client.GetEdge(ctx, createdEdge.ID)
	require.NoError(t, err)
	require.NotNil(t, retrievedEdge)
	require.Equal(t, createdEdge.ID, retrievedEdge.ID)
	require.Equal(t, sourceNode.ID, retrievedEdge.SourceNodeID)

	// 4. Update Edge
	err = client.UpdateEdge(ctx, createdEdge.ID, graph.Properties{"weight": 2.5})
	require.NoError(t, err)

	updatedEdge, err := client.GetEdge(ctx, createdEdge.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedEdge)
	require.Equal(t, 2.5, updatedEdge.Properties["weight"])

	// 5. Delete Edge
	err = client.DeleteEdge(ctx, createdEdge.ID)
	require.NoError(t, err)

	deletedEdge, err := client.GetEdge(ctx, createdEdge.ID)
	require.NoError(t, err)
	require.Nil(t, deletedEdge)
}

func TestQueryConvenienceMethods(t *testing.T) {
	client, teardown := setup(t)
	defer teardown()

	ctx := context.Background()

	// 1. Setup test data
	user1, err := client.CreateNode(ctx, &graph.Node{
		Labels:     []string{"User"},
		Properties: graph.Properties{"name": "Bob"},
	})
	require.NoError(t, err)

	user2, err := client.CreateNode(ctx, &graph.Node{
		Labels:     []string{"User"},
		Properties: graph.Properties{"name": "Charlie"},
	})
	require.NoError(t, err)

	product, err := client.CreateNode(ctx, &graph.Node{
		Labels:     []string{"Product"},
		Properties: graph.Properties{"name": "GraphDB"},
	})
	require.NoError(t, err)

	_, err = client.CreateEdge(ctx, &graph.Edge{
		Label:        "PURCHASED",
		SourceNodeID: user1.ID,
		TargetNodeID: product.ID,
		Properties:   graph.Properties{"year": 2024},
	})
	require.NoError(t, err)

	// 2. Test FindNodes
	findNodesQuery := &graph.Query{
		Match: []graph.Pattern{
			{Alias: "u", Labels: []string{"User"}},
		},
		Return: []graph.Return{
			{Expression: "u"},
		},
	}
	nodes, err := client.FindNodes(ctx, findNodesQuery)
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	// Check if we got Bob and Charlie
	names := make(map[string]struct{})
	for _, n := range nodes {
		names[n.Properties["name"].(string)] = struct{}{}
	}
	require.Contains(t, names, "Bob")
	require.Contains(t, names, "Charlie")
	foundUser2 := false
	for _, n := range nodes {
		if n.ID == user2.ID {
			foundUser2 = true
			break
		}
	}
	require.True(t, foundUser2, "user2 should be found in the results")

	// 3. Test FindEdges
	findEdgesQuery := &graph.Query{
		Match: []graph.Pattern{
			{
				Alias:      "u",
				Labels:     []string{"User"},
				Properties: graph.Properties{"name": "Bob"},
				Edge: &graph.EdgePattern{
					Alias:  "p",
					Labels: []string{"PURCHASED"},
					Node: &graph.Pattern{
						Alias:  "pr",
						Labels: []string{"Product"},
					},
				},
			},
		},
		Return: []graph.Return{
			{Expression: "p"},
		},
	}
	edges, err := client.FindEdges(ctx, findEdgesQuery)
	require.NoError(t, err)
	require.Len(t, edges, 1)
	require.Equal(t, "PURCHASED", edges[0].Label)
	require.EqualValues(t, 2024, edges[0].Properties["year"])

	// 4. Test Count
	count, err := client.Count(ctx, findNodesQuery)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}
