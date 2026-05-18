package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseRepairRunScopeTrimsParams(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/_repair/run?bucket=%20bucket-1%20&prefix=%20images/%20", http.NoBody)
	bucket, prefix := parseRepairRunScope(request)

	if bucket != "bucket-1" {
		t.Fatalf("expected bucket %q, got %q", "bucket-1", bucket)
	}
	if prefix != "images/" {
		t.Fatalf("expected prefix %q, got %q", "images/", prefix)
	}
}
