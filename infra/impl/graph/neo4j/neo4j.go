package neo4j

import (
	"context"
	"errors"
	"strings"

	"github.com/me2seeks/forge/infra/contract/graph"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/auth"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j/config"
)

type neo4jClient struct {
	driver neo4j.DriverWithContext
}

// Option is a function that configures the neo4j client
type Option func(*options)

// options contains the configuration for the neo4j client
type options struct {
	auth        auth.TokenManager
	configurers []func(*config.Config)
}

// WithAuth sets the authentication token for the client
func WithAuth(auth neo4j.AuthToken) Option {
	return func(o *options) {
		o.auth = auth
	}
}

// WithBasicAuth sets basic authentication for the client
func WithBasicAuth(username, password, realm string) Option {
	return func(o *options) {
		o.auth = neo4j.BasicAuth(username, password, realm)
	}
}

// WithBearerAuth sets bearer authentication for the client
func WithBearerAuth(token string) Option {
	return func(o *options) {
		o.auth = neo4j.BearerAuth(token)
	}
}

// WithKerberosAuth sets kerberos authentication for the client
func WithKerberosAuth(ticket string) Option {
	return func(o *options) {
		o.auth = neo4j.KerberosAuth(ticket)
	}
}

// WithCustomAuth sets custom authentication for the client
func WithCustomAuth(scheme, username, password, realm string, parameters map[string]any) Option {
	return func(o *options) {
		o.auth = neo4j.CustomAuth(scheme, username, password, realm, parameters)
	}
}

// WithNoAuth sets no authentication for the client
func WithNoAuth() Option {
	return func(o *options) {
		o.auth = neo4j.NoAuth()
	}
}

// WithConfigurers adds configurers for the neo4j driver
func WithConfigurers(configurers ...func(*config.Config)) Option {
	return func(o *options) {
		o.configurers = configurers
	}
}

// New creates a new neo4j client with the given options
func New(ctx context.Context, uri string, opts ...Option) (graph.Client, error) {
	// Default options
	o := &options{
		auth: neo4j.NoAuth(),
	}

	// Apply provided options
	for _, opt := range opts {
		opt(o)
	}

	driver, err := neo4j.NewDriverWithContext(uri, o.auth, o.configurers...)
	if err != nil {
		return nil, err
	}

	err = driver.VerifyConnectivity(ctx)
	if err != nil {
		return nil, err
	}

	return &neo4jClient{
		driver: driver,
	}, nil
}

func (c *neo4jClient) CreateNode(ctx context.Context, node *graph.Node) (*graph.Node, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	result, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		cypher := "CREATE (n:`" + node.Labels[0] + "` $props) RETURN n"
		params := map[string]any{"props": node.Properties}
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}

		if res.Next(ctx) {
			record := res.Record()
			createdNode, ok := record.Get("n")
			if !ok {
				return nil, nil // Or an error
			}
			return createdNode.(neo4j.Node), nil
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil // Or a specific error
	}

	neo4jNode := result.(neo4j.Node)
	return &graph.Node{
		ID:         neo4jNode.ElementId,
		Labels:     neo4jNode.Labels,
		Properties: neo4jNode.Props,
	}, nil
}

func (c *neo4jClient) GetNode(ctx context.Context, nodeID string) (*graph.Node, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		cypher := "MATCH (n) WHERE elementId(n) = $id RETURN n"
		params := map[string]any{"id": nodeID}
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}

		if res.Next(ctx) {
			record := res.Record()
			node, ok := record.Get("n")
			if !ok {
				return nil, nil // Or an error
			}
			return node.(neo4j.Node), nil
		}

		return nil, nil // Not found
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil // Not found
	}

	neo4jNode := result.(neo4j.Node)
	return &graph.Node{
		ID:         neo4jNode.ElementId,
		Labels:     neo4jNode.Labels,
		Properties: neo4jNode.Props,
	}, nil
}

func (c *neo4jClient) UpdateNode(ctx context.Context, nodeID string, properties graph.Properties) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		cypher := "MATCH (n) WHERE elementId(n) = $id SET n += $props"
		params := map[string]any{"id": nodeID, "props": properties}
		_, err := tx.Run(ctx, cypher, params)
		return nil, err
	})

	return err
}

func (c *neo4jClient) DeleteNode(ctx context.Context, nodeID string) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		cypher := "MATCH (n) WHERE elementId(n) = $id DETACH DELETE n"
		params := map[string]any{"id": nodeID}
		_, err := tx.Run(ctx, cypher, params)
		return nil, err
	})

	return err
}

