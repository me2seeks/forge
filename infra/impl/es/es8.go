package es

import (
	"context"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esutil"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/create"
	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/delete"
	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/exists"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/operator"
	esrefresh "github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/textquerytype"

	"github.com/me2seeks/forge/infra/contract/es"
	"github.com/me2seeks/forge/logs"
	"github.com/me2seeks/forge/prelude/conv"
	"github.com/me2seeks/forge/prelude/ptr"
	"github.com/me2seeks/forge/sonic"
)

type es8Client struct {
	esClient *elasticsearch.TypedClient
	types    *es8Types
}

type es8BulkIndexer struct {
	bi esutil.BulkIndexer
}

type es8Types struct{}

func NewES8(cfg elasticsearch.Config) (Client, error) {
	esClient, err := elasticsearch.NewTypedClient(cfg)
	if err != nil {
		return nil, err
	}

	return &es8Client{
		esClient: esClient,
		types:    &es8Types{},
	}, nil
}

func (c *es8Client) Create(ctx context.Context, index, id string, document any, refresh bool) error {
	// Create an index request
	req := c.esClient.Index(index).Document(document)

	// If id is not empty, use PUT method with the specified id
	// If id is empty, use POST method to let ES generate an id
	if id != "" {
		req = req.Id(id)
	}

	if refresh {
		req.Refresh(esrefresh.True)
	}

	_, err := req.Do(ctx)
	return err
}

func (c *es8Client) Update(ctx context.Context, index, id string, document any, refresh bool) error {
	req := c.esClient.Update(index, id).Doc(document)
	if refresh {
		req.Refresh(esrefresh.True)
	}
	_, err := req.Do(ctx)
	return err
}

func (c *es8Client) Delete(ctx context.Context, index, id string, refresh bool) error {
	req := c.esClient.Delete(index, id)
	if refresh {
		req.Refresh(esrefresh.True)
	}
	_, err := req.Do(ctx)
	return err
}

// UpdateByQuery updates documents that match a query.
func (c *es8Client) UpdateByQuery(ctx context.Context, index string, query *es.Query, script *es.Script, refresh bool) error {
	// Start building the request
	req := c.esClient.UpdateByQuery(index)
	if refresh {
		req.Refresh(true)
	}

	// Set the query
	if query != nil {
		req = req.Query(c.query2ESQuery(query))
	}

	// Set the script
	if script != nil {
		// Build the script object for ES v8 typed client
		esScript := types.Script{
			Source: &script.Source,
			// Note: Lang and Params are omitted for simplicity.
			// If needed, they would require specific type conversions
			// that depend on the exact version of the ES client library.
		}
		req = req.Script(&esScript)
	}

	// Execute the request
	_, err := req.Do(ctx)
	if err != nil {
		return err
	}

	// The typed client handles error checking internally for Do().
	// If Do() returns without error, the operation was successful.
	return nil
}

// DeleteByQuery deletes documents that match a query.
func (c *es8Client) DeleteByQuery(ctx context.Context, index string, query *es.Query, refresh bool) error {
	// Start building the request
	req := c.esClient.DeleteByQuery(index)
	if refresh {
		req.Refresh(true)
	}

	// Set the query
	if query != nil {
		req = req.Query(c.query2ESQuery(query))
	}

	// Execute the request
	_, err := req.Do(ctx)
	if err != nil {
		return err
	}

	// The typed client handles error checking internally for Do().
	// If Do() returns without error, the operation was successful.
	return nil
}

func (c *es8Client) Exists(ctx context.Context, index string) (bool, error) {
	exist, err := exists.NewExistsFunc(c.esClient)(index).Do(ctx)
	if err != nil {
		return false, err
	}

	return exist, nil
}

func (c *es8Client) Count(ctx context.Context, index string, query *Query) (int64, error) {
	resp, err := c.esClient.Count().Index(index).Query(c.query2ESQuery(query)).Do(ctx)
	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}

