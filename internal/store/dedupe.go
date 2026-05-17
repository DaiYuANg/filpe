package store

import (
	"context"
	"fmt"

	"github.com/lyonbrown4d/maxio/internal/metadata"
)

func (s *Store) PlanDedupe(ctx context.Context) (DedupeResult, error) {
	return s.Dedupe(ctx, DedupeOptions{DryRun: true})
}

func (s *Store) Dedupe(ctx context.Context, opts DedupeOptions) (DedupeResult, error) {
	result := newDedupeResult(opts)
	objects, stats, err := s.collectDedupeObjects(ctx, &result)
	if err != nil {
		return result, err
	}
	refs, err := s.meta.ListBlobRefs(ctx)
	if err != nil {
		return result, fmt.Errorf("list blob refs for dedupe: %w", mapStoreError(err))
	}
	result.BlobRefs = len(refs)
	refMap := dedupeRefMap(refs)
	s.planMissingBlobRefs(stats, refMap, &result)
	if err := s.fixMissingBlobRefs(ctx, opts, stats, refMap, &result); err != nil {
		return result, err
	}
	if err := s.fixExistingBlobRefs(ctx, opts, stats, refs, &result); err != nil {
		return result, err
	}
	if err := s.canonicalizeDedupeLayouts(ctx, opts, objects, refMap, &result); err != nil {
		return result, err
	}
	return result, nil
}

func dedupeRefMap(refs []metadata.BlobRef) map[string]metadata.BlobRef {
	refMap := make(map[string]metadata.BlobRef, len(refs))
	for index := range refs {
		refMap[refs[index].Hash] = refs[index]
	}
	return refMap
}
