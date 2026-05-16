package raft

import (
	"strings"

	"github.com/lyonbrown4d/maxio/internal/model"
)

func (s *raftStateMachine) createBlobRef(
	hash string,
	path string,
	size int64,
	shardPlacements []model.ShardPlacement,
	shardChecksums []string,
) string {
	if invalidName(hash) || strings.TrimSpace(path) == "" {
		return MetadataErrorBadRequest
	}
	if ref, ok := s.blobRefs[hash]; ok {
		ref.RefCount++
		s.blobRefs[hash] = ref
		return ""
	}
	s.blobRefs[hash] = MetadataBlobRef{
		Path:            path,
		ShardPlacements: cloneRaftBlobPlacements(shardPlacements),
		ShardChecksums:  cloneRaftBlobChecksums(shardChecksums),
		RefCount:        1,
		Size:            size,
	}
	return ""
}

func (s *raftStateMachine) increaseBlobRef(hash string) string {
	if invalidName(hash) {
		return MetadataErrorBadRequest
	}
	ref, ok := s.blobRefs[hash]
	if !ok {
		return MetadataErrorObjectNotFound
	}
	ref.RefCount++
	s.blobRefs[hash] = ref
	return ""
}

func (s *raftStateMachine) updateBlobRefPlacements(hash string, placements []model.ShardPlacement) string {
	if invalidName(hash) {
		return MetadataErrorBadRequest
	}
	ref, ok := s.blobRefs[hash]
	if !ok {
		return MetadataErrorObjectNotFound
	}
	ref.ShardPlacements = cloneRaftBlobPlacements(placements)
	s.blobRefs[hash] = ref
	return ""
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

func cloneRaftBlobPlacements(placements []model.ShardPlacement) []model.ShardPlacement {
	if len(placements) == 0 {
		return nil
	}
	output := make([]model.ShardPlacement, len(placements))
	copy(output, placements)
	return output
}
