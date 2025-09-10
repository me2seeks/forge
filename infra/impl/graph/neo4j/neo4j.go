package neo4j

import (
	"context"
	"fmt"
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
		var cypher string
		params := make(map[string]any)

		// Check if selectors are provided
		if edge.SourceNodeSelector != nil && edge.TargetNodeSelector != nil {
			// Build MATCH clauses for source and target nodes based on selectors
			sourceMatch, sourceParams := buildNodeMatchClause("a", edge.SourceNodeSelector)
			targetMatch, targetParams := buildNodeMatchClause("b", edge.TargetNodeSelector)

			// Merge parameters
			for k, v := range sourceParams {
				params[k] = v
			}
			for k, v := range targetParams {
				params[k] = v
			}
			params["props"] = edge.Properties

			// Construct the full Cypher query
			cypher = fmt.Sprintf("%s %s CREATE (a)-[r:`%s` $props]->(b) RETURN r", sourceMatch, targetMatch, edge.Label)
		} else {
			// Fallback to the original implementation using element IDs
			cypher = "MATCH (a), (b) WHERE elementId(a) = $sourceId AND elementId(b) = $targetId CREATE (a)-[r:`" + edge.Label + "` $props]->(b) RETURN r"
			params = map[string]any{
				"sourceId": edge.SourceNodeID,
				"targetId": edge.TargetNodeID,
				"props":    edge.Properties,
			}
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

// UpdateNodesByQuery updates properties of all nodes matching the query.
func (c *neo4jClient) UpdateNodesByQuery(ctx context.Context, query *graph.Query, properties graph.Properties) (int, error) {
	// We need to know the aliases to build the RETURN count clause.
	// Collect aliases from MATCH for the operation clause generator and RETURN clause
	var aliasesInMatch []string
	for _, p := range query.Match {
		// If alias is empty, use a default one
		alias := p.Alias
		if alias == "" {
			alias = "n" // Default alias for nodes
		}
		aliasesInMatch = append(aliasesInMatch, alias)
		if p.Edge != nil {
			// If edge alias is empty, use a default one
			edgeAlias := p.Edge.Alias
			if edgeAlias == "" {
				edgeAlias = "r" // Default alias for relationships
			}
			aliasesInMatch = append(aliasesInMatch, edgeAlias)
			if p.Edge.Node != nil {
				// If edge node alias is empty, use a default one
				edgeNodeAlias := p.Edge.Node.Alias
				if edgeNodeAlias == "" {
					edgeNodeAlias = "m" // Default alias for nodes connected by relationship
				}
				aliasesInMatch = append(aliasesInMatch, edgeNodeAlias)
			}
		}
	}

	// Define the operation clause generator for SET
	opClauseGenerator := func(aliasesInMatchForOp []string) (string, map[string]any) {
		// Heuristic: Assume the first node alias in the MATCH clause is the target for update.
		// A more robust solution might require the query or method to specify the target alias.
		if len(aliasesInMatchForOp) == 0 {
			// If no aliases, we can't build a SET clause. This should ideally be an error.
			return "", make(map[string]any)
		}
		targetAlias := aliasesInMatchForOp[0]
		return buildSetClause(targetAlias, properties)
	}

	cypher, params := buildCypherQueryForOperation(query, opClauseGenerator)
	// Append RETURN count for affected nodes
	if len(aliasesInMatch) > 0 {
		cypher += " RETURN count(" + aliasesInMatch[0] + ")"
	} else {
		cypher += " RETURN count(*)"
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	result, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		if res.Next(ctx) {
			key := "count(*)"
			if len(aliasesInMatch) > 0 {
				key = "count(" + aliasesInMatch[0] + ")"
			}
			count, _ := res.Record().Get(key)
			return count, nil
		}
		return int64(0), nil
	})
	if err != nil {
		return 0, err
	}

	return int(result.(int64)), nil
}

// UpdateEdgesByQuery updates properties of all edges matching the query.
// The implementation is very similar to UpdateNodesByQuery.
func (c *neo4jClient) UpdateEdgesByQuery(ctx context.Context, query *graph.Query, properties graph.Properties) (int, error) {
	// Determine edge alias for SET clause and RETURN count
	edgeAlias := ""
	if len(query.Match) > 0 && query.Match[0].Edge != nil && query.Match[0].Edge.Alias != "" {
		edgeAlias = query.Match[0].Edge.Alias
	}

	// Define the operation clause generator for SET
	opClauseGenerator := func(aliasesInMatchForOp []string) (string, map[string]any) {
		// Use the pre-determined edge alias
		if edgeAlias == "" {
			return "", make(map[string]any)
		}
		return buildSetClause(edgeAlias, properties)
	}

	cypher, params := buildCypherQueryForOperation(query, opClauseGenerator)
	// Append RETURN count for affected edges
	if edgeAlias != "" {
		cypher += " RETURN count(" + edgeAlias + ")"
	} else {
		cypher += " RETURN count(*)"
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	result, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		if res.Next(ctx) {
			key := "count(*)"
			if edgeAlias != "" {
				key = "count(" + edgeAlias + ")"
			}
			count, _ := res.Record().Get(key)
			return count, nil
		}
		return int64(0), nil
	})
	if err != nil {
		return 0, err
	}

	return int(result.(int64)), nil
}

