package es

import (
	"fmt"
	"os"

	"github.com/me2seeks/forge/infra/contract/es"
)

type (
	Client          = es.Client
	Types           = es.Types
	BulkIndexer     = es.BulkIndexer
	BulkIndexerItem = es.BulkIndexerItem
	BoolQuery       = es.BoolQuery
	Query           = es.Query
	Response        = es.Response
	Request         = es.Request
)

func New() (Client, error) {
	v := os.Getenv("ES_VERSION")
	switch v {
	case "v8":
		return newES8()
	case "v7":
		return newES7()
	default:
		return nil, fmt.Errorf("unsupported es version %s", v)
	}
}
