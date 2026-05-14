package metadata

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/storx/badgerx"
	"github.com/arcgolabs/storx/codec"
	"github.com/arcgolabs/storx/keycodec"
	"github.com/dgraph-io/badger/v4"
	"github.com/lyonbrown4d/maxio/internal/model"
)

type BadgerMetadata struct {
	db       *badgerx.DB
	buckets  *badgerx.Namespace[string, model.Bucket]
	objects  *badgerx.Namespace[string, model.ObjectMeta]
	blobRefs *badgerx.Namespace[string, BlobRef]
}

type BadgerMetadataOption func(*badger.Options)

func WithBadgerOptions(optionFns ...func(*badger.Options)) BadgerMetadataOption {
	return func(opts *badger.Options) {
		for _, optionFn := range optionFns {
			optionFn(opts)
		}
	}
}

func NewBadgerMetadata(path string, options ...BadgerMetadataOption) (*BadgerMetadata, error) {
	badgerOptions := badger.DefaultOptions(filepath.Clean(path)).WithLogger(nil)
	for _, option := range options {
		option(&badgerOptions)
	}

	db, err := badgerx.Open(badgerOptions)
	if err != nil {
		return nil, err
	}

	return &BadgerMetadata{
		db:       db,
		buckets:  badgerx.NewNamespaceWithDB(db, "bucket", keycodec.String(), codec.JSON[model.Bucket]()),
		objects:  badgerx.NewNamespaceWithDB(db, "object", keycodec.String(), codec.JSON[model.ObjectMeta]()),
		blobRefs: badgerx.NewNamespaceWithDB(db, "blob", keycodec.String(), codec.JSON[BlobRef]()),
	}, nil
}

func (m *BadgerMetadata) Close() error {
	return m.db.Close()
}

func (m *BadgerMetadata) ListBuckets(ctx context.Context) ([]model.Bucket, error) {
	entries, err := m.buckets.List(ctx)
	if err != nil {
		return nil, err
	}
	buckets := list.NewListWithCapacity[model.Bucket](len(entries))
	for _, entry := range entries {
		buckets.Add(entry.Value)
	}
	sorted := buckets.Sort(func(left, right model.Bucket) int {
		if left.Name < right.Name {
			return -1
		}
		if left.Name > right.Name {
			return 1
		}
		return 0
	})
	return sorted.Values(), nil
}

func (m *BadgerMetadata) BucketExists(ctx context.Context, bucket string) (bool, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return false, ErrBadRequest
	}
	_, exists, err := m.buckets.Get(ctx, bucket)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (m *BadgerMetadata) CreateBucket(ctx context.Context, bucket string) error {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return ErrBadRequest
	}
	_, exists, err := m.buckets.Get(ctx, bucket)
	if err != nil {
		return err
	}
	if exists {
		return ErrBucketExists
	}
	return m.buckets.Set(ctx, bucket, model.Bucket{
		Name:      bucket,
		CreatedAt: time.Now().UTC(),
	})
}

func (m *BadgerMetadata) DeleteBucket(ctx context.Context, bucket string) error {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return ErrBadRequest
	}
	_, exists, err := m.buckets.Get(ctx, bucket)
	if err != nil {
		return err
	}
	if !exists {
		return ErrBucketNotFound
	}

	objects, err := m.objects.List(ctx, badgerx.WithPrefix[string]([]byte(objectPrefix(bucket))))
	if err != nil {
		return err
	}
	for _, object := range objects {
		if _, _, err := m.decreaseBlobRef(ctx, object.Value.Hash); err != nil {
			if !errors.Is(err, ErrObjectNotFound) {
				return err
			}
		}
		if err := m.objects.Delete(ctx, object.Key); err != nil {
			return err
		}
	}
	if err := m.buckets.Delete(ctx, bucket); err != nil {
		return err
	}
	return nil
}

