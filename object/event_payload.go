package object

func objectEventPayload(meta ObjectMeta) map[string]any {
	return map[string]any{
		"bucket":              meta.Bucket,
		"key":                 meta.Key,
		"hash":                meta.Hash,
		"etag":                meta.ETag,
		"size":                meta.Size,
		"content_type":        meta.ContentType,
		"cache_control":       meta.CacheControl,
		"content_disposition": meta.ContentDisposition,
		"content_encoding":    meta.ContentEncoding,
		"content_language":    meta.ContentLanguage,
		"user_metadata":       meta.UserMetadata,
		"updated_at":          meta.UpdatedAt,
		"state":               meta.State,
		"write_intent":        meta.WriteIntent,
		"shard_placements":    meta.ShardPlacements,
		"shard_checksums":     meta.ShardChecksums,
		"shard_sizes":         meta.ShardSizes,
	}
}
