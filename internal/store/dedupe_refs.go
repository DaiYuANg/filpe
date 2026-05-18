package store

import (
	"context"
	"fmt"

	"github.com/lyonbrown4d/maxio/internal/metadata"
)

func (s *Store) planMissingBlobRefs(
	stats map[string]dedupeHashStat,
	refs map[string]metadata.BlobRef,
	result *DedupeResult,
) {
	for hash := range stats {
		if _, ok := refs[hash]; ok {
			continue
		}
		stat := stats[hash]
		result.MissingBlobRefs++
		result.addIssue(DedupeIssue{
			Kind:             DedupeIssueMissingBlobRef,
			Hash:             hash,
			Bucket:           stat.first.meta.Bucket,
			Key:              stat.first.meta.Key,
			ExpectedRefCount: stat.count,
			Size:             stat.size,
		})
	}
}

func (s *Store) fixMissingBlobRefs(
	ctx context.Context,
	opts DedupeOptions,
	stats map[string]dedupeHashStat,
	refs map[string]metadata.BlobRef,
	result *DedupeResult,
) error {
	for hash := range stats {
		if err := s.fixMissingBlobRef(ctx, opts, hash, stats[hash], refs, result); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) fixMissingBlobRef(
	ctx context.Context,
	opts DedupeOptions,
	hash string,
	stat dedupeHashStat,
	refs map[string]metadata.BlobRef,
	result *DedupeResult,
) error {
	if _, ok := refs[hash]; ok || opts.DryRun || !stat.first.hasInfo {
		return nil
	}
	if result.reachedLimit(opts) {
		return nil
	}
	if err := s.meta.CreateBlobRef(ctx, hash, stat.first.info.ShardDir, stat.first.info.Size, stat.first.info.ShardPlacements, stat.first.info.ShardChecksums, stat.first.info.ShardSizes); err != nil {
		return fmt.Errorf("create missing blob ref: %w", mapStoreError(err))
	}
	result.MissingBlobRefsFixed++
	result.Fixes++
	refs[hash] = metadata.BlobRef{
		Hash:            hash,
		Path:            stat.first.info.ShardDir,
		ShardPlacements: stat.first.info.ShardPlacements,
		ShardChecksums:  stat.first.info.ShardChecksums,
		ShardSizes:      stat.first.info.ShardSizes,
		RefCount:        1,
		Size:            stat.first.info.Size,
	}
	return s.increaseBlobRef(ctx, opts, hash, stat.count-1, result)
}

func (s *Store) fixExistingBlobRefs(
	ctx context.Context,
	opts DedupeOptions,
	stats map[string]dedupeHashStat,
	refs []metadata.BlobRef,
	result *DedupeResult,
) error {
	for index := range refs {
		ref := refs[index]
		expected := stats[ref.Hash].count
		if expected == ref.RefCount {
			continue
		}
		recordRefCountDrift(ref, expected, result)
		if opts.DryRun {
			recordDryRunOrphan(ref, expected, result)
			continue
		}
		if err := s.fixBlobRefCount(ctx, opts, ref, expected, result); err != nil {
			return err
		}
	}
	return nil
}

func recordRefCountDrift(ref metadata.BlobRef, expected int, result *DedupeResult) {
	result.RefCountDrift++
	result.addIssue(DedupeIssue{
		Kind:             refCountIssueKind(expected),
		Hash:             ref.Hash,
		ExpectedRefCount: expected,
		ActualRefCount:   ref.RefCount,
		Path:             ref.Path,
		Size:             ref.Size,
	})
}

func recordDryRunOrphan(ref metadata.BlobRef, expected int, result *DedupeResult) {
	if expected != 0 {
		return
	}
	result.OrphanBlobRefs++
	result.BytesReclaimable += ref.Size
}

func (s *Store) fixBlobRefCount(
	ctx context.Context,
	opts DedupeOptions,
	ref metadata.BlobRef,
	expected int,
	result *DedupeResult,
) error {
	if expected == 0 {
		result.OrphanBlobRefs++
		result.BytesReclaimable += ref.Size
		return s.removeOrphanBlobRef(ctx, opts, ref, result)
	}
	if ref.RefCount < expected {
		return s.increaseBlobRef(ctx, opts, ref.Hash, expected-ref.RefCount, result)
	}
	return s.decreaseBlobRefToExpected(ctx, opts, ref, expected, result)
}

func (s *Store) increaseBlobRef(
	ctx context.Context,
	opts DedupeOptions,
	hash string,
	delta int,
	result *DedupeResult,
) error {
	for range delta {
		if result.reachedLimit(opts) {
			return nil
		}
		if err := s.meta.IncreaseBlobRef(ctx, hash); err != nil {
			return fmt.Errorf("increase blob ref during dedupe: %w", mapStoreError(err))
		}
		result.RefCountIncreased++
		result.Fixes++
	}
	return nil
}

func (s *Store) decreaseBlobRefToExpected(
	ctx context.Context,
	opts DedupeOptions,
	ref metadata.BlobRef,
	expected int,
	result *DedupeResult,
) error {
	for current := ref.RefCount; current > expected; current-- {
		if result.reachedLimit(opts) {
			return nil
		}
		if _, _, err := s.meta.DecreaseBlobRef(ctx, ref.Hash); err != nil {
			return fmt.Errorf("decrease blob ref during dedupe: %w", mapStoreError(err))
		}
		result.RefCountDecreased++
		result.Fixes++
	}
	return nil
}

func (s *Store) removeOrphanBlobRef(
	ctx context.Context,
	opts DedupeOptions,
	ref metadata.BlobRef,
	result *DedupeResult,
) error {
	for {
		if result.reachedLimit(opts) {
			return nil
		}
		path, removed, err := s.meta.DecreaseBlobRef(ctx, ref.Hash)
		if err != nil {
			return fmt.Errorf("remove orphan blob ref: %w", mapStoreError(err))
		}
		result.RefCountDecreased++
		result.Fixes++
		if removed {
			return s.deleteRemovedOrphanBlob(ctx, path, ref, result)
		}
	}
}

func (s *Store) deleteRemovedOrphanBlob(
	ctx context.Context,
	path string,
	ref metadata.BlobRef,
	result *DedupeResult,
) error {
	if err := s.engine.DeleteBlob(ctx, path, ref.Hash); err != nil {
		return fmt.Errorf("delete orphan dedupe blob: %w", mapStoreError(err))
	}
	result.OrphanBlobRefsRemoved++
	result.BytesReclaimed += ref.Size
	return nil
}

func refCountIssueKind(expected int) string {
	if expected == 0 {
		return DedupeIssueOrphanBlobRef
	}
	return DedupeIssueRefCountDrift
}
