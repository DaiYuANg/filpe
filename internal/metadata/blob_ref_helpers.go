package metadata

import "github.com/lyonbrown4d/maxio/internal/model"

func cloneBlobRefPlacements(input []model.ShardPlacement) []model.ShardPlacement {
	if len(input) == 0 {
		return nil
	}
	output := make([]model.ShardPlacement, len(input))
	copy(output, input)
	return output
}
