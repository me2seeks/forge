package graph

// --- Query Structure ---

// Query represents a full graph query.
type Query struct {
	Match   []Pattern `json:"match"`
	Where   *Where    `json:"where,omitempty"`
	Return  []Return  `json:"return"`
	OrderBy []Order   `json:"order_by,omitempty"`
	Skip    *int      `json:"skip,omitempty"`
	Limit   *int      `json:"limit,omitempty"`
}

// Pattern defines a graph pattern to match, e.g., (n:Label)-[r:REL]->(m:Label).
type Pattern struct {
	// A unique alias for the node in the query, e.g., "n", "m".
	Alias string
	// Optional: filter by node labels.
	Labels []string
	// Optional: filter by node properties.
	Properties Properties
	// Defines the relationship to the next node in the pattern.
	Edge *EdgePattern
}

// EdgePattern represents the edge part of a pattern.
type EdgePattern struct {
	Alias      string
	Labels     []string
	Properties Properties
	Direction  EdgeDirection
	// MinHops specifies the minimum number of hops for a variable-length path.
	// A nil value defaults to 1.
	MinHops *int
	// MaxHops specifies the maximum number of hops for a variable-length path.
	// A nil value indicates an unbounded path.
	MaxHops *int
	// The next node in the pattern.
	Node *Pattern
}

// EdgeDirection specifies the direction of an edge in a pattern.
type EdgeDirection string

const (
	DirectionOutgoing EdgeDirection = "->"
	DirectionIncoming EdgeDirection = "<-"
	DirectionBoth     EdgeDirection = "--"
)

// --- Filtering ---

// Where clause for complex filtering, borrowing from the es contract's BoolQuery.
type Where struct {
	Filter  []Condition
	Must    []Condition
	MustNot []Condition
	Should  []Condition
}

// Condition is a single filter condition.
type Condition struct {
	// The alias of the node/edge to apply the filter on.
	Alias string
	// The property name.
	Property string
	// The comparison operator.
	Operator Operator
	// The value to compare against.
	Value any
}

// Operator defines the comparison type.
type Operator string

const (
	OpEqual              Operator = "="
	OpNotEqual           Operator = "!="
	OpGreaterThan        Operator = ">"
	OpGreaterThanOrEqual Operator = ">="
	OpLessThan           Operator = "<"
	OpLessThanOrEqual    Operator = "<="
	OpIn                 Operator = "IN"
	OpContains           Operator = "CONTAINS"
)

// --- Return & Result ---

// Return specifies a single item to be returned by the query.
type Return struct {
	// Expression is the item to be returned. It can be a variable from the MATCH clause (e.g., "n", "p"),
	// a property (e.g., "n.name"), or a function call (e.g., "count(n)", "max(n.age)", "length(p)").
	Expression string `json:"expression"`
	// Alias is the name to use for the returned item in the result set.
	// e.g., if Expression is "count(n)" and Alias is "node_count", the result will contain a "node_count" field.
	Alias string `json:"alias,omitempty"`
}

// Order specifies a field to sort the results by.
type Order struct {
	Alias    string
	Property string
	Asc      bool
}

// QueryResult holds the data returned from a query.
type QueryResult struct {
	Records []Record `json:"records"`
}

// Record is a single result row, a map of alias to the returned entity.
type Record map[string]ResultEntity

// ResultEntity can be a Node, an Edge, or a single property value.
type ResultEntity any

// --- Schema ---

// ConstraintType defines the type of constraint to apply.
type ConstraintType string

const (
	// ConstraintUnique ensures that the value of a property is unique for all nodes/edges with a given label.
	ConstraintUnique ConstraintType = "UNIQUE"
	// ConstraintExists ensures that a property exists for all nodes/edges with a given label.
	ConstraintExists ConstraintType = "EXISTS"
)
