package object_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/lyonbrown4d/maxio/internal/index"
	"github.com/lyonbrown4d/maxio/internal/metadata"
	"github.com/lyonbrown4d/maxio/internal/model"
	"github.com/lyonbrown4d/maxio/internal/store"
	"github.com/lyonbrown4d/maxio/object"
)

func TestRebuildIndexIndexesObjectContent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	objects := newIndexTestService(t)
	if err := objects.CreateBucket(ctx, "docs"); err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	if _, err := objects.PutObject(ctx, "docs", "guide.txt", strings.NewReader("needle searchable content"), object.PutOptions{
		ContentType: "text/plain",
	}); err != nil {
		t.Fatalf("put object: %v", err)
	}

	result, err := objects.RebuildIndex(ctx)
	if err != nil {
		t.Fatalf("rebuild index: %v", err)
	}
	if result.Objects != 1 || result.Failed != 0 {
		t.Fatalf("rebuild result = %+v", result)
	}

	search, err := objects.Search(ctx, model.SearchQuery{Query: "needle"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(search.Items) != 1 {
		t.Fatalf("search hits = %d, want 1", len(search.Items))
	}
	status := objects.IndexStatus()
	if status.LastRebuildObjects != 1 || status.LastRebuildFailed != 0 {
		t.Fatalf("index status = %+v", status)
	}
}

func newIndexTestService(t *testing.T) *object.Service {
	t.Helper()

	storage, err := store.NewStore(t.TempDir(), metadata.NewInMemoryMetadata(), nil)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return object.NewService(storage, index.NewInMemorySearchEngine(), nil, slog.New(slog.DiscardHandler))
}