func (c *neo4jClient) CreateEdge(ctx context.Context, edge *graph.Edge) (*graph.Edge, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	result, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		cypher := "MATCH (a), (b) WHERE elementId(a) = $sourceId AND elementId(b) = $targetId CREATE (a)-[r:`" + edge.Label + "` $props]->(b) RETURN r"
		params := map[string]any{
			"sourceId": edge.SourceNodeID,
			"targetId": edge.TargetNodeID,
			"props":    edge.Properties,
		}
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		if res.Next(ctx) {
			record := res.Record()
			createdEdge, ok := record.Get("r")
			if !ok {
				return nil, nil // Or an error
			}
			return createdEdge.(neo4j.Relationship), nil
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil // Or a specific error
	}

	neo4jEdge := result.(neo4j.Relationship)
	return &graph.Edge{
		ID:           neo4jEdge.ElementId,
		Label:        neo4jEdge.Type,
		SourceNodeID: neo4jEdge.StartElementId,
		TargetNodeID: neo4jEdge.EndElementId,
		Properties:   neo4jEdge.Props,
	}, nil
}

func (c *neo4jClient) GetEdge(ctx context.Context, edgeID string) (*graph.Edge, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		cypher := "MATCH ()-[r]->() WHERE elementId(r) = $id RETURN r"
		params := map[string]any{"id": edgeID}
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}

		if res.Next(ctx) {
			record := res.Record()
			edge, ok := record.Get("r")
			if !ok {
				return nil, nil // Or an error
			}
			return edge.(neo4j.Relationship), nil
		}

		return nil, nil // Not found
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil // Not found
	}

	neo4jEdge := result.(neo4j.Relationship)
	return &graph.Edge{
		ID:           neo4jEdge.ElementId,
		Label:        neo4jEdge.Type,
		SourceNodeID: neo4jEdge.StartElementId,
		TargetNodeID: neo4jEdge.EndElementId,
		Properties:   neo4jEdge.Props,
	}, nil
}

func (c *neo4jClient) UpdateEdge(ctx context.Context, edgeID string, properties graph.Properties) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		cypher := "MATCH ()-[r]->() WHERE elementId(r) = $id SET r += $props"
		params := map[string]any{"id": edgeID, "props": properties}
		_, err := tx.Run(ctx, cypher, params)
		return nil, err
	})

	return err
}

func (c *neo4jClient) DeleteEdge(ctx context.Context, edgeID string) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		cypher := "MATCH ()-[r]->() WHERE elementId(r) = $id DELETE r"
		params := map[string]any{"id": edgeID}
		_, err := tx.Run(ctx, cypher, params)
		return nil, err
	})

	return err
}

func (c *neo4jClient) CreateNodeIndex(ctx context.Context, label string, properties []string) error {
	// TODO Neo4j 社区版一次只能为一个属性创建索引
	cypher := "CREATE INDEX ON :`" + label + "`(" + properties[0] + ")"
	return c.runCypher(ctx, cypher, nil)
}

func (c *neo4jClient) CreateEdgeIndex(ctx context.Context, label string, properties []string) error {
	// Neo4j 社区版不支持边的索引
	return nil
}

func (c *neo4jClient) CreateConstraint(ctx context.Context, label, property string, constraintType graph.ConstraintType) error {
	var cypher string
	switch constraintType {
	case graph.ConstraintUnique:
		cypher = "CREATE CONSTRAINT ON (n:`" + label + "`) ASSERT n." + property + " IS UNIQUE"
	case graph.ConstraintExists:
		cypher = "CREATE CONSTRAINT ON (n:`" + label + "`) ASSERT exists(n." + property + ")"
	}
	return c.runCypher(ctx, cypher, nil)
}

func (c *neo4jClient) DropNodeIndex(ctx context.Context, label string, properties []string) error {
	cypher := "DROP INDEX ON :`" + label + "`(" + properties[0] + ")"
	return c.runCypher(ctx, cypher, nil)
}

func (c *neo4jClient) DropEdgeIndex(ctx context.Context, label string, properties []string) error {
	// Neo4j 社区版不支持边的索引
	return nil
}

func (c *neo4jClient) DropConstraint(ctx context.Context, label, property string, constraintType graph.ConstraintType) error {
	var cypher string
	switch constraintType {
	case graph.ConstraintUnique:
		cypher = "DROP CONSTRAINT ON (n:`" + label + "`) ASSERT n." + property + " IS UNIQUE"
	case graph.ConstraintExists:
		cypher = "DROP CONSTRAINT ON (n:`" + label + "`) ASSERT exists(n." + property + ")"
	}
	return c.runCypher(ctx, cypher, nil)
}

func (c *neo4jClient) runCypher(ctx context.Context, cypher string, params map[string]any) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, cypher, params)
		return nil, err
	})
	return err
}

func (c *neo4jClient) Query(ctx context.Context, query *graph.Query) (*graph.QueryResult, error) {
	cypher, params := buildCypherQuery(query)

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}

		var records []graph.Record
		for res.Next(ctx) {
			record := res.Record()
			rec := make(graph.Record)
			for _, key := range record.Keys {
				value, _ := record.Get(key)
				rec[key] = toGraphEntity(value)
			}
			records = append(records, rec)
		}
		return &graph.QueryResult{Records: records}, nil
	})
	if err != nil {
		return nil, err
	}

	return result.(*graph.QueryResult), nil
}

