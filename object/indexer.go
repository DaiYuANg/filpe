package object

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/arcgolabs/eventx"
	"github.com/lyonbrown4d/maxio/internal/index"
)

func (s *Service) StartIndexWorker(_ context.Context) error {
	if s == nil || s.bus == nil || s.search == nil {
		return nil
	}
	_, err := eventx.Subscribe(s.bus, func(ctx context.Context, event ObjectEvent) error {
		workerCtx := context.WithoutCancel(ctx)
		go s.handleIndexEvent(workerCtx, event)
		return nil
	})
	if err != nil {
		return fmt.Errorf("subscribe object index worker: %w", err)
	}
	return nil
}

func (s *Service) handleIndexEvent(ctx context.Context, event ObjectEvent) {
	bucket, key := eventObjectLocation(event)
	if bucket == "" || key == "" {
		return
	}
	switch event.Event {
	case "object.updated":
		s.indexObject(ctx, bucket, key)
	case "object.deleted":
		s.search.Remove(bucket, key)
	}
}

func (s *Service) indexObject(ctx context.Context, bucket, key string) {
	body, meta, err := s.GetObject(ctx, bucket, key)
	if err != nil {
		s.logger.WarnContext(ctx, "load object for indexing failed", "bucket", bucket, "key", key, "error", err)
		return
	}
	defer closeIndexBody(ctx, s, body)

	text, err := index.ExtractText(body, meta)
	if err != nil {
		s.logger.WarnContext(ctx, "extract object text failed", "bucket", bucket, "key", key, "error", err)
	}
	s.search.UpsertDocument(meta, text)
}

func eventObjectLocation(event ObjectEvent) (string, string) {
	bucket := strings.TrimSpace(payloadString(event.Payload, "bucket"))
	key := strings.TrimSpace(payloadString(event.Payload, "key"))
	return bucket, key
}

func payloadString(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func closeIndexBody(ctx context.Context, s *Service, body io.Closer) {
	if err := body.Close(); err != nil {
		s.logger.WarnContext(ctx, "close object indexing body failed", "error", err)
	}
}
