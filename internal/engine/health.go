package engine

import "context"

func (e *Engine) healthFromLayout(ctx context.Context, layout *Layout) Health {
	total := e.coder.TotalChunks()
	available := 0
	for i := range total {
		if e.shardExists(ctx, layout, i) {
			available++
		}
	}

	return Health{
		Bucket:      layout.Bucket,
		Key:         layout.Key,
		TotalChunks: total,
		Available:   available,
		Missing:     total - available,
		Recoverable: available >= e.dataChunks,
	}
}
