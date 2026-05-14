package raft

import (
	"sort"
	"strings"

	"github.com/lyonbrown4d/maxio/internal/model"
)

func (s *raftStateMachine) applyMetadataCommand(cmd MetadataCommand) (MetadataResult, string) {
	switch cmd.Type {
	case MetadataCommandCreateBucket:
		return s.createBucket(cmd.Bucket, cmd.BucketMeta)
	case MetadataCommandDeleteBucket:
		return s.deleteBucket(cmd.Bucket)
	case MetadataCommandUpsertObjectMeta:
		return s.upsertObjectMeta(cmd.Meta)
	case MetadataCommandDeleteObjectMeta:
		return s.deleteObjectMeta(cmd.Bucket, cmd.Key)
	case MetadataCommandCreateBlobRef:
		return s.createBlobRef(cmd.Hash, cmd.Path, cmd.Size)
	case MetadataCommandIncreaseBlobRef:
		return s.increaseBlobRef(cmd.Hash)
	case MetadataCommandDecreaseBlobRef:
		return s.decreaseBlobRef(cmd.Hash)
	default:
		return MetadataResult{}, MetadataErrorBadRequest
	}
}

func (s *raftStateMachine) lookupMetadataQuery(query MetadataQuery) metadataEnvelope {
	switch query.Type {
	case MetadataQueryListBuckets:
		return metadataEnvelope{Result: MetadataResult{Buckets: s.listBuckets()}}
	case MetadataQueryBucketExists:
		return s.lookupBucketExists(query.Bucket)
	case MetadataQueryListObjectMetas:
		return s.lookupListObjectMetas(query.Bucket, query.Prefix)
	case MetadataQueryGetObjectMeta:
		return s.lookupObjectMeta(query.Bucket, query.Key)
	case MetadataQueryGetBlobRef:
		return s.lookupBlobRef(query.Hash)
	default:
		return metadataEnvelope{Error: MetadataErrorBadRequest}
	}
}

func (s *raftStateMachine) lookupBucketExists(bucket string) metadataEnvelope {
	if invalidName(bucket) {
		return metadataEnvelope{Error: MetadataErrorBadRequest}
	}
	_, ok := s.buckets[bucket]
	return metadataEnvelope{Result: MetadataResult{BucketExists: ok}}
}

func (s *raftStateMachine) lookupListObjectMetas(bucket, prefix string) metadataEnvelope {
	if _, ok := s.buckets[bucket]; !ok {
		return metadataEnvelope{Error: MetadataErrorBucketNotFound}
	}
	return metadataEnvelope{Result: MetadataResult{Objects: s.listObjectMetas(bucket, prefix)}}
}

func (s *raftStateMachine) lookupObjectMeta(bucket, key string) metadataEnvelope {
	if invalidObject(bucket, key) {
		return metadataEnvelope{Error: MetadataErrorBadRequest}
	}
	meta, ok := s.objects[objectMapKey(bucket, key)]
	return metadataEnvelope{Result: MetadataResult{Object: meta, ObjectExists: ok}}
}

func (s *raftStateMachine) lookupBlobRef(hash string) metadataEnvelope {
	if invalidName(hash) {
		return metadataEnvelope{Error: MetadataErrorBadRequest}
	}
	ref, ok := s.blobRefs[hash]
	return metadataEnvelope{Result: MetadataResult{Blob: ref, BlobExists: ok}}
}

func (s *raftStateMachine) createBucket(bucket string, bucketMeta model.Bucket) (MetadataResult, string) {
	if bucketMeta.Name != "" {
		bucket = bucketMeta.Name
	}
	if invalidName(bucket) {
		return MetadataResult{}, MetadataErrorBadRequest
	}
	if _, ok := s.buckets[bucket]; ok {
		return MetadataResult{}, MetadataErrorBucketExists
	}
	if bucketMeta.Name == "" {
		bucketMeta.Name = bucket
	}
	s.buckets[bucket] = bucketMeta
	return MetadataResult{}, ""
}

