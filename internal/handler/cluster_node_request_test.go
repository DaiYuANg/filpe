package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeClusterNodeMapRejectsEmptyPayload(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{}`))
	_, err := decodeClusterNodeMap(request)
	if err == nil {
		t.Fatal("expected nodes required error")
	}
}

func TestDecodeClusterNodeMapRejectsZeroReplicaID(t *testing.T) {
	t.Parallel()

	body := `{"nodes":{"0":"127.0.0.1:63000"}}`
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	_, err := decodeClusterNodeMap(request)
	if err == nil {
		t.Fatal("expected zero replica_id error")
	}
}

func TestDecodeClusterNodeMapTrimsTarget(t *testing.T) {
	t.Parallel()

	body := `{"nodes":{"1":" 127.0.0.1:63000 "}}`
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	nodes, err := decodeClusterNodeMap(request)
	if err != nil {
		t.Fatalf("decode cluster node map: %v", err)
	}
	if got := nodes[1]; got != "127.0.0.1:63000" {
		t.Fatalf("target = %q, want %q", got, "127.0.0.1:63000")
	}
}

func TestDecodeAddReplicaRequestRejectsBlankTarget(t *testing.T) {
	t.Parallel()

	body := `{"replica_id":1,"target":"   "}`
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	_, err := decodeAddReplicaRequest(request, "join")
	if err == nil {
		t.Fatal("expected blank target error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "target is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestDecodeAddReplicaRequestRejectsZeroID(t *testing.T) {
	t.Parallel()

	body := `{"replica_id":0,"target":"127.0.0.1:63000"}`
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	_, err := decodeAddReplicaRequest(request, "join")
	if err == nil {
		t.Fatal("expected replica_id error")
	}
}

func TestDecodeAddReplicaRequestTrimsTarget(t *testing.T) {
	t.Parallel()

	body := `{"replica_id":1,"target":" 127.0.0.1:63000 "}`
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(body))
	req, err := decodeAddReplicaRequest(request, "join")
	if err != nil {
		t.Fatalf("decode add replica request: %v", err)
	}
	if req.Target != "127.0.0.1:63000" {
		t.Fatalf("target = %q, want %q", req.Target, "127.0.0.1:63000")
	}
}