// DeleteNodesByQuery deletes all nodes matching the query.
func (c *neo4jClient) DeleteNodesByQuery(ctx context.Context, query *graph.Query) (int, error) {
	// Collect aliases from MATCH
	var aliasesInMatch []string
	for _, p := range query.Match {
		// If alias is empty, use a default one
		alias := p.Alias
		if alias == "" {
			alias = "n" // Default alias for nodes
		}
		aliasesInMatch = append(aliasesInMatch, alias)
		if p.Edge != nil {
			// If edge alias is empty, use a default one
			edgeAlias := p.Edge.Alias
			if edgeAlias == "" {
				edgeAlias = "r" // Default alias for relationships
			}
			aliasesInMatch = append(aliasesInMatch, edgeAlias)
			if p.Edge.Node != nil {
				// If edge node alias is empty, use a default one
				edgeNodeAlias := p.Edge.Node.Alias
				if edgeNodeAlias == "" {
					edgeNodeAlias = "m" // Default alias for nodes connected by relationship
				}
				aliasesInMatch = append(aliasesInMatch, edgeNodeAlias)
			}
		}
	}

	opClauseGenerator := func(aliasesInMatchForOp []string) (string, map[string]any) {
		// Heuristic: Assume the first node alias in the MATCH clause is the target for deletion.
		if len(aliasesInMatchForOp) == 0 {
			return "", make(map[string]any)
		}
		targetAlias := aliasesInMatchForOp[0]
		// Neo4j requires DETACH DELETE for nodes to remove relationships too.
		return "DETACH DELETE " + targetAlias, make(map[string]any)
	}

	cypher, params := buildCypherQueryForOperation(query, opClauseGenerator)
	// Append RETURN count for deleted nodes
	if len(aliasesInMatch) > 0 {
		cypher += " RETURN count(" + aliasesInMatch[0] + ")"
	} else {
		cypher += " RETURN count(*)"
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	result, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		if res.Next(ctx) {
			key := "count(*)"
			if len(aliasesInMatch) > 0 {
				key = "count(" + aliasesInMatch[0] + ")"
			}
			count, _ := res.Record().Get(key)
			return count, nil
		}
		return int64(0), nil
	})
	if err != nil {
		return 0, err
	}

	return int(result.(int64)), nil
}

// DeleteEdgesByQuery deletes all edges matching the query.
func (c *neo4jClient) DeleteEdgesByQuery(ctx context.Context, query *graph.Query) (int, error) {
	// Determine edge alias for DELETE clause and RETURN count
	edgeAlias := ""
	if len(query.Match) > 0 && query.Match[0].Edge != nil && query.Match[0].Edge.Alias != "" {
		edgeAlias = query.Match[0].Edge.Alias
	}

	// Define the operation clause generator for DELETE
	opClauseGenerator := func(aliasesInMatchForOp []string) (string, map[string]any) {
		// Use the pre-determined edge alias
		if edgeAlias == "" {
			return "", make(map[string]any)
		}
		// For edges, it's just DELETE
		return "DELETE " + edgeAlias, make(map[string]any)
	}

	cypher, params := buildCypherQueryForOperation(query, opClauseGenerator)
	// Append RETURN count for deleted edges
	if edgeAlias != "" {
		cypher += " RETURN count(" + edgeAlias + ")"
	} else {
		cypher += " RETURN count(*)"
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	result, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}
		if res.Next(ctx) {
			key := "count(*)"
			if edgeAlias != "" {
				key = "count(" + edgeAlias + ")"
			}
			count, _ := res.Record().Get(key)
			return count, nil
		}
		return int64(0), nil
	})
	if err != nil {
		return 0, err
	}

	return int(result.(int64)), nil
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

