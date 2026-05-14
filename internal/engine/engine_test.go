package engine_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/lyonbrown4d/maxio/internal/engine"
	"github.com/spf13/afero"
)

func newTestEngine(t *testing.T) *engine.Engine {
	t.Helper()
	fs := afero.NewMemMapFs()
	e, err := engine.NewEngine("/test", engine.DefaultDataChunks, engine.DefaultParityChunks, fs)
	if err != nil {
		t.Fatalf("create test engine: %v", err)
	}
	return e
}

func TestNewEngine(t *testing.T) {
	fs := afero.NewMemMapFs()
	storage, err := engine.NewEngine("/test", engine.DefaultDataChunks, engine.DefaultParityChunks, fs)
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	if storage == nil {
		t.Fatal("engine is nil")
	}
}

func TestPutAndGetObject(t *testing.T) {
	ctx := context.Background()
	e := newTestEngine(t)

	content := []byte("hello world, this is a test object for erasure coding")
	meta, err := e.PutObject(ctx, "test-bucket", "test-key.txt", strings.NewReader(string(content)), "text/plain")
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if meta.Size != int64(len(content)) {
		t.Errorf("size = %d, want %d", meta.Size, len(content))
	}
	if meta.ETag == "" {
		t.Error("ETag is empty")
	}

	reader, objInfo, err := e.GetObject(ctx, "test-bucket", "test-key.txt")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Fatalf("close reader: %v", closeErr)
		}
	}()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read object data: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Errorf("data = %s, want %s", data, content)
	}
	if objInfo.ETag != meta.ETag {
		t.Errorf("ETag = %s, want %s", objInfo.ETag, meta.ETag)
	}
}

func TestDeleteObject(t *testing.T) {
	ctx := context.Background()
	e := newTestEngine(t)

	content := []byte("delete me")
	_, err := e.PutObject(ctx, "test-bucket", "delete-key.txt", strings.NewReader(string(content)), "text/plain")
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	err = e.DeleteObject(ctx, "test-bucket", "delete-key.txt")
	if err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}

	_, _, err = e.GetObject(ctx, "test-bucket", "delete-key.txt")
	if !errors.Is(err, engine.ErrObjectNotFound) {
		t.Errorf("GetObject after delete = %v, want ErrObjectNotFound", err)
	}
}

func TestHealthCheck(t *testing.T) {
	ctx := context.Background()
	e := newTestEngine(t)

	content := []byte("health check test")
	_, err := e.PutObject(ctx, "test-bucket", "health-key.txt", strings.NewReader(string(content)), "text/plain")
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	health, err := e.CheckHealth(ctx, "test-bucket", "health-key.txt")
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if health.TotalChunks != engine.DefaultDataChunks+engine.DefaultParityChunks {
		t.Errorf("TotalChunks = %d, want %d", health.TotalChunks, engine.DefaultDataChunks+engine.DefaultParityChunks)
	}
	if health.Available != health.TotalChunks {
		t.Errorf("Available = %d, want %d", health.Available, health.TotalChunks)
	}
	if !health.Recoverable {
		t.Error("Recoverable should be true when all shards are present")
	}
}
