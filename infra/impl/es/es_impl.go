package es

import (
	"fmt"
	"os"

	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
	elasticsearchv8 "github.com/elastic/go-elasticsearch/v8"
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
		return NewES8(elasticsearchv8.Config{
			Addresses: []string{os.Getenv("ES_ADDR")},
			Username:  os.Getenv("ES_USERNAME"),
			Password:  os.Getenv("ES_PASSWORD"),
		})
	case "v7":
		return NewES7(elasticsearch7.Config{
			Addresses: []string{os.Getenv("ES_ADDR")},
			Username:  os.Getenv("ES_USERNAME"),
			Password:  os.Getenv("ES_PASSWORD"),
		})
	default:
		return nil, fmt.Errorf("unsupported es version %s", v)
	}
}