// buildMatchClause generates the MATCH part of the Cypher query.
func buildMatchClause(matchPatterns []graph.Pattern, params map[string]any) string {
	if len(matchPatterns) == 0 {
		return ""
	}

	var matchParts []string
	for _, p := range matchPatterns {
		var sb strings.Builder
		// Handle path alias assignment, e.g., "p = (n)-[]->(m)"
		if p.PathAlias != "" {
			sb.WriteString(p.PathAlias)
			sb.WriteString(" = ")
		}
		// Build node pattern: (alias:Label1:Label2 {prop1: $param1, prop2: $param2})
		sb.WriteString("(")
		// If alias is empty, use a default one
		alias := p.Alias
		if alias == "" {
			alias = "n" // Default alias for nodes
		}
		sb.WriteString(alias)
		// Correctly format multiple node labels, e.g., :Person:Manager
		for _, label := range p.Labels {
			sb.WriteString(":`")
			sb.WriteString(label)
			sb.WriteString("`")
		}
		if len(p.Properties) > 0 {
			sb.WriteString(" {")
			propStrings := make([]string, 0, len(p.Properties))
			for key, value := range p.Properties {
				paramName := alias + "_" + key
				params[paramName] = value
				propStrings = append(propStrings, key+": $"+paramName)
			}
			sb.WriteString(strings.Join(propStrings, ", "))
			sb.WriteString("}")
		}
		sb.WriteString(")")

		// Build edge pattern if it exists
		if p.Edge != nil {
			edgePattern := p.Edge
			// Direction
			switch edgePattern.Direction {
			case graph.DirectionIncoming:
				sb.WriteString("<-")
			case graph.DirectionBoth:
				sb.WriteString("-")
			default: // DirectionOutgoing or empty
				sb.WriteString("-")
			}
			// Edge part: [alias:TYPE1|TYPE2*min..max {prop1: $param1}]
			sb.WriteString("[")
			// If edge alias is empty, use a default one
			edgeAlias := edgePattern.Alias
			if edgeAlias == "" {
				edgeAlias = "r" // Default alias for relationships
			}
			sb.WriteString(edgeAlias)
			if len(edgePattern.Labels) > 0 {
				// Correctly format multiple relationship types, e.g., :KNOWS|LOVES
				sb.WriteString(":")
				sb.WriteString(strings.Join(edgePattern.Labels, "|"))
			}

			// Handle variable-length path syntax
			if edgePattern.MinHops != nil || edgePattern.MaxHops != nil {
				sb.WriteString("*")
				// Case 1: Fixed length, e.g., *3
				if edgePattern.MinHops != nil && edgePattern.MaxHops != nil && *edgePattern.MinHops == *edgePattern.MaxHops {
					sb.WriteString(fmt.Sprintf("%d", *edgePattern.MinHops))
				} else {
					// Case 2: Bounded range, e.g., *2..5
					if edgePattern.MinHops != nil {
						sb.WriteString(fmt.Sprintf("%d", *edgePattern.MinHops))
					}
					sb.WriteString("..")
					// Case 3: Unbounded range, e.g., *2.. or *..5
					if edgePattern.MaxHops != nil {
						sb.WriteString(fmt.Sprintf("%d", *edgePattern.MaxHops))
					}
				}
			}
			if len(edgePattern.Properties) > 0 {
				sb.WriteString(" {")
				propStrings := make([]string, 0, len(edgePattern.Properties))
				for key, value := range edgePattern.Properties {
					paramName := edgeAlias + "_" + key
					params[paramName] = value
					propStrings = append(propStrings, key+": $"+paramName)
				}
				sb.WriteString(strings.Join(propStrings, ", "))
				sb.WriteString("}")
			}
			sb.WriteString("]")
			// Direction suffix
			switch edgePattern.Direction {
			case graph.DirectionIncoming:
				sb.WriteString("-")
			case graph.DirectionBoth:
				sb.WriteString("-")
			default: // DirectionOutgoing or empty
				sb.WriteString("->")
			}

			// Recursively build the next node pattern
			if edgePattern.Node != nil {
				// For simplicity in this step, we'll just append the node's basic pattern.
				// A full implementation might recursively call a node pattern builder.
				sb.WriteString("(")
				// If edge node alias is empty, use a default one
				edgeNodeAlias := edgePattern.Node.Alias
				if edgeNodeAlias == "" {
					edgeNodeAlias = "m" // Default alias for nodes connected by relationship
				}
				sb.WriteString(edgeNodeAlias)
				// Correctly format multiple node labels for the target node
				for _, label := range edgePattern.Node.Labels {
					sb.WriteString(":`")
					sb.WriteString(label)
					sb.WriteString("`")
				}
				// Note: For full recursive support, we'd also handle edgePattern.Node.Properties here
				// and in the params map. For now, we keep it simple for the first step.
				sb.WriteString(")")
			}
		}
		matchParts = append(matchParts, sb.String())
	}
	return "MATCH " + strings.Join(matchParts, ", ")
}