func (c *es8Client) query2ESQuery(q *Query) *types.Query {
	if q == nil {
		return nil
	}

	var typesQ *types.Query
	switch q.Type {
	case es.QueryTypeEqual:
		typesQ = &types.Query{
			Term: map[string]types.TermQuery{
				q.KV.Key: {Value: q.KV.Value},
			},
		}
	case es.QueryTypeMatch:
		typesQ = &types.Query{
			Match: map[string]types.MatchQuery{
				q.KV.Key: {Query: fmt.Sprint(q.KV.Value)},
			},
		}
	case es.QueryTypeMultiMatch:
		typesQ = &types.Query{
			MultiMatch: &types.MultiMatchQuery{
				Fields:   q.MultiMatchQuery.Fields,
				Operator: &operator.Operator{Name: q.MultiMatchQuery.Operator},
				Query:    q.MultiMatchQuery.Query,
				Type:     &textquerytype.TextQueryType{Name: q.MultiMatchQuery.Type},
			},
		}
	case es.QueryTypeNotExists:
		typesQ = &types.Query{
			Bool: &types.BoolQuery{
				MustNot: []types.Query{{Exists: &types.ExistsQuery{Field: q.KV.Key}}},
			},
		}
	case es.QueryTypeContains:
		typesQ = &types.Query{
			Wildcard: map[string]types.WildcardQuery{
				q.KV.Key: {
					Value:           ptr.Of(fmt.Sprintf("*%s*", q.KV.Value)),
					CaseInsensitive: ptr.Of(true), // Ignore case
				},
			},
		}
	case es.QueryTypeIn:
		typesQ = &types.Query{
			Terms: &types.TermsQuery{
				TermsQuery: map[string]types.TermsQueryField{
					q.KV.Key: q.KV.Value,
				},
			},
		}
	case es.QueryTypePrefix:
		typesQ = &types.Query{
			Prefix: map[string]types.PrefixQuery{
				q.KV.Key: {Value: fmt.Sprint(q.KV.Value)},
			},
		}
	default:
		typesQ = &types.Query{}
	}

	if q.Bool == nil {
		return typesQ
	}

	typesQ.Bool = &types.BoolQuery{}
	for idx := range q.Bool.Filter {
		v := q.Bool.Filter[idx]
		typesQ.Bool.Filter = append(typesQ.Bool.Filter, *c.query2ESQuery(&v))
	}

	for idx := range q.Bool.Must {
		v := q.Bool.Must[idx]
		typesQ.Bool.Must = append(typesQ.Bool.Must, *c.query2ESQuery(&v))
	}

	for idx := range q.Bool.MustNot {
		v := q.Bool.MustNot[idx]
		typesQ.Bool.MustNot = append(typesQ.Bool.MustNot, *c.query2ESQuery(&v))
	}

	for idx := range q.Bool.Should {
		v := q.Bool.Should[idx]
		typesQ.Bool.Should = append(typesQ.Bool.Should, *c.query2ESQuery(&v))
	}

	if q.Bool.MinimumShouldMatch != nil {
		typesQ.Bool.MinimumShouldMatch = q.Bool.MinimumShouldMatch
	}

	return typesQ
}

func (c *es8Client) Search(ctx context.Context, index string, req *Request) (*Response, error) {
	esReq := &search.Request{
		Query:    c.query2ESQuery(req.Query),
		Size:     req.Size,
		MinScore: (*types.Float64)(req.MinScore),
	}

	for _, sort := range req.Sort {
		order := sortorder.Asc
		if !sort.Asc {
			order = sortorder.Desc
		}
		esReq.Sort = append(esReq.Sort, types.SortCombinations(types.SortOptions{
			SortOptions: map[string]types.FieldSort{
				sort.Field: {
					Order: ptr.Of(order),
				},
			},
		}))
	}

	if req.From != nil {
		esReq.From = req.From
	} else {
		for _, v := range req.SearchAfter {
			esReq.SearchAfter = append(esReq.SearchAfter, types.FieldValue(v))
		}
	}

	logs.CtxDebugf(ctx, "Elasticsearch Request: %s\n", conv.DebugJsonToStr(esReq))

	resp, err := c.esClient.Search().Request(esReq).Index(index).Do(ctx)
	if err != nil {
		return nil, err
	}

	respJson, err := sonic.MarshalString(resp)
	if err != nil {
		return nil, err
	}

	var esResp Response
	if err := sonic.UnmarshalString(respJson, &esResp); err != nil {
		return nil, err
	}

	return &esResp, nil
}

func (c *es8Client) CreateIndex(ctx context.Context, index string, properties map[string]any) error {
	propertiesMap := make(map[string]types.Property)
	for k, v := range properties {
		propertiesMap[k] = v
	}

	if _, err := create.NewCreateFunc(c.esClient)(index).Request(&create.Request{
		Mappings: &types.TypeMapping{
			Properties: propertiesMap,
		},
	}).Do(ctx); err != nil {
		return err
	}
	return nil
}

func (c *es8Client) DeleteIndex(ctx context.Context, index string) error {
	_, err := delete.NewDeleteFunc(c.esClient)(index).
		IgnoreUnavailable(true).Do(ctx)
	return err
}

func (c *es8Client) NewBulkIndexer(index string) (BulkIndexer, error) {
	bi, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Client: c.esClient,
		Index:  index,
	})
	if err != nil {
		return nil, err
	}

	return &es8BulkIndexer{bi}, nil
}

func (c *es8Client) Types() Types {
	return c.types
}

func (t *es8Types) NewLongNumberProperty() any {
	return types.NewLongNumberProperty()
}

func (t *es8Types) NewTextProperty() any {
	return types.NewTextProperty()
}

func (t *es8Types) NewUnsignedLongNumberProperty() any {
	return types.NewUnsignedLongNumberProperty()
}

func (b *es8BulkIndexer) Add(ctx context.Context, item BulkIndexerItem) error {
	return b.bi.Add(ctx, esutil.BulkIndexerItem{
		Index:           item.Index,
		Action:          item.Action,
		DocumentID:      item.DocumentID,
		Routing:         item.Routing,
		Version:         item.Version,
		VersionType:     item.VersionType,
		Body:            item.Body,
		RetryOnConflict: item.RetryOnConflict,
		// not support in es7
		// RequireAlias:    item.RequireAlias,
		// IfSeqNo:         item.IfSeqNo,
		// IfPrimaryTerm:   item.IfPrimaryTerm,
	})
}

func (b *es8BulkIndexer) Close(ctx context.Context) error {
	return b.bi.Close(ctx)
}
