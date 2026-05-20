package object

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var errIndexRebuildRunning = errors.New("index rebuild already running")

type IndexRebuildResult struct {
	Objects    int       `json:"objects"`
	Failed     int       `json:"failed"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
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
	validObjects := make([]ObjectMeta, 0)
	for _, bucket := range buckets {
		if err := s.rebuildBucketIndex(ctx, bucket.Name, &result, &validObjects); err != nil {
			result.FinishedAt = time.Now().UTC()
			return result, err
		}
	}
	if err := s.search.PruneExcept(validObjects); err != nil {
		result.FinishedAt = time.Now().UTC()
		result.Failed++
		s.recordIndexFailure(err, false)
		return result, fmt.Errorf("prune stale index records: %w", err)
	}
	result.FinishedAt = time.Now().UTC()
	return result, nil
}

func (s *Service) rebuildBucketIndex(
	ctx context.Context,
	bucket string,
	result *IndexRebuildResult,
	validObjects *[]ObjectMeta,
) error {
	objects, err := s.ListObjects(ctx, bucket, "")
	if err != nil {
		return fmt.Errorf("list objects for index rebuild: %w", err)
	}
	*validObjects = append(*validObjects, objects...)
	for index := range objects {
		meta := objects[index]
		if err := s.indexObject(ctx, meta.Bucket, meta.Key); err != nil {
			result.Failed++
			s.recordIndexFailure(err, false)
			s.logger.WarnContext(ctx, "rebuild object index failed", "bucket", meta.Bucket, "key", meta.Key, "error", err)
			continue
		}
		result.Objects++
		s.recordIndexSuccess()
	}
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
