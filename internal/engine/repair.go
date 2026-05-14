package engine

import (
	"context"
	"fmt"
)

// RepairResult reports shard health before and after a repair operation.
type RepairResult struct {
	HealthBefore Health `json:"health_before"`
	HealthAfter  Health `json:"health_after"`
	Repaired     []int  `json:"repaired"`
}

// RepairObject reconstructs missing shards for one recoverable object and writes
// them back to the configured shard store.
func (e *Engine) RepairObject(ctx context.Context, bucket, key string) (RepairResult, error) {
	layout, err := e.getLayout(bucket, key)
	if err != nil {
		return RepairResult{}, err
	}

	before := e.healthFromLayout(ctx, layout)
	result := RepairResult{HealthBefore: before}
	if before.Missing == 0 {
		result.HealthAfter = before
		return result, nil
	}
	if !before.Recoverable {
		return result, ErrShardRecoveryFailed
	}

	shards, missing, err := e.readRepairShards(ctx, layout)
	if err != nil {
		return result, err
	}
	if err := e.coder.Rebuild(shards); err != nil {
		return result, fmt.Errorf("%w: %w", ErrShardRecoveryFailed, err)
	}
	if err := e.writeRepairedShards(ctx, layout, shards, missing); err != nil {
		return result, err
	}

	result.Repaired = missing
	result.HealthAfter = e.healthFromLayout(ctx, layout)
	return result, nil
}

func (e *Engine) readRepairShards(ctx context.Context, layout *Layout) ([][]byte, []int, error) {
	total := e.coder.TotalChunks()
	shards := make([][]byte, total)
	missing := make([]int, 0)
	for i := range total {
		data, err := e.readShard(ctx, layout, i)
		if err != nil {
			return nil, nil, fmt.Errorf("engine: read shard %d for repair: %w", i, err)
		}
		if data == nil {
			missing = append(missing, i)
			continue
		}
		shards[i] = data
	}
	return shards, missing, nil
}

func (e *Engine) writeRepairedShards(ctx context.Context, layout *Layout, shards [][]byte, missing []int) error {
	for _, index := range missing {
		if len(shards[index]) == 0 && layout.Size > 0 {
			return fmt.Errorf("%w: shard %d was not reconstructed", ErrShardRecoveryFailed, index)
		}
		if err := e.writeShard(ctx, e.shardPlacement(layout, index), layout.ShardDir, layout.Hash, index, shards[index]); err != nil {
			return fmt.Errorf("engine: write repaired shard %d: %w", index, err)
		}
	}
	return nil
}