// buildCondition translates a graph.Condition into a Cypher condition string and updates the params map.
func buildCondition(cond graph.Condition, params map[string]any) string {
	var sb strings.Builder
	sb.WriteString(cond.Alias)
	sb.WriteString(".")
	sb.WriteString(cond.Property)

	// Generate a unique parameter name
	paramName := cond.Alias + "_" + cond.Property
	// Handle potential name collisions by appending a counter if needed
	// A more robust solution might be needed for complex cases, but this is a start.
	counter := 0
	originalParamName := paramName
	for {
		if _, exists := params[paramName]; !exists {
			break
		}
		counter++
		paramName = fmt.Sprintf("%s_%d", originalParamName, counter)
	}

	params[paramName] = cond.Value

	switch cond.Operator {
	case graph.OpEqual:
		sb.WriteString(" = ")
	case graph.OpNotEqual:
		sb.WriteString(" <> ")
	case graph.OpGreaterThan:
		sb.WriteString(" > ")
	case graph.OpGreaterThanOrEqual:
		sb.WriteString(" >= ")
	case graph.OpLessThan:
		sb.WriteString(" < ")
	case graph.OpLessThanOrEqual:
		sb.WriteString(" <= ")
	case graph.OpIn:
		sb.WriteString(" IN ")
	case graph.OpContains:
		sb.WriteString(" CONTAINS ")
	default:
		// Default to equality if operator is unknown
		sb.WriteString(" = ")
	}
	sb.WriteString("$")
	sb.WriteString(paramName)

	return sb.String()
}

// buildWhereClause generates the WHERE part of the Cypher query.
func buildWhereClause(where *graph.Where, params map[string]any) string {
	if where == nil {
		return ""
	}

	var clauses []string

	// Handle Filter (treated as AND)
	if len(where.Filter) > 0 {
		var filterParts []string
		for _, cond := range where.Filter {
			filterParts = append(filterParts, buildCondition(cond, params))
		}
		clauses = append(clauses, strings.Join(filterParts, " AND "))
	}

	// Handle Must (AND)
	if len(where.Must) > 0 {
		var mustParts []string
		for _, cond := range where.Must {
			mustParts = append(mustParts, buildCondition(cond, params))
		}
		clauses = append(clauses, "("+strings.Join(mustParts, " AND ")+")")
	}

	// Handle Should (OR)
	if len(where.Should) > 0 {
		var shouldParts []string
		for _, cond := range where.Should {
			shouldParts = append(shouldParts, buildCondition(cond, params))
		}
		clauses = append(clauses, "("+strings.Join(shouldParts, " OR ")+")")
	}

	// Handle MustNot (NOT)
	if len(where.MustNot) > 0 {
		var mustNotParts []string
		for _, cond := range where.MustNot {
			mustNotParts = append(mustNotParts, buildCondition(cond, params))
		}
		clauses = append(clauses, "NOT ("+strings.Join(mustNotParts, " AND ")+")")
	}

	if len(clauses) == 0 {
		return ""
	}

	return "WHERE " + strings.Join(clauses, " AND ")
}