func (m *BadgerMetadata) ListObjectMetas(ctx context.Context, bucket string, prefix string) ([]model.ObjectMeta, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, ErrBadRequest
	}
	_, ok, err := m.buckets.Get(ctx, bucket)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrBucketNotFound
	}

	objects, err := m.objects.List(ctx, badgerx.WithPrefix[string]([]byte(objectPrefix(bucket))))
	if err != nil {
		return nil, err
	}
	filtered := list.NewListWithCapacity[model.ObjectMeta](len(objects))
	for _, object := range objects {
		if strings.HasPrefix(object.Value.Key, prefix) {
			filtered.Add(object.Value)
		}
	}
	filtered = filtered.Sort(func(left, right model.ObjectMeta) int {
		if left.Key < right.Key {
			return -1
		}
		if left.Key > right.Key {
			return 1
		}
		return 0
	})
	return filtered.Values(), nil
}

func (m *BadgerMetadata) GetObjectMeta(ctx context.Context, bucket string, key string) (model.ObjectMeta, bool, error) {
	meta, ok, err := m.objects.Get(ctx, objectKey(bucket, key))
	if err != nil {
		return model.ObjectMeta{}, false, err
	}
	return meta, ok, nil
}

func (m *BadgerMetadata) UpsertObjectMeta(ctx context.Context, meta model.ObjectMeta) error {
	meta.Bucket = strings.TrimSpace(meta.Bucket)
	meta.Key = strings.TrimSpace(meta.Key)
	if meta.Bucket == "" || meta.Key == "" {
		return ErrBadRequest
	}
	if _, exists, err := m.buckets.Get(ctx, meta.Bucket); err != nil {
		return err
	} else if !exists {
		return ErrBucketNotFound
	}
	return m.objects.Set(ctx, objectKey(meta.Bucket, meta.Key), meta)
}

func (m *BadgerMetadata) DeleteObjectMeta(ctx context.Context, bucket string, key string) (model.ObjectMeta, bool, error) {
	objectKey := objectKey(bucket, key)
	meta, ok, err := m.objects.Get(ctx, objectKey)
	if err != nil {
		return model.ObjectMeta{}, false, err
	}
	if !ok {
		return model.ObjectMeta{}, false, nil
	}
	if err := m.objects.Delete(ctx, objectKey); err != nil {
		return model.ObjectMeta{}, false, err
	}
	return meta, true, nil
}

func (m *BadgerMetadata) GetBlobRef(ctx context.Context, hash string) (BlobRef, bool, error) {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return BlobRef{}, false, nil
	}
	ref, ok, err := m.blobRefs.Get(ctx, hash)
	if err != nil {
		return BlobRef{}, false, err
	}
	return ref, ok, nil
}

func (m *BadgerMetadata) CreateBlobRef(ctx context.Context, hash string, path string, size int64) error {
	hash = strings.TrimSpace(hash)
	if hash == "" || strings.TrimSpace(path) == "" {
		return ErrBadRequest
	}
	_, ok, err := m.blobRefs.Get(ctx, hash)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return m.blobRefs.Set(ctx, hash, BlobRef{
		Path:     path,
		RefCount: 1,
		Size:     size,
	})
}

func (m *BadgerMetadata) IncreaseBlobRef(ctx context.Context, hash string) error {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return ErrBadRequest
	}
	ref, ok, err := m.blobRefs.Get(ctx, hash)
	if err != nil {
		return err
	}
	if !ok {
		return ErrObjectNotFound
	}
	ref.RefCount++
	return m.blobRefs.Set(ctx, hash, ref)
}

func (m *BadgerMetadata) DecreaseBlobRef(ctx context.Context, hash string) (string, bool, error) {
	return m.decreaseBlobRef(ctx, hash)
}

func (m *BadgerMetadata) decreaseBlobRef(ctx context.Context, hash string) (string, bool, error) {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return "", false, ErrBadRequest
	}

	ref, ok, err := m.blobRefs.Get(ctx, hash)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, ErrObjectNotFound
	}
	ref.RefCount--
	if ref.RefCount <= 0 {
		if err := m.blobRefs.Delete(ctx, hash); err != nil {
			return "", false, err
		}
		return ref.Path, true, nil
	}
	if err := m.blobRefs.Set(ctx, hash, ref); err != nil {
		return "", false, err
	}
	return ref.Path, false, nil
}

func objectKey(bucket string, key string) string {
	bucket = strings.TrimSpace(bucket)
	key = strings.TrimSpace(key)
	return objectPrefix(bucket) + key
}

func objectPrefix(bucket string) string {
	return strings.TrimSpace(bucket) + "\x00"
}
