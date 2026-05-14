package raft

import (
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"sync"

	dbsm "github.com/lni/dragonboat/v4/statemachine"
	"github.com/lyonbrown4d/maxio/internal/model"
)

const (
	MetadataErrorBadRequest     = "bad_request"
	MetadataErrorBucketExists   = "bucket_exists"
	MetadataErrorBucketNotFound = "bucket_not_found"
	MetadataErrorObjectNotFound = "object_not_found"

	MetadataCommandCreateBucket     = "create_bucket"
	MetadataCommandDeleteBucket     = "delete_bucket"
	MetadataCommandUpsertObjectMeta = "upsert_object_meta"
	MetadataCommandDeleteObjectMeta = "delete_object_meta"
	MetadataCommandCreateBlobRef    = "create_blob_ref"
	MetadataCommandIncreaseBlobRef  = "increase_blob_ref"
	MetadataCommandDecreaseBlobRef  = "decrease_blob_ref"

	MetadataQueryListBuckets     = "list_buckets"
	MetadataQueryBucketExists    = "bucket_exists"
	MetadataQueryListObjectMetas = "list_object_metas"
	MetadataQueryGetObjectMeta   = "get_object_meta"
	MetadataQueryGetBlobRef      = "get_blob_ref"
)

type MetadataBlobRef struct {
	Path     string `json:"path"`
	RefCount int    `json:"ref_count"`
	Size     int64  `json:"size"`
}

type MetadataCommand struct {
	Type       string           `json:"type"`
	Bucket     string           `json:"bucket,omitempty"`
	Key        string           `json:"key,omitempty"`
	BucketMeta model.Bucket     `json:"bucket_meta,omitempty"`
	Meta       model.ObjectMeta `json:"meta,omitempty"`
	Hash       string           `json:"hash,omitempty"`
	Path       string           `json:"path,omitempty"`
	Size       int64            `json:"size,omitempty"`
}

type MetadataQuery struct {
	Type   string `json:"type"`
	Bucket string `json:"bucket,omitempty"`
	Key    string `json:"key,omitempty"`
	Prefix string `json:"prefix,omitempty"`
	Hash   string `json:"hash,omitempty"`
}

type MetadataResult struct {
	Buckets      []model.Bucket     `json:"buckets,omitempty"`
	BucketExists bool               `json:"bucket_exists,omitempty"`
	Objects      []model.ObjectMeta `json:"objects,omitempty"`
	Object       model.ObjectMeta   `json:"object,omitempty"`
	ObjectExists bool               `json:"object_exists,omitempty"`
	Blob         MetadataBlobRef    `json:"blob,omitempty"`
	BlobExists   bool               `json:"blob_exists,omitempty"`
	BlobPath     string             `json:"blob_path,omitempty"`
	BlobRemoved  bool               `json:"blob_removed,omitempty"`
}

type metadataEnvelope struct {
	Result MetadataResult `json:"result"`
	Error  string         `json:"error,omitempty"`
}

type metadataSnapshot struct {
	Buckets  map[string]model.Bucket     `json:"buckets"`
	Objects  map[string]model.ObjectMeta `json:"objects"`
	BlobRefs map[string]MetadataBlobRef  `json:"blob_refs"`
}

type raftStateMachine struct {
	shardID   uint64
	replicaID uint64

	mu       sync.RWMutex
	closed   bool
	buckets  map[string]model.Bucket
	objects  map[string]model.ObjectMeta
	blobRefs map[string]MetadataBlobRef
}

func newRaftStateMachine(shardID uint64, replicaID uint64) *raftStateMachine {
	return &raftStateMachine{
		shardID:   shardID,
		replicaID: replicaID,
		buckets:   make(map[string]model.Bucket),
		objects:   make(map[string]model.ObjectMeta),
		blobRefs:  make(map[string]MetadataBlobRef),
	}
}

func (s *raftStateMachine) Lookup(query any) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return metadataEnvelope{Error: MetadataErrorBadRequest}, nil
	}

	switch typed := query.(type) {
	case MetadataQuery:
		return s.lookupMetadataQuery(typed), nil
	case *MetadataQuery:
		if typed == nil {
			return metadataEnvelope{Error: MetadataErrorBadRequest}, nil
		}
		return s.lookupMetadataQuery(*typed), nil
	case string:
		switch typed {
		case "health":
			return metadataEnvelope{}, nil
		case "state":
			return metadataEnvelope{Result: MetadataResult{BucketExists: true}}, nil
		default:
			return metadataEnvelope{Error: MetadataErrorBadRequest}, nil
		}
	default:
		return metadataEnvelope{Error: MetadataErrorBadRequest}, nil
	}
}

func (s *raftStateMachine) Update(entry dbsm.Entry) (dbsm.Result, error) {
	var cmd MetadataCommand
	if err := json.Unmarshal(entry.Cmd, &cmd); err != nil {
		return encodeMetadataEnvelope(metadataEnvelope{Error: MetadataErrorBadRequest})
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return encodeMetadataEnvelope(metadataEnvelope{Error: MetadataErrorBadRequest})
	}

	result, code := s.applyMetadataCommand(cmd)
	return encodeMetadataEnvelope(metadataEnvelope{Result: result, Error: code})
}

