package s3bench

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
)

func (b bench) runObjectScenario(ctx context.Context) {
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range b.cfg.Concurrency {
		wg.Go(func() {
			for index := range jobs {
				b.runObjectFlow(ctx, index)
			}
		})
	}
	for index := range b.cfg.Objects {
		jobs <- index
	}
	close(jobs)
	wg.Wait()
}

func (b bench) runObjectFlow(ctx context.Context, index int) {
	key := fmt.Sprintf("objects/%06d.bin", index)
	payload := deterministicBytes(b.cfg.ObjectBytes, index)

	if !b.expectStatus(ctx, http.MethodPut, key, bytes.NewReader(payload), nil, "put_object", http.StatusOK) {
		return
	}
	if !b.expectStatus(ctx, http.MethodHead, key, nil, nil, "head_object", http.StatusOK) {
		return
	}

	body, ok := b.expectBody(ctx, http.MethodGet, key, nil, nil, "get_object", http.StatusOK)
	if ok && !bytes.Equal(body, payload) {
		b.metrics.recordFailure("get_object", "body mismatch")
	}

	rangeHeader := http.Header{}
	rangeHeader.Set("Range", "bytes=0-31")
	rangeBody, ok := b.expectBody(ctx, http.MethodGet, key, nil, rangeHeader, "range_get", http.StatusPartialContent)
	if ok && !bytes.Equal(rangeBody, payload[:min(32, len(payload))]) {
		b.metrics.recordFailure("range_get", "range body mismatch")
	}

	if !b.cfg.KeepObjects {
		b.expectStatus(ctx, http.MethodDelete, key, nil, nil, "delete_object", http.StatusNoContent)
	}
}