func buildCypherQuery(query *graph.Query) (string, map[string]any) {
	// This is a simplified builder. A real implementation would be more robust.
	var sb strings.Builder
	params := make(map[string]any)

	sb.WriteString("MATCH ")
	// Simplified: assumes a single path pattern for now
	p := query.Match[0]
	sb.WriteString("(" + p.Alias + ":" + strings.Join(p.Labels, ":") + ")")
	if p.Edge != nil {
		sb.WriteString("-" + "[" + p.Edge.Alias + ":" + strings.Join(p.Edge.Labels, "|") + "]" + "->")
		sb.WriteString("(" + p.Edge.Node.Alias + ":" + strings.Join(p.Edge.Node.Labels, ":") + ")")
	}

	// WHERE clause would be built here

	sb.WriteString(" RETURN ")
	var returnClauses []string
	for _, r := range query.Return {
		returnClauses = append(returnClauses, r.Alias)
	}
	sb.WriteString(strings.Join(returnClauses, ", "))

	return sb.String(), params
}

func (c *neo4jClient) FindNodes(ctx context.Context, query *graph.Query) ([]*graph.Node, error) {
	result, err := c.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	var nodes []*graph.Node
	nodeIDs := make(map[string]struct{})

	for _, record := range result.Records {
		for _, entity := range record {
			if node, ok := entity.(*graph.Node); ok {
				if _, exists := nodeIDs[node.ID]; !exists {
					nodes = append(nodes, node)
					nodeIDs[node.ID] = struct{}{}
				}
			}
		}
	}
	return nodes, nil
}

func (c *neo4jClient) FindEdges(ctx context.Context, query *graph.Query) ([]*graph.Edge, error) {
	result, err := c.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	var edges []*graph.Edge
	edgeIDs := make(map[string]struct{})

	for _, record := range result.Records {
		for _, entity := range record {
			if edge, ok := entity.(*graph.Edge); ok {
				if _, exists := edgeIDs[edge.ID]; !exists {
					edges = append(edges, edge)
					edgeIDs[edge.ID] = struct{}{}
				}
			}
		}
	}
	return edges, nil
}

func (c *neo4jClient) Count(ctx context.Context, query *graph.Query) (int64, error) {
	cypher, params := buildCypherQuery(query)

	returnIndex := strings.LastIndex(strings.ToUpper(cypher), " RETURN ")
	if returnIndex == -1 {
		return 0, errors.New("invalid query for count: no RETURN clause found")
	}
	countCypher := cypher[:returnIndex] + " RETURN count(*)"

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	count, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, countCypher, params)
		if err != nil {
			return nil, err
		}
		if res.Next(ctx) {
			return res.Record().Values[0], nil
		}
		if err := res.Err(); err != nil {
			return nil, err
		}
		return int64(0), nil
	})
	if err != nil {
		return 0, err
	}

	return count.(int64), nil
}

func toGraphNode(n neo4j.Node) *graph.Node {
	return &graph.Node{
		ID:         n.ElementId,
		Labels:     n.Labels,
		Properties: n.Props,
	}
}

func toGraphEdge(r neo4j.Relationship) *graph.Edge {
	return &graph.Edge{
		ID:           r.ElementId,
		Label:        r.Type,
		SourceNodeID: r.StartElementId,
		TargetNodeID: r.EndElementId,
		Properties:   r.Props,
	}
}

func toGraphEntity(entity any) graph.ResultEntity {
	switch v := entity.(type) {
	case neo4j.Node:
		return toGraphNode(v)
	case neo4j.Relationship:
		return toGraphEdge(v)
	default:
		return v
	}
}

type bulkWriter struct {
	nodes  []*graph.Node
	edges  []*graph.Edge
	client *neo4jClient
}

func (c *neo4jClient) NewBulkWriter() graph.BulkWriter {
	return &bulkWriter{
		client: c,
	}
}

func (b *bulkWriter) AddNode(ctx context.Context, node *graph.Node) error {
	b.nodes = append(b.nodes, node)
	return nil
}

func (b *bulkWriter) AddEdge(ctx context.Context, edge *graph.Edge) error {
	b.edges = append(b.edges, edge)
	return nil
}

func (b *bulkWriter) Close(ctx context.Context) error {
	// This should be a single transaction
	if len(b.nodes) > 0 {
		cypher := "UNWIND $nodes AS nodeProps CREATE (n) SET n = nodeProps"
		params := map[string]any{"nodes": b.nodes} // needs conversion
		if err := b.client.runCypher(ctx, cypher, params); err != nil {
			return err
		}
	}
	// Similar UNWIND for edges
	return nil
}
