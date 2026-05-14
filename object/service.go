package object

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/arcgolabs/eventx"
	"github.com/arcgolabs/mapper"
	"github.com/lyonbrown4d/maxio/internal/index"
	"github.com/lyonbrown4d/maxio/internal/model"
	"github.com/lyonbrown4d/maxio/internal/store"
)

var (
	ErrNotFound       = store.ErrNotFound
	ErrBucketExists   = store.ErrBucketExists
	ErrBucketNotFound = store.ErrBucketNotFound
	ErrBadRequest     = store.ErrBadRequest
	ErrEngineFailed   = store.ErrEngineFailed
)

type Bucket = model.Bucket
type ObjectMeta = model.ObjectMeta
type SearchQuery = model.SearchQuery
type SearchResult = model.SearchResult

type PutOptions struct {
	ContentType string
}

type Service struct {
	logger *slog.Logger
	store  *store.Store
	search *index.SearchEngine
	bus    eventx.BusRuntime
}

func NewService(
	storage *store.Store,
	search *index.SearchEngine,
	bus eventx.BusRuntime,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		logger: logger,
		store:  storage,
		search: search,
		bus:    bus,
	}
}

func (s *Service) ListBuckets(ctx context.Context) ([]Bucket, error) {
	return s.store.ListBuckets(ctx)
}

func (s *Service) CreateBucket(ctx context.Context, name string) error {
	return s.store.CreateBucket(ctx, name)
}

func (s *Service) DeleteBucket(ctx context.Context, name string) error {
	return s.store.DeleteBucket(ctx, name)
}

func (s *Service) ListObjects(ctx context.Context, bucket string, prefix string) ([]ObjectMeta, error) {
	return s.store.ListObjects(ctx, bucket, prefix)
}

func (s *Service) PutObject(ctx context.Context, bucket, key string, reader io.Reader, opts PutOptions) (ObjectMeta, error) {
	meta, err := s.store.PutObject(ctx, bucket, key, reader, opts.ContentType)
	if err != nil {
		return ObjectMeta{}, err
	}
	if err := s.publishObjectEvent(ctx, "object.updated", meta); err != nil {
		s.logger.WarnContext(ctx, "publish object event failed", "event", "object.updated", "error", err)
	}
	s.search.Upsert(meta)
	return meta, nil
}

func (s *Service) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, ObjectMeta, error) {
	return s.store.GetObject(ctx, bucket, key)
}

func (s *Service) StatObject(ctx context.Context, bucket, key string) (ObjectMeta, error) {
	body, meta, err := s.store.GetObject(ctx, bucket, key)
	if err != nil {
		return ObjectMeta{}, err
	}
	if body != nil {
		if closeErr := body.Close(); closeErr != nil {
			s.logger.WarnContext(ctx, "close object body failed", "error", closeErr)
		}
	}
	return meta, nil
}

func (s *Service) DeleteObject(ctx context.Context, bucket, key string) (ObjectMeta, error) {
	meta, err := s.store.DeleteObject(ctx, bucket, key)
	if err != nil {
		return ObjectMeta{}, err
	}
	if err := s.publishObjectEvent(ctx, "object.deleted", meta); err != nil {
		s.logger.WarnContext(ctx, "publish object event failed", "event", "object.deleted", "error", err)
	}
	s.search.Remove(bucket, key)
	return meta, nil
}

func (s *Service) Search(ctx context.Context, query SearchQuery) (SearchResult, error) {
	_ = ctx
	return s.search.Search(query), nil
}

func (s *Service) publishObjectEvent(ctx context.Context, eventType string, meta ObjectMeta) error {
	if s.bus == nil {
		return nil
	}
	payload, err := mapper.Map[map[string]any](meta, mapper.WithFallbackTags("json"))
	if err != nil {
		s.logger.WarnContext(ctx, "object event mapping failed", "event", eventType, "error", err)
		return err
	}
	if err := s.bus.Publish(ctx, ObjectEvent{
		Event:   eventType,
		Payload: payload,
	}); err != nil {
		return fmt.Errorf("publish object event: %w", err)
	}
	return nil
}

type ObjectEvent struct {
	Event   string
	Payload map[string]any
}

func (e ObjectEvent) Name() string {
	return e.Event
}
