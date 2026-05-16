package index

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/arcgolabs/collectionx/list"
	"github.com/blevesearch/bleve/v2"
	"github.com/lyonbrown4d/maxio/internal/config"
	"github.com/lyonbrown4d/maxio/internal/model"
)

const indexDir = "index/bleve"

type SearchEngine struct {
	logger *slog.Logger
	index  bleve.Index
	ready  bool
	mu     sync.RWMutex
	items  map[string]model.ObjectMeta
}

func NewSearchEngine(cfg config.Config, logger *slog.Logger) (*SearchEngine, error) {
	if logger == nil {
		logger = slog.Default()
	}
	idx, err := openPersistentIndex(cfg)
	if err != nil {
		return nil, err
	}
	return &SearchEngine{
		logger: logger,
		index:  idx,
		ready:  true,
		items:  make(map[string]model.ObjectMeta),
	}, nil
}

func NewInMemorySearchEngine() *SearchEngine {
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

func openPersistentIndex(cfg config.Config) (bleve.Index, error) {
	path := filepath.Join(cfg.DataDir, indexDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create search index parent: %w", err)
	}
	idx, err := bleve.Open(path)
	if err == nil {
		return idx, nil
	}
	if !errors.Is(err, bleve.ErrorIndexPathDoesNotExist) {
		return nil, fmt.Errorf("open search index: %w", err)
	}
	idx, err = bleve.New(path, bleve.NewIndexMapping())
	if err != nil {
		return nil, fmt.Errorf("create search index: %w", err)
	}
	return idx, nil
}

func (s *SearchEngine) Upsert(meta model.ObjectMeta) {
	s.UpsertDocument(meta, "")
}

func (s *SearchEngine) UpsertDocument(meta model.ObjectMeta, text string) {
	id := objectID(meta.Bucket, meta.Key)
	doc := documentFromMeta(meta, text)
	if s.ready {
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

	hits, err := s.searchIndex(query)
	if err != nil {
		s.logger.Warn("search index query failed", "error", err)
		return s.searchFromMemory(query)
	}
	return s.resultFromHits(query, hits)
}

func (s *SearchEngine) Close() error {
	if s == nil || s.index == nil {
		return nil
	}
	if err := s.index.Close(); err != nil {
		return fmt.Errorf("close search index: %w", err)
	}
	return nil
}

func (s *SearchEngine) searchIndex(query model.SearchQuery) ([]searchHit, error) {
	req := bleve.NewSearchRequest(s.buildQuery(query))
	if query.Limit > 0 {
		req.Size = query.Limit
	}
	req.Fields = []string{"bucket", "key", "hash", "etag", "size", "content_type", "updated_at"}
	result, err := s.index.Search(req)
	if err != nil {
		return nil, fmt.Errorf("search bleve index: %w", err)
	}
	hits := make([]searchHit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		hits = append(hits, searchHit{
			ID:     hit.ID,
			Fields: hit.Fields,
		})
	}
	return hits, nil
}

type searchHit struct {
	ID     string
	Fields map[string]any
}

func (s *SearchEngine) resultFromHits(query model.SearchQuery, hits []searchHit) model.SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := list.NewListWithCapacity[model.ObjectMeta](len(hits))
	for _, hit := range hits {
		meta, ok := s.items[hit.ID]
		if !ok {
			meta = metaFromFields(hit.Fields)
		}
		if meta.Bucket == "" || meta.Key == "" {
			continue
		}
		items.Add(meta)
	}

	return limitedSearchResult(query, items)
}

func (s *SearchEngine) searchFromMemory(query model.SearchQuery) model.SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := list.NewListWithCapacity[model.ObjectMeta](len(s.items))
	for key := range s.items {
		meta := s.items[key]
		if matchesQuery(meta, query) {
			items.Add(meta)
		}
	}
	return limitedSearchResult(query, items)
}