func (s *raftStateMachine) SaveSnapshot(w io.Writer, _ dbsm.ISnapshotFileCollection, _ <-chan struct{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return json.NewEncoder(w).Encode(metadataSnapshot{
		Buckets:  copyBucketMap(s.buckets),
		Objects:  copyObjectMap(s.objects),
		BlobRefs: copyBlobRefMap(s.blobRefs),
	})
}

func (s *raftStateMachine) RecoverFromSnapshot(r io.Reader, _ []dbsm.SnapshotFile, _ <-chan struct{}) error {
	var snapshot metadataSnapshot
	if err := json.NewDecoder(r).Decode(&snapshot); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.buckets = snapshot.Buckets
	if s.buckets == nil {
		s.buckets = make(map[string]model.Bucket)
	}
	s.objects = snapshot.Objects
	if s.objects == nil {
		s.objects = make(map[string]model.ObjectMeta)
	}
	s.blobRefs = snapshot.BlobRefs
	if s.blobRefs == nil {
		s.blobRefs = make(map[string]MetadataBlobRef)
	}
	return nil
}

func (s *raftStateMachine) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	return nil
}

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
		if invalidName(query.Bucket) {
			return metadataEnvelope{Error: MetadataErrorBadRequest}
		}
		_, ok := s.buckets[query.Bucket]
		return metadataEnvelope{Result: MetadataResult{BucketExists: ok}}
	case MetadataQueryListObjectMetas:
		if _, ok := s.buckets[query.Bucket]; !ok {
			return metadataEnvelope{Error: MetadataErrorBucketNotFound}
		}
		return metadataEnvelope{Result: MetadataResult{Objects: s.listObjectMetas(query.Bucket, query.Prefix)}}
	case MetadataQueryGetObjectMeta:
		if invalidObject(query.Bucket, query.Key) {
			return metadataEnvelope{Error: MetadataErrorBadRequest}
		}
		meta, ok := s.objects[objectMapKey(query.Bucket, query.Key)]
		return metadataEnvelope{Result: MetadataResult{Object: meta, ObjectExists: ok}}
	case MetadataQueryGetBlobRef:
		if invalidName(query.Hash) {
			return metadataEnvelope{Error: MetadataErrorBadRequest}
		}
		ref, ok := s.blobRefs[query.Hash]
		return metadataEnvelope{Result: MetadataResult{Blob: ref, BlobExists: ok}}
	default:
		return metadataEnvelope{Error: MetadataErrorBadRequest}
	}
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

func (s *raftStateMachine) deleteObjectMeta(bucket string, key string) (MetadataResult, string) {
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

func (s *raftStateMachine) createBlobRef(hash string, path string, size int64) (MetadataResult, string) {
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

func (s *raftStateMachine) listObjectMetas(bucket string, prefix string) []model.ObjectMeta {
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

func encodeMetadataEnvelope(envelope metadataEnvelope) (dbsm.Result, error) {
	data, err := json.Marshal(envelope)
	if err != nil {
		return dbsm.Result{}, err
	}
	return dbsm.Result{Data: data}, nil
}

func decodeMetadataEnvelope(data []byte) (MetadataResult, error) {
	if len(data) == 0 {
		return MetadataResult{}, nil
	}
	var envelope metadataEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return MetadataResult{}, err
	}
	if envelope.Error != "" {
		return envelope.Result, metadataError(envelope.Error)
	}
	return envelope.Result, nil
}

func resultFromMetadataEnvelope(value any) (MetadataResult, error) {
	switch typed := value.(type) {
	case MetadataResult:
		return typed, nil
	case metadataEnvelope:
		if typed.Error != "" {
			return typed.Result, metadataError(typed.Error)
		}
		return typed.Result, nil
	case *metadataEnvelope:
		if typed == nil {
			return MetadataResult{}, metadataError(MetadataErrorBadRequest)
		}
		if typed.Error != "" {
			return typed.Result, metadataError(typed.Error)
		}
		return typed.Result, nil
	default:
		return MetadataResult{}, metadataError(MetadataErrorBadRequest)
	}
}

type metadataError string

func (e metadataError) Error() string {
	return string(e)
}

func MetadataErrorCode(err error) string {
	var code metadataError
	if errors.As(err, &code) {
		return string(code)
	}
	return ""
}

func invalidName(value string) bool {
	return strings.TrimSpace(value) == ""
}

func invalidObject(bucket string, key string) bool {
	return invalidName(bucket) || strings.TrimSpace(key) == ""
}

func objectMapKey(bucket string, key string) string {
	return bucket + "\x00" + key
}

func copyBucketMap(input map[string]model.Bucket) map[string]model.Bucket {
	output := make(map[string]model.Bucket, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func copyObjectMap(input map[string]model.ObjectMeta) map[string]model.ObjectMeta {
	output := make(map[string]model.ObjectMeta, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func copyBlobRefMap(input map[string]MetadataBlobRef) map[string]MetadataBlobRef {
	output := make(map[string]MetadataBlobRef, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