func (s *raftStateMachine) deleteBucket(bucket string) (MetadataResult, string) {
	if invalidName(bucket) {
		return MetadataResult{}, MetadataErrorBadRequest
	}
	if _, ok := s.buckets[bucket]; !ok {
		return MetadataResult{}, MetadataErrorBucketNotFound
	}
	delete(s.buckets, bucket)
	for key, meta := range s.objects {
		if meta.Bucket == bucket {
			delete(s.objects, key)
		}
	}
	return MetadataResult{}, ""
}

func (s *raftStateMachine) upsertObjectMeta(meta model.ObjectMeta) (MetadataResult, string) {
	if invalidObject(meta.Bucket, meta.Key) || invalidName(meta.Hash) {
		return MetadataResult{}, MetadataErrorBadRequest
	}
	if _, ok := s.buckets[meta.Bucket]; !ok {
		return MetadataResult{}, MetadataErrorBucketNotFound
	}
	s.objects[objectMapKey(meta.Bucket, meta.Key)] = meta
	return MetadataResult{}, ""
}

func (s *raftStateMachine) deleteObjectMeta(bucket, key string) (MetadataResult, string) {
	if invalidObject(bucket, key) {
		return MetadataResult{}, MetadataErrorBadRequest
	}
	mapKey := objectMapKey(bucket, key)
	meta, ok := s.objects[mapKey]
	if !ok {
		return MetadataResult{ObjectExists: false}, ""
	}
	delete(s.objects, mapKey)
	return MetadataResult{Object: meta, ObjectExists: true}, ""
}

func (s *raftStateMachine) createBlobRef(hash, path string, size int64) (MetadataResult, string) {
	if invalidName(hash) || strings.TrimSpace(path) == "" {
		return MetadataResult{}, MetadataErrorBadRequest
	}
	if ref, ok := s.blobRefs[hash]; ok {
		ref.RefCount++
		s.blobRefs[hash] = ref
		return MetadataResult{}, ""
	}
	s.blobRefs[hash] = MetadataBlobRef{Path: path, RefCount: 1, Size: size}
	return MetadataResult{}, ""
}

func (s *raftStateMachine) increaseBlobRef(hash string) (MetadataResult, string) {
	if invalidName(hash) {
		return MetadataResult{}, MetadataErrorBadRequest
	}
	ref, ok := s.blobRefs[hash]
	if !ok {
		return MetadataResult{}, MetadataErrorObjectNotFound
	}
	ref.RefCount++
	s.blobRefs[hash] = ref
	return MetadataResult{}, ""
}

func (s *raftStateMachine) decreaseBlobRef(hash string) (MetadataResult, string) {
	if invalidName(hash) {
		return MetadataResult{}, MetadataErrorBadRequest
	}
	ref, ok := s.blobRefs[hash]
	if !ok {
		return MetadataResult{BlobRemoved: false}, ""
	}
	if ref.RefCount <= 1 {
		delete(s.blobRefs, hash)
		return MetadataResult{BlobPath: ref.Path, BlobRemoved: true}, ""
	}
	ref.RefCount--
	s.blobRefs[hash] = ref
	return MetadataResult{BlobRemoved: false}, ""
}

func (s *raftStateMachine) listBuckets() []model.Bucket {
	buckets := make([]model.Bucket, 0, len(s.buckets))
	for _, bucket := range s.buckets {
		buckets = append(buckets, bucket)
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Name < buckets[j].Name
	})
	return buckets
}

func (s *raftStateMachine) listObjectMetas(bucket, prefix string) []model.ObjectMeta {
	objects := make([]model.ObjectMeta, 0)
	for _, meta := range s.objects {
		if meta.Bucket == bucket && strings.HasPrefix(meta.Key, prefix) {
			objects = append(objects, meta)
		}
	}
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].Key < objects[j].Key
	})
	return objects
}
