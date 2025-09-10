package es

import (
	"encoding/json"
	"io"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/totalhitsrelation"
)

type BulkIndexerItem struct {
	Index           string
	Action          string
	DocumentID      string
	Routing         string
	Version         *int64
	VersionType     string
	Body            io.ReadSeeker
	RetryOnConflict *int
}

// Script represents an Elasticsearch script, used for UpdateByQuery operations.
// It defines the logic for how documents should be updated.
type Script struct {
	// Lang is the scripting language. Common values are "painless", "expression", "mustache".
	// If empty, "painless" is typically used by default.
	Lang string `json:"lang,omitempty"`
	// Source is the script source code.
	Source string `json:"source"`
	// Params is a map of parameters that can be used within the script.
	Params map[string]any `json:"params,omitempty"`
}

type Request struct {
	Size        *int
	Query       *Query
	MinScore    *float64
	Sort        []SortFiled
	SearchAfter []any
	From        *int
}

type SortFiled struct {
	Field string
	Asc   bool
}

type Response struct {
	Hits     HitsMetadata `json:"hits"`
	MaxScore *float64     `json:"max_score,omitempty"`
}

type HitsMetadata struct {
	Hits     []Hit    `json:"hits"`
	MaxScore *float64 `json:"max_score,omitempty"`
	// Total Total hit count information, present only if `track_total_hits` wasn't
	// `false` in the search request.
	Total *TotalHits `json:"total,omitempty"`
}

type Hit struct {
	Id_     *string         `json:"_id,omitempty"`
	Score_  *float64        `json:"_score,omitempty"`
	Source_ json.RawMessage `json:"_source,omitempty"`
}

type TotalHits struct {
	Relation totalhitsrelation.TotalHitsRelation `json:"relation"`
	Value    int64                               `json:"value"`
}