// buildCypherQuery translates a graph.Query into a Cypher query string and its parameters.
// It is kept for backward compatibility and refactored to use the more flexible buildCypherQueryForOperation.
func buildCypherQuery(query *graph.Query) (string, map[string]any) {
	// Define the operation clause generator for RETURN
	opClauseGenerator := func(aliasesInMatch []string) (string, map[string]any) {
		var sb strings.Builder
		params := make(map[string]any) // Local params for this operation clause
		sb.WriteString("RETURN ")

		var returnClauses []string
		for _, r := range query.Return {
			clause := r.Expression
			if r.Alias != "" {
				clause += " AS " + r.Alias
			}
			returnClauses = append(returnClauses, clause)
		}
		if len(returnClauses) == 0 {
			// If no return clauses are specified, return everything from MATCH
			// This is a simple heuristic; a more robust solution might be needed.
			// Re-use the aliasesInMatch passed from buildCypherQueryForOperation
			returnClauses = aliasesInMatch
		}
		sb.WriteString(strings.Join(returnClauses, ", "))

		// --- ORDER BY Clause ---
		if len(query.OrderBy) > 0 {
			var orderParts []string
			for _, o := range query.OrderBy {
				// Note: OrderBy might need adjustment if it's ordering by an aliased expression.
				// This implementation assumes ordering by a property on a variable.
				orderStr := o.Alias + "." + o.Property
				if !o.Asc {
					orderStr += " DESC"
				} else {
					orderStr += " ASC"
				}
				orderParts = append(orderParts, orderStr)
			}
			sb.WriteString(" ORDER BY ")
			sb.WriteString(strings.Join(orderParts, ", "))
		}

		// --- SKIP Clause ---
		if query.Skip != nil {
			params["skip"] = *query.Skip
			sb.WriteString(" SKIP $skip")
		}

		// --- LIMIT Clause ---
		if query.Limit != nil {
			params["limit"] = *query.Limit
			sb.WriteString(" LIMIT $limit")
		}

		return sb.String(), params // Return the clause and its specific params
	}

	return buildCypherQueryForOperation(query, opClauseGenerator)
}

// buildCypherQueryForOperation is a more flexible builder that can construct different types of Cypher queries
// based on the operation type (RETURN, SET, DELETE).
// opClauseGenerator is a function that generates the operation-specific part of the query (e.g., "RETURN n", "SET n.prop = $val", "DETACH DELETE n")
// and returns the clause string and any additional parameters it needs.
func buildCypherQueryForOperation(query *graph.Query, opClauseGenerator func(aliasesInMatch []string) (string, map[string]any)) (string, map[string]any) {
	var sb strings.Builder
	params := make(map[string]any)

	// --- MATCH Clause ---
	matchClause := buildMatchClause(query.Match, params)
	if matchClause != "" {
		sb.WriteString(matchClause)
		sb.WriteString(" ")
	}

	// --- WHERE Clause ---
	whereClause := buildWhereClause(query.Where, params)
	if whereClause != "" {
		sb.WriteString(whereClause)
		sb.WriteString(" ")
	}

	// --- Operation Clause (RETURN, SET, DELETE) ---
	// Collect aliases from MATCH for the operation clause generator
	var aliasesInMatch []string
	for _, p := range query.Match {
		// If alias is empty, use a default one
		alias := p.Alias
		if alias == "" {
			alias = "n" // Default alias for nodes
		}
		aliasesInMatch = append(aliasesInMatch, alias)
		if p.Edge != nil {
			// If edge alias is empty, use a default one
			edgeAlias := p.Edge.Alias
			if edgeAlias == "" {
				edgeAlias = "r" // Default alias for relationships
			}
			aliasesInMatch = append(aliasesInMatch, edgeAlias)
			if p.Edge.Node != nil {
				// If edge node alias is empty, use a default one
				edgeNodeAlias := p.Edge.Node.Alias
				if edgeNodeAlias == "" {
					edgeNodeAlias = "m" // Default alias for nodes connected by relationship
				}
				aliasesInMatch = append(aliasesInMatch, edgeNodeAlias)
			}
		}
	}

	opClause, opParams := opClauseGenerator(aliasesInMatch)
	sb.WriteString(opClause)

	// Merge operation-specific params
	for k, v := range opParams {
		params[k] = v
	}

	// TODO: ORDER BY, SKIP, LIMIT are typically only used with RETURN.
	// If needed for other operations, they can be added here conditionally.
	// For now, we keep it simple and focused on the core operation.

	return sb.String(), params
}

// buildSetClause generates a Cypher SET clause for updating properties.
// It assumes the primary node to update has the alias 'n'.
// A more sophisticated version could take the target alias as a parameter.
func buildSetClause(alias string, properties graph.Properties) (string, map[string]any) {
	if len(properties) == 0 {
		return "", make(map[string]any)
	}

	var setParts []string
	params := make(map[string]any)
	for key, value := range properties {
		// Generate a unique parameter name to avoid conflicts
		paramName := fmt.Sprintf("%s_set_%s_%d", alias, key, len(params)) // Simple uniqueness
		params[paramName] = value
		setParts = append(setParts, fmt.Sprintf("%s.%s = $%s", alias, key, paramName))
	}
	return "SET " + strings.Join(setParts, ", "), params
}

