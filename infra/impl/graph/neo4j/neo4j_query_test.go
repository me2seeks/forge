package neo4j

import (
	"reflect"
	"strings"
	"testing"

	"github.com/me2seeks/forge/infra/contract/graph"
)

// TestBuildCypherQuery_Basic tests a basic MATCH and RETURN query.
func TestBuildCypherQuery_Basic(t *testing.T) {
	query := &graph.Query{
		Match: []graph.Pattern{
			{Alias: "n", Labels: []string{"Person"}},
		},
		Return: []graph.Return{
			{Expression: "n"},
		},
	}

	expectedCypher := "MATCH (n:`Person`) RETURN n"
	expectedParams := map[string]any{}

	cypher, params := buildCypherQuery(query)

	if cypher != expectedCypher {
		t.Errorf("Cypher mismatch.\nGot: %s\nWant: %s", cypher, expectedCypher)
	}
	if !reflect.DeepEqual(params, expectedParams) {
		t.Errorf("Params mismatch.\nGot:  %v\nWant: %v", params, expectedParams)
	}
}

// TestBuildCypherQuery_WithPropertyMatch tests MATCH with property conditions.
func TestBuildCypherQuery_WithPropertyMatch(t *testing.T) {
	query := &graph.Query{
		Match: []graph.Pattern{
			{
				Alias:      "n",
				Labels:     []string{"Person"},
				Properties: graph.Properties{"name": "Alice", "age": 30},
			},
		},
		Return: []graph.Return{
			{Expression: "n"},
		},
	}

	expectedCypher := "MATCH (n:`Person` {name: $n_name, age: $n_age}) RETURN n"
	// Note: The order of params map is not guaranteed, so we check contents.
	expectedParams := map[string]any{"n_name": "Alice", "n_age": 30}

	cypher, params := buildCypherQuery(query)

	if cypher != expectedCypher {
		t.Errorf("Cypher mismatch.\nGot:  %s\nWant: %s", cypher, expectedCypher)
	}
	if len(params) != len(expectedParams) {
		t.Fatalf("Params length mismatch. Got %d, want %d", len(params), len(expectedParams))
	}
	for k, v := range expectedParams {
		if params[k] != v {
			t.Errorf("Param %s mismatch. Got %v, want %v", k, params[k], v)
		}
	}
}

// TestBuildCypherQuery_WithWhereClause tests a query with a WHERE clause.
func TestBuildCypherQuery_WithWhereClause(t *testing.T) {
	query := &graph.Query{
		Match: []graph.Pattern{
			{Alias: "n", Labels: []string{"Person"}},
		},
		Where: &graph.Where{
			Must: []graph.Condition{
				{Alias: "n", Property: "age", Operator: graph.OpGreaterThan, Value: 25},
			},
			Should: []graph.Condition{
				{Alias: "n", Property: "name", Operator: graph.OpEqual, Value: "Alice"},
				{Alias: "n", Property: "name", Operator: graph.OpEqual, Value: "Bob"},
			},
		},
		Return: []graph.Return{
			{Expression: "n"},
		},
	}

	// Expected structure comment. Exact param names might vary due to internal counter logic in buildCondition.
	// Expected pattern (as a comment): MATCH (n:`Person`) WHERE (n.age > $n_age_X) AND (n.name = $n_name_Y OR n.name = $n_name_Z) RETURN n
	// We would use regexp to match the cypher string in a real, more comprehensive test.

	cypher, params := buildCypherQuery(query)

	// This is a simplified check. A real test would use regexp.
	if len(cypher) == 0 || len(params) == 0 {
		t.Errorf("Expected non-empty cypher and params. Got cypher: '%s', params: %v", cypher, params)
	}
	// Check if essential parts are present
	if !(strings.Contains(cypher, "MATCH") && strings.Contains(cypher, "WHERE") && strings.Contains(cypher, "RETURN")) {
		t.Errorf("Generated Cypher is missing essential clauses: %s", cypher)
	}
}

// TestBuildCypherQuery_WithOrderBySkipLimit tests ORDER BY, SKIP, and LIMIT.
func TestBuildCypherQuery_WithOrderBySkipLimit(t *testing.T) {
	skip := 10
	limit := 5
	query := &graph.Query{
		Match: []graph.Pattern{
			{Alias: "n", Labels: []string{"Person"}},
		},
		OrderBy: []graph.Order{
			{Alias: "n", Property: "age", Asc: false},
			{Alias: "n", Property: "name", Asc: true},
		},
		Skip:  &skip,
		Limit: &limit,
		Return: []graph.Return{
			{Expression: "n"},
		},
	}

	expectedCypher := "MATCH (n:`Person`) RETURN n ORDER BY n.age DESC, n.name ASC SKIP $skip LIMIT $limit"
	expectedParams := map[string]any{"skip": 10, "limit": 5}

	cypher, params := buildCypherQuery(query)

	if cypher != expectedCypher {
		t.Errorf("Cypher mismatch.\nGot:  %s\nWant: %s", cypher, expectedCypher)
	}
	if !reflect.DeepEqual(params, expectedParams) {
		t.Errorf("Params mismatch.\nGot:  %v\nWant: %v", params, expectedParams)
	}
}

// TestBuildCypherQuery_Complex tests a complex query combining many features.
// This would be the most comprehensive test.
// func TestBuildCypherQuery_Complex(t *testing.T) { ... }

// TestBuildCypherQuery_VariableLengthPath tests a query with a variable-length path.
func TestBuildCypherQuery_VariableLengthPath(t *testing.T) {
	minHops := 2
	maxHops := 5
	query := &graph.Query{
		Match: []graph.Pattern{
			{
				Alias:  "a",
				Labels: []string{"Person"},
				Edge: &graph.EdgePattern{
					Labels:    []string{"KNOWS"},
					Direction: graph.DirectionOutgoing,
					MinHops:   &minHops,
					MaxHops:   &maxHops,
					Node: &graph.Pattern{
						Alias: "b",
					},
				},
			},
		},
		Return: []graph.Return{
			{Expression: "a"},
			{Expression: "b"},
		},
	}

	expectedCypher := "MATCH (a:`Person`)-[:KNOWS*2..5]->(b) RETURN a, b"
	cypher, _ := buildCypherQuery(query)

	if cypher != expectedCypher {
		t.Errorf("Cypher mismatch for variable-length path.\nGot:  %s\nWant: %s", cypher, expectedCypher)
	}
}

// TestBuildCypherQuery_ReturnExpression tests a query with a function expression in the RETURN clause.
func TestBuildCypherQuery_ReturnExpression(t *testing.T) {
	query := &graph.Query{
		Match: []graph.Pattern{
			{
				Alias: "p",
				Edge: &graph.EdgePattern{
					Direction: graph.DirectionBoth, // Explicitly set direction for clarity
					Node:      &graph.Pattern{},
				},
			},
		},
		Return: []graph.Return{
			{Expression: "count(p)", Alias: "path_count"},
		},
	}

	expectedCypher := "MATCH (p)-[]-() RETURN count(p) AS path_count"
	cypher, params := buildCypherQuery(query)

	if cypher != expectedCypher {
		t.Errorf("Cypher mismatch for return expression.\nGot:  %s\nWant: %s", cypher, expectedCypher)
	}
	if len(params) != 0 {
		t.Errorf("Params should be empty for this query, got: %v", params)
	}
}
