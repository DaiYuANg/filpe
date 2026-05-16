package object

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/arcgolabs/eventx"
	"github.com/lyonbrown4d/maxio/internal/index"
)

var errIndexRebuildRunning = errors.New("index rebuild already running")

type IndexStatus struct {
	Rebuilding            bool      `json:"rebuilding"`
	IndexedObjects        int       `json:"indexed_objects"`
	FailedObjects         int       `json:"failed_objects"`
	LastIndexedAt         time.Time `json:"last_indexed_at,omitzero"`
	LastError             string    `json:"last_error,omitempty"`
	LastRebuildStartedAt  time.Time `json:"last_rebuild_started_at,omitzero"`
	LastRebuildFinishedAt time.Time `json:"last_rebuild_finished_at,omitzero"`
	LastRebuildObjects    int       `json:"last_rebuild_objects"`
	LastRebuildFailed     int       `json:"last_rebuild_failed"`
	LastRebuildError      string    `json:"last_rebuild_error,omitempty"`
}

type IndexRebuildResult struct {
	Objects    int       `json:"objects"`
	Failed     int       `json:"failed"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

func (s *Service) StartIndexWorker(_ context.Context) error {
	if s == nil || s.bus == nil || s.search == nil {
		return nil
	}
	_, err := eventx.Subscribe(s.bus, func(ctx context.Context, event ObjectEvent) error {
		workerCtx := context.WithoutCancel(ctx)
		go s.handleIndexEvent(workerCtx, event)
		return nil
	})
	if err != nil {
		return fmt.Errorf("subscribe object index worker: %w", err)
	}
	return nil
}

func (s *Service) handleIndexEvent(ctx context.Context, event ObjectEvent) {
	bucket, key := eventObjectLocation(event)
	if bucket == "" || key == "" {
		return
	}
	switch event.Event {
	case "object.updated":
		if err := s.indexObject(ctx, bucket, key); err != nil {
			s.recordIndexFailure(err)
			s.logger.WarnContext(ctx, "index object failed", "bucket", bucket, "key", key, "error", err)
			return
		}
		s.recordIndexSuccess()
	case "object.deleted":
		s.search.Remove(bucket, key)
	}
}

func (s *Service) RebuildIndex(ctx context.Context) (IndexRebuildResult, error) {
	if s == nil || s.search == nil {
		return IndexRebuildResult{}, errors.New("index service unavailable")
	}
	startedAt := time.Now().UTC()
	if !s.beginIndexRebuild(startedAt) {
		return IndexRebuildResult{}, errIndexRebuildRunning
	}

	result, err := s.rebuildIndex(ctx, startedAt)
	s.finishIndexRebuild(result, err)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (s *Service) rebuildIndex(ctx context.Context, startedAt time.Time) (IndexRebuildResult, error) {
	result := IndexRebuildResult{StartedAt: startedAt}
	buckets, err := s.ListBuckets(ctx)
	if err != nil {
		result.FinishedAt = time.Now().UTC()
		return result, fmt.Errorf("list buckets for index rebuild: %w", err)
	}
	for _, bucket := range buckets {
		if err := s.rebuildBucketIndex(ctx, bucket.Name, &result); err != nil {
			result.FinishedAt = time.Now().UTC()
			return result, err
		}
	}
	result.FinishedAt = time.Now().UTC()
	return result, nil
}

func (s *Service) rebuildBucketIndex(ctx context.Context, bucket string, result *IndexRebuildResult) error {
	objects, err := s.ListObjects(ctx, bucket, "")
	if err != nil {
		return fmt.Errorf("list objects for index rebuild: %w", err)
	}
	for index := range objects {
		meta := objects[index]
		if err := s.indexObject(ctx, meta.Bucket, meta.Key); err != nil {
			result.Failed++
			s.recordIndexFailure(err)
			s.logger.WarnContext(ctx, "rebuild object index failed", "bucket", meta.Bucket, "key", meta.Key, "error", err)
			continue
		}
		result.Objects++
		s.recordIndexSuccess()
	}
	return nil
}

func (s *Service) indexObject(ctx context.Context, bucket, key string) error {
	body, meta, err := s.GetObject(ctx, bucket, key)
	if err != nil {
		return fmt.Errorf("load object for indexing: %w", err)
	}
	defer closeIndexBody(ctx, s, body)

	text, err := index.ExtractText(body, meta)
	if err != nil {
		s.search.UpsertDocument(meta, "")
		return fmt.Errorf("extract object text: %w", err)
	}
	s.search.UpsertDocument(meta, text)
	return nil
}

func (s *Service) beginIndexRebuild(startedAt time.Time) bool {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	if s.index.Rebuilding {
		return false
	}
	s.index.Rebuilding = true
	s.index.LastRebuildStartedAt = startedAt
	s.index.LastRebuildFinishedAt = time.Time{}
	s.index.LastRebuildObjects = 0
	s.index.LastRebuildFailed = 0
	s.index.LastRebuildError = ""
	return true
}

func (s *Service) finishIndexRebuild(result IndexRebuildResult, err error) {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	s.index.Rebuilding = false
	s.index.LastRebuildFinishedAt = result.FinishedAt
	s.index.LastRebuildObjects = result.Objects
	s.index.LastRebuildFailed = result.Failed
	if err != nil {
		s.index.LastRebuildError = err.Error()
	}
}

func (s *Service) recordIndexSuccess() {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	s.index.IndexedObjects++
	s.index.LastIndexedAt = time.Now().UTC()
	s.index.LastError = ""
}

func (s *Service) recordIndexFailure(err error) {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	s.index.FailedObjects++
	s.index.LastIndexedAt = time.Now().UTC()
	if err != nil {
		s.index.LastError = err.Error()
	}
}

func eventObjectLocation(event ObjectEvent) (string, string) {
	bucket := strings.TrimSpace(payloadString(event.Payload, "bucket"))
	key := strings.TrimSpace(payloadString(event.Payload, "key"))
	return bucket, key
}

func payloadString(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func closeIndexBody(ctx context.Context, s *Service, body io.Closer) {
	if err := body.Close(); err != nil {
		s.logger.WarnContext(ctx, "close object indexing body failed", "error", err)
	}
}