// buildNodeMatchClause generates a Cypher MATCH clause for a node based on its selector.
// It returns the MATCH clause string and the parameters map.
func buildNodeMatchClause(alias string, selector *graph.NodeSelector) (string, map[string]any) {
	if selector == nil {
		return "", make(map[string]any)
	}

	var sb strings.Builder
	sb.WriteString("(")
	sb.WriteString(alias)

	// Add labels
	for _, label := range selector.Labels {
		sb.WriteString(":`")
		sb.WriteString(label)
		sb.WriteString("`")
	}

	// Add properties
	if len(selector.Properties) > 0 {
		sb.WriteString(" {")
		propStrings := make([]string, 0, len(selector.Properties))
		params := make(map[string]any)
		for key, value := range selector.Properties {
			paramName := alias + "_" + key
			params[paramName] = value
			propStrings = append(propStrings, key+": $"+paramName)
		}
		sb.WriteString(strings.Join(propStrings, ", "))
		sb.WriteString("}")
		sb.WriteString(")")
		return "MATCH " + sb.String(), params
	}

	sb.WriteString(")")
	return "MATCH " + sb.String(), make(map[string]any)
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
	// Define the operation clause generator for COUNT
	opClauseGenerator := func(aliasesInMatch []string) (string, map[string]any) {
		// Heuristic: count the first alias in the MATCH clause.
		if len(aliasesInMatch) > 0 {
			return "RETURN count(" + aliasesInMatch[0] + ")", nil
		}
		// Fallback if no aliases are found
		return "RETURN count(*)", nil
	}

	cypher, params := buildCypherQueryForOperation(query, opClauseGenerator)

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	count, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypher, params)
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
	case []any:
		return handleInterfaceSlice(v)
	default:
		return v
	}
}

func handleInterfaceSlice(slice []any) graph.ResultEntity {
	if isAllRelationships(slice) {
		return convertToEdges(slice)
	}

	if isAllNodes(slice) {
		return convertToNodes(slice)
	}

	return handleMixedTypes(slice)
}

func isAllRelationships(slice []any) bool {
	for _, item := range slice {
		if _, ok := item.(neo4j.Relationship); !ok {
			return false
		}
	}
	return len(slice) > 0
}

func convertToEdges(slice []any) []*graph.Edge {
	edges := make([]*graph.Edge, len(slice))
	for i, item := range slice {
		if rel, ok := item.(neo4j.Relationship); ok {
			edges[i] = toGraphEdge(rel)
		}
	}
	return edges
}

func isAllNodes(slice []any) bool {
	for _, item := range slice {
		if _, ok := item.(neo4j.Node); !ok {
			return false
		}
	}
	return len(slice) > 0
}

func convertToNodes(slice []any) []*graph.Node {
	nodes := make([]*graph.Node, len(slice))
	for i, item := range slice {
		if node, ok := item.(neo4j.Node); ok {
			nodes[i] = toGraphNode(node)
		}
	}
	return nodes
}

func handleMixedTypes(slice []any) graph.ResultEntity {
	var result []any
	for _, item := range slice {
		switch v := item.(type) {
		case neo4j.Node:
			result = append(result, toGraphNode(v))
		case neo4j.Relationship:
			result = append(result, toGraphEdge(v))
		default:
			result = append(result, v)
		}
	}
	return result
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

// --- Graph Algorithms ---

func (c *neo4jClient) ShortestPath(ctx context.Context, sourceNodeID, targetNodeID string, config map[string]any) ([]*graph.Path, error) {
	// TODO: Implement with GDS or APOC
	return nil, fmt.Errorf("not implemented")
}

func (c *neo4jClient) PageRank(ctx context.Context, config map[string]any) (map[string]float64, error) {
	// TODO: Implement with GDS
	return nil, fmt.Errorf("not implemented")
}

func (c *neo4jClient) ConnectedComponents(ctx context.Context, config map[string]any) (map[string]string, error) {
	// TODO: Implement with GDS
	return nil, fmt.Errorf("not implemented")
}

func (c *neo4jClient) BetweennessCentrality(ctx context.Context, config map[string]any) (map[string]float64, error) {
	// TODO: Implement with GDS
	return nil, fmt.Errorf("not implemented")
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
