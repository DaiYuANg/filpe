package s3bench

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (b bench) createBucket(ctx context.Context) error {
	resp, err := b.do(ctx, http.MethodPut, b.resourceURL(b.cfg.Bucket, "", nil), nil, nil, "create_bucket")
	if err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}
	defer b.closeResponse("create_bucket", resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("create bucket status %d", resp.StatusCode)
	}
	return nil
}

func (b bench) deleteBucket(ctx context.Context) {
	resp, err := b.do(ctx, http.MethodDelete, b.resourceURL(b.cfg.Bucket, "", nil), nil, nil, "delete_bucket")
	if err != nil {
		b.metrics.recordError("delete bucket: " + err.Error())
		return
	}
	defer b.closeResponse("delete_bucket", resp.Body)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		b.metrics.recordError(fmt.Sprintf("delete bucket status %d", resp.StatusCode))
	}
}

func (b bench) expectStatus(
	ctx context.Context,
	method string,
	key string,
	body io.Reader,
	values any,
	op string,
	expected int,
) bool {
	resp, err := b.doResource(ctx, method, key, body, values, op)
	if err != nil {
		b.metrics.recordError(op + ": " + err.Error())
		return false
	}
	defer b.closeResponse(op, resp.Body)
	if resp.StatusCode != expected {
		b.metrics.recordFailure(op, fmt.Sprintf("status %d want %d", resp.StatusCode, expected))
		return false
	}
	return true
}

func (b bench) expectBody(
	ctx context.Context,
	method string,
	key string,
	body io.Reader,
	headers http.Header,
	op string,
	expected int,
) ([]byte, bool) {
	resp, err := b.do(ctx, method, b.resourceURL(b.cfg.Bucket, key, nil), body, headers, op)
	if err != nil {
		b.metrics.recordError(op + ": " + err.Error())
		return nil, false
	}
	defer b.closeResponse(op, resp.Body)
	if resp.StatusCode != expected {
		b.metrics.recordFailure(op, fmt.Sprintf("status %d want %d", resp.StatusCode, expected))
		return nil, false
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		b.metrics.recordFailure(op, "read body: "+err.Error())
		return nil, false
	}
	return data, true
}

func (b bench) doResource(
	ctx context.Context,
	method string,
	key string,
	body io.Reader,
	values any,
	op string,
) (*http.Response, error) {
	switch typed := values.(type) {
	case nil:
		return b.do(ctx, method, b.resourceURL(b.cfg.Bucket, key, nil), body, nil, op)
	case http.Header:
		return b.do(ctx, method, b.resourceURL(b.cfg.Bucket, key, nil), body, typed, op)
	case url.Values:
		return b.do(ctx, method, b.resourceURL(b.cfg.Bucket, key, typed), body, nil, op)
	default:
		return nil, fmt.Errorf("unsupported request values %T", values)
	}
}

func (b bench) do(
	ctx context.Context,
	method string,
	target *url.URL,
	body io.Reader,
	headers http.Header,
	op string,
) (*http.Response, error) {
	if body == nil {
		body = http.NoBody
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		b.metrics.recordFailure(op, err.Error())
		return nil, fmt.Errorf("build request: %w", err)
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	b.sign(req)

	started := time.Now()
	resp, err := b.client.Do(req)
	elapsed := time.Since(started)
	if err != nil {
		b.metrics.record(op, elapsed, 0, err)
		return nil, fmt.Errorf("send request: %w", err)
	}
	b.metrics.record(op, elapsed, resp.ContentLength, nil)
	return resp, nil
}

func (b bench) closeResponse(op string, body io.Closer) {
	if body == nil {
		return
	}
	if err := body.Close(); err != nil {
		b.metrics.recordFailure(op, "close response body: "+err.Error())
	}
}

func (b bench) resourceURL(bucket, key string, query url.Values) *url.URL {
	base, err := url.Parse(b.cfg.Endpoint)
	if err != nil {
		panic(err)
	}
	parts := []string{strings.TrimRight(base.EscapedPath(), "/"), url.PathEscape(bucket)}
	if key != "" {
		for segment := range strings.SplitSeq(key, "/") {
			parts = append(parts, url.PathEscape(segment))
		}
	}
	base.Path = strings.Join(parts, "/")
	base.RawPath = ""
	if query != nil {
		base.RawQuery = query.Encode()
	}
	return base
}
