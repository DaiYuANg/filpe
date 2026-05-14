package raft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const defaultRaftOperationTimeout = 5 * time.Second

func (rt *Runtime) ProposeMetadata(ctx context.Context, command MetadataCommand) (MetadataResult, error) {
	if rt == nil || rt.node == nil {
		return MetadataResult{}, errors.New("raft runtime is not ready")
	}

	payload, err := json.Marshal(command)
	if err != nil {
		return MetadataResult{}, fmt.Errorf("marshal metadata command: %w", err)
	}

	ctx, cancel := withRaftOperationTimeout(ctx)
	defer cancel()

	session := rt.node.GetNoOPSession(rt.cfg.shardID)
	result, err := rt.node.SyncPropose(ctx, session, payload)
	if err != nil {
		return MetadataResult{}, fmt.Errorf("propose metadata command: %w", err)
	}

	return decodeMetadataEnvelope(result.Data)
}

func (rt *Runtime) ReadMetadata(ctx context.Context, query MetadataQuery) (MetadataResult, error) {
	if rt == nil || rt.node == nil {
		return MetadataResult{}, errors.New("raft runtime is not ready")
	}

	ctx, cancel := withRaftOperationTimeout(ctx)
	defer cancel()

	value, err := rt.node.SyncRead(ctx, rt.cfg.shardID, query)
	if err != nil {
		return MetadataResult{}, fmt.Errorf("read metadata query: %w", err)
	}

	return resultFromMetadataEnvelope(value)
}

func withRaftOperationTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), defaultRaftOperationTimeout)
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, defaultRaftOperationTimeout)
}
