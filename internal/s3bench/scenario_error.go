package s3bench

import (
	"bytes"
	"context"
	"net/http"
)

func (b bench) runErrorScenario(ctx context.Context) {
	b.expectStatus(ctx, http.MethodHead, "missing/object.bin", nil, nil, "error_missing_head", http.StatusNotFound)

	key := "errors/range.bin"
	payload := deterministicBytes(1024, 7)
	if !b.expectStatus(ctx, http.MethodPut, key, bytes.NewReader(payload), nil, "error_range_seed", http.StatusOK) {
		return
	}
	headers := http.Header{}
	headers.Set("Range", "bytes=4096-8192")
	b.expectStatus(ctx, http.MethodGet, key, nil, headers, "error_invalid_range", http.StatusRequestedRangeNotSatisfiable)
	if !b.cfg.KeepObjects {
		b.expectStatus(ctx, http.MethodDelete, key, nil, nil, "error_range_cleanup", http.StatusNoContent)
	}
}
