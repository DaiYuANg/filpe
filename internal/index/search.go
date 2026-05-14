package index

import (
	"log/slog"
	"strings"
	"sync"

	"github.com/arcgolabs/collectionx/list"
	"github.com/blevesearch/bleve/v2"
	qry "github.com/blevesearch/bleve/v2/search/query"
	"github.com/lyonbrown4d/maxio/internal/model"
)

type SearchEngine struct {
	logger *slog.Logger
	index  bleve.Index
	ready  bool
	mu     sync.RWMutex
	items  map[string]model.ObjectMeta
}

func NewSearchEngine() *SearchEngine {
	mapping := bleve.NewIndexMapping()
	idx, err := bleve.NewMemOnly(mapping)
	if err != nil {
		slog.Default().Error("search index init failed", "error", err)
		return &SearchEngine{
			logger: slog.Default(),
			items:  make(map[string]model.ObjectMeta),
		}
	}
	return &SearchEngine{
		logger: slog.Default(),
		index:  idx,
		ready:  true,
		items:  make(map[string]model.ObjectMeta),
	}
}

func (s *SearchEngine) Upsert(meta model.ObjectMeta) {
	id := objectID(meta.Bucket, meta.Key)
	if s.ready {
		doc := map[string]any{
			"bucket":        strings.ToLower(meta.Bucket),
			"key":           meta.Key,
			"name_contains": meta.Key,
			"size":          float64(meta.Size),
			"etag":          meta.ETag,
			"content_type":  meta.ContentType,
		}
		if err := s.index.Index(id, doc); err != nil {
			s.logger.Warn("upsert search index failed", "error", err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[id] = meta
}

func (s *SearchEngine) Remove(bucket, key string) {
	id := objectID(bucket, key)
	if s.ready {
		if err := s.index.Delete(id); err != nil {
			s.logger.Warn("remove search index failed", "error", err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
}

func (s *SearchEngine) Search(query model.SearchQuery) model.SearchResult {
	if !s.ready {
		return s.searchFromMemory(query)
	}

	req := bleve.NewSearchRequest(s.buildQuery(query))
	if query.Limit > 0 {
		req.Size = query.Limit
	}
	req.Fields = []string{"*"}

	result, err := s.index.Search(req)
	if err != nil {
		s.logger.Warn("search index query failed", "error", err)
		return s.searchFromMemory(query)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	items := list.NewListWithCapacity[model.ObjectMeta](len(result.Hits))
	for _, hit := range result.Hits {
		meta, ok := s.items[hit.ID]
		if !ok {
			continue
		}
		items.Add(meta)
	}

	sorted := items.Sort(func(left, right model.ObjectMeta) int {
		switch {
		case left.UpdatedAt.After(right.UpdatedAt):
			return -1
		case left.UpdatedAt.Before(right.UpdatedAt):
			return 1
		default:
			return 0
		}
	})
	resultItems := sorted.Values()
	if query.Limit > 0 && len(resultItems) > query.Limit {
		resultItems = resultItems[:query.Limit]
	}
	return model.SearchResult{Items: resultItems}
}

func (s *SearchEngine) searchFromMemory(query model.SearchQuery) model.SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := list.NewListWithCapacity[model.ObjectMeta](len(s.items))
	for _, meta := range s.items {
		items.Add(meta)
	}

	matched := items.Where(func(_ int, meta model.ObjectMeta) bool {
		return s.matchQuery(meta, query)
	}).Sort(func(left, right model.ObjectMeta) int {
		switch {
		case left.UpdatedAt.After(right.UpdatedAt):
			return -1
		case left.UpdatedAt.Before(right.UpdatedAt):
			return 1
		default:
			return 0
		}
	})
	resultItems := matched.Values()
	if query.Limit > 0 && len(resultItems) > query.Limit {
		resultItems = resultItems[:query.Limit]
	}
	return model.SearchResult{Items: resultItems}
}

func (s *SearchEngine) matchQuery(meta model.ObjectMeta, query model.SearchQuery) bool {
	return (query.Bucket == "" || meta.Bucket == query.Bucket) &&
		(query.Prefix == "" || strings.HasPrefix(meta.Key, query.Prefix)) &&
		(query.NameContains == "" || strings.Contains(meta.Key, query.NameContains)) &&
		(query.MinSize <= 0 || meta.Size >= query.MinSize) &&
		(query.MaxSize <= 0 || meta.Size <= query.MaxSize)
}

func (s *SearchEngine) buildQuery(criteria model.SearchQuery) qry.Query {
	queries := make([]qry.Query, 0, 4)

	if criteria.Bucket != "" {
		q := bleve.NewMatchQuery(strings.ToLower(criteria.Bucket))
		q.SetField("bucket")
		queries = append(queries, q)
	}
	if criteria.Prefix != "" {
		q := bleve.NewPrefixQuery(criteria.Prefix)
		q.SetField("key")
		queries = append(queries, q)
	}
	if criteria.NameContains != "" {
		q := bleve.NewMatchQuery(criteria.NameContains)
		q.SetField("name_contains")
		queries = append(queries, q)
	}

	if criteria.MinSize > 0 || criteria.MaxSize > 0 {
		var minSize, maxSize *float64
		if criteria.MinSize > 0 {
			minSizeValue := float64(criteria.MinSize)
			minSize = &minSizeValue
		}
		if criteria.MaxSize > 0 {
			maxSizeValue := float64(criteria.MaxSize)
			maxSize = &maxSizeValue
		}
		size := bleve.NewNumericRangeQuery(minSize, maxSize)
		size.SetField("size")
		queries = append(queries, size)
	}

	if len(queries) == 0 {
		return bleve.NewMatchAllQuery()
	}
	if len(queries) == 1 {
		return queries[0]
	}
	return bleve.NewConjunctionQuery(queries...)
}

func objectID(bucket, key string) string {
	return bucket + "\x00" + key
}
