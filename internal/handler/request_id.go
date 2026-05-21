package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

const requestIDHeader = "X-Request-ID"
const maxRequestIDLength = 128

type requestIDContextKey struct{}

var requestIDFallback atomic.Uint64

func requestIDFromRequest(r *http.Request) string {
	if r == nil {
		return newRequestID()
	}
	requestID := cleanRequestID(r.Header.Get(requestIDHeader))
	if requestID != "" {
		return requestID
	}
	return newRequestID()
}

func cleanRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) > maxRequestIDLength {
		return value[:maxRequestIDLength]
	}
	return value
}

func newRequestID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err == nil {
		return hex.EncodeToString(data[:])
	}
	return fmt.Sprintf("%d-%d", time.Now().UTC().UnixNano(), requestIDFallback.Add(1))
}

func contextWithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, ok := ctx.Value(requestIDContextKey{}).(string)
	if !ok {
		return ""
	}
	return requestID
}
