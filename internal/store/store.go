package store

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/lyonbrown4d/maxio/internal/engine"
	"github.com/lyonbrown4d/maxio/internal/metadata"
	"github.com/lyonbrown4d/maxio/internal/model"
)

var (
	ErrNotFound       = errors.New("object or bucket not found")
	ErrBucketExists   = errors.New("bucket already exists")
	ErrBucketNotFound = errors.New("bucket not found")
	ErrBadRequest     = errors.New("bad request")
	ErrEngineFailed   = errors.New("storage engine failed")
)

// Store is the unified object store: metadata (Badger) + engine (erasure-coded file storage).
type Store struct {
	meta   metadata.MetadataStore
	engine *engine.Engine
}

func NewStore(dataDir string, meta metadata.MetadataStore, e *engine.Engine) (*Store, error) {
	if meta == nil {
		meta = metadata.NewInMemoryMetadata()
	}
	if e == nil {
		var err error
		e, err = engine.NewEngine(dataDir, engine.DefaultDataChunks, engine.DefaultParityChunks, nil)
		if err != nil {
			return nil, err
		}
	}
	return &Store{
		meta:   meta,
		engine: e,
	}, nil
}

// --- Bucket operations (metadata only) ---

func (s *Store) ListBuckets(ctx context.Context) ([]model.Bucket, error) {
	return s.meta.ListBuckets(ctx)
}

func (s *Store) CreateBucket(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrBadRequest
	}
	return mapStoreError(s.meta.CreateBucket(ctx, name))
}

func (s *Store) DeleteBucket(ctx context.Context, name string) error {
	return mapStoreError(s.meta.DeleteBucket(ctx, name))
}

// --- Object operations (delegated to engine) ---

func (s *Store) ListObjects(ctx context.Context, bucket string, prefix string) ([]model.ObjectMeta, error) {
	bucket = strings.TrimSpace(bucket)
	exists, err := s.meta.BucketExists(ctx, bucket)
	if err != nil {
		return nil, mapStoreError(err)
	}
	if !exists {
		return nil, ErrBucketNotFound
	}
	objects, err := s.meta.ListObjectMetas(ctx, bucket, prefix)
	return objects, mapStoreError(err)
}

func (s *Store) PutObject(ctx context.Context, bucket, key string, reader io.Reader, contentType string) (model.ObjectMeta, error) {
	meta, err := s.engine.PutObject(ctx, bucket, key, reader, contentType)
	if err != nil {
		return model.ObjectMeta{}, ErrEngineFailed
	}
	// Also persist metadata
	err = s.meta.UpsertObjectMeta(ctx, model.ObjectMeta{
		Bucket:      meta.Bucket,
		Key:         meta.Key,
		Hash:        meta.Hash,
		ETag:        meta.ETag,
		Size:        meta.Size,
		ContentType: meta.ContentType,
		UpdatedAt:   meta.UpdatedAt,
	})
	if err != nil {
		return model.ObjectMeta{}, err
	}
	return model.ObjectMeta{
		Bucket:      meta.Bucket,
		Key:         meta.Key,
		Hash:        meta.Hash,
		ETag:        meta.ETag,
		Size:        meta.Size,
		ContentType: meta.ContentType,
		UpdatedAt:   meta.UpdatedAt,
	}, nil
}

func (s *Store) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, model.ObjectMeta, error) {
	reader, meta, err := s.engine.GetObject(ctx, bucket, key)
	if err != nil {
		return nil, model.ObjectMeta{}, mapStoreError(err)
	}
	return reader, model.ObjectMeta{
		Bucket:      meta.Bucket,
		Key:         meta.Key,
		Hash:        meta.Hash,
		ETag:        meta.ETag,
		Size:        meta.Size,
		ContentType: meta.ContentType,
		UpdatedAt:   meta.UpdatedAt,
	}, nil
}

func (s *Store) DeleteObject(ctx context.Context, bucket, key string) (model.ObjectMeta, error) {
	// Delete from engine
	if err := s.engine.DeleteObject(ctx, bucket, key); err != nil {
		return model.ObjectMeta{}, mapStoreError(err)
	}
	// Delete from metadata
	objMeta, _, err := s.meta.GetObjectMeta(ctx, bucket, key)
	if err != nil {
		return model.ObjectMeta{}, mapStoreError(err)
	}
	_, _, err = s.meta.DeleteObjectMeta(ctx, bucket, key)
	if err != nil {
		return model.ObjectMeta{}, mapStoreError(err)
	}
	return objMeta, nil
}

// CheckHealth reports shard health for an object.
func (s *Store) CheckHealth(ctx context.Context, bucket, key string) (engine.Health, error) {
	return s.engine.CheckHealth(ctx, bucket, key)
}

func mapStoreError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, metadata.ErrBucketExists) {
		return ErrBucketExists
	}
	if errors.Is(err, metadata.ErrBucketNotFound) {
		return ErrBucketNotFound
	}
	if errors.Is(err, metadata.ErrObjectNotFound) {
		return ErrNotFound
	}
	if errors.Is(err, metadata.ErrBadRequest) {
		return ErrBadRequest
	}
	if errors.Is(err, engine.ErrObjectNotFound) {
		return ErrNotFound
	}
	return err
}
