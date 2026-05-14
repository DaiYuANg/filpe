// Package model contains shared MaxIO data models.
package model

import "time"

const (
	ObjectStatePending   = "pending"
	ObjectStateCommitted = "committed"
)

type Bucket struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type ShardPlacement struct {
	Index       int    `json:"index"`
	NodeID      string `json:"node_id"`
	NodeAddress string `json:"node_address,omitempty"`
	Local       bool   `json:"local,omitempty"`
}

type ObjectMeta struct {
	Bucket          string           `json:"bucket"`
	Key             string           `json:"key"`
	Hash            string           `json:"hash"`
	ETag            string           `json:"etag"`
	Size            int64            `json:"size"`
	ContentType     string           `json:"content_type"`
	UpdatedAt       time.Time        `json:"updated_at"`
	State           string           `json:"state,omitempty"`
	ShardPlacements []ShardPlacement `json:"shard_placements,omitempty"`
}

type SearchQuery struct {
	Bucket       string `json:"bucket,omitempty"`
	Prefix       string `json:"prefix,omitempty"`
	NameContains string `json:"name_contains,omitempty"`
	Limit        int    `json:"limit,omitempty"`
	MinSize      int64  `json:"min_size,omitempty"`
	MaxSize      int64  `json:"max_size,omitempty"`
}

type SearchResult struct {
	Items []ObjectMeta `json:"items"`
}
