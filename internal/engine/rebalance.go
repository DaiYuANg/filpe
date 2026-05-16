package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lyonbrown4d/maxio/internal/model"
)

type ShardMove struct {
	Index int                  `json:"index"`
	From  model.ShardPlacement `json:"from"`
	To    model.ShardPlacement `json:"to"`
}

type RebalanceObjectResult struct {
	Object ObjectInfo  `json:"object"`
	Moved  []ShardMove `json:"moved"`
}

func (e *Engine) RebalanceObjectFromNode(ctx context.Context, bucket, key, fromNodeID string) (RebalanceObjectResult, error) {
	if fromNodeID == "" {
		return RebalanceObjectResult{}, errors.New("rebalance source node id is required")
	}
	layout, err := e.getLayout(bucket, key)
	if err != nil {
		return RebalanceObjectResult{}, err
	}

	placements, moved, err := e.rebalanceLayoutPlacements(ctx, layout, fromNodeID)
	if err != nil {
		return RebalanceObjectResult{}, err
	}
	if len(moved) == 0 {
		return RebalanceObjectResult{Object: e.objectInfoFromLayout(layout)}, nil
	}

	layout.ShardPlacements = placements
	if err := e.persistLayout(layout); err != nil {
		return RebalanceObjectResult{}, err
	}
	return RebalanceObjectResult{Object: e.objectInfoFromLayout(layout), Moved: moved}, nil
}

func (e *Engine) rebalanceLayoutPlacements(
	ctx context.Context,
	layout *Layout,
	fromNodeID string,
) ([]model.ShardPlacement, []ShardMove, error) {
	placements := cloneShardPlacements(layout.ShardPlacements)
	moved := make([]ShardMove, 0)
	for index := range placements {
		if placements[index].NodeID != fromNodeID {
			continue
		}
		target, err := e.rebalanceTargetPlacement(index, fromNodeID)
		if err != nil {
			return nil, nil, err
		}
		if err := e.moveShard(ctx, layout, index, target); err != nil {
			return nil, nil, err
		}
		moved = append(moved, ShardMove{Index: index, From: placements[index], To: target})
		placements[index] = target
	}
	return placements, moved, nil
}

func (e *Engine) rebalanceTargetPlacement(index int, fromNodeID string) (model.ShardPlacement, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	nodes := e.placementNodesLocked()
	candidates := make([]StorageNode, 0, len(nodes))
	for _, node := range nodes {
		if node.ID() != fromNodeID {
			candidates = append(candidates, node)
		}
	}
	if len(candidates) == 0 {
		return model.ShardPlacement{}, errors.New("rebalance target node is unavailable")
	}
	node := candidates[index%len(candidates)]
	nodeID := node.ID()
	return model.ShardPlacement{
		Index:       index,
		NodeID:      nodeID,
		NodeAddress: node.Address(),
		Local:       nodeID == e.localNodeID,
	}, nil
}

func (e *Engine) moveShard(ctx context.Context, layout *Layout, index int, target model.ShardPlacement) error {
	data, err := e.readShard(ctx, layout, index)
	if err != nil {
		return fmt.Errorf("read shard %d for rebalance: %w", index, err)
	}
	if err := e.writeShard(ctx, target, layout.ShardDir, layout.Hash, index, data); err != nil {
		return fmt.Errorf("write shard %d for rebalance: %w", index, err)
	}
	return nil
}

func (e *Engine) persistLayout(layout *Layout) error {
	if layout == nil {
		return errors.New("layout is required")
	}
	layoutID := layout.ID
	if layoutID == "" {
		layoutID = layoutHash(layoutKey(layout.Bucket, layout.Key))
		layout.ID = layoutID
	}
	data, err := json.Marshal(layout)
	if err != nil {
		return fmt.Errorf("engine: marshal rebalance layout: %w", err)
	}
	if err := e.backend.WriteMeta(layout.ShardDir, layoutID, data); err != nil {
		return fmt.Errorf("engine: write rebalance layout: %w", err)
	}
	e.layoutCache.Store(layoutKey(layout.Bucket, layout.Key), layout)
	return nil
}
