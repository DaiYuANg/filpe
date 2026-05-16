package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/lyonbrown4d/maxio/object"
)

const defaultSearchPath = "/_search"
const defaultClusterMembersPath = "/_cluster/members"
const defaultClusterBootstrapPath = "/_cluster/bootstrap"
const defaultClusterJoinPath = "/_cluster/join"
const defaultClusterRebalancePath = "/_cluster/rebalance"
const defaultClusterRebalancePlanPath = "/_cluster/rebalance/plan"
const defaultClusterStorageNodesPath = "/_cluster/storage-nodes"
const defaultClusterStorageNodesSyncPath = "/_cluster/storage-nodes/sync"
const defaultDiscoveryPath = "/_cluster/discovery"
const defaultRepairStatusPath = "/_repair/status"

type Service struct {
	logger *slog.Logger
	Dependencies
}

func NewService(deps Dependencies, logger *slog.Logger) *Service {
	return &Service{
		logger:       logger,
		Dependencies: deps,
	}
}

func (s *Service) RegisterHTTP(router *http.ServeMux) {
	router.HandleFunc("/", s.serveHTTP)
}

func (s *Service) serveHTTP(w http.ResponseWriter, r *http.Request) {
	route := strings.Trim(path.Clean(r.URL.Path), "/")
	parts := strings.Split(route, "/")

	if s.handleControlRoute(w, r, route, parts) {
		return
	}

	if route == "" {
		s.handleBuckets(w, r)
		return
	}

	if len(parts) == 1 {
		s.handleBucket(w, r, parts[0])
		return
	}

	bucket := parts[0]
	key := strings.Join(parts[1:], "/")
	s.handleObject(w, r, bucket, key)
}

func (s *Service) handleS3Route(w http.ResponseWriter, r *http.Request) bool {
	if s.s3 == nil || !s.s3.Match(r) {
		return false
	}
	s.s3.ServeHTTP(w, r)
	return true
}

func (s *Service) handleBuckets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	buckets, err := s.objects.ListBuckets(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, buckets)
}

func (s *Service) handleBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	switch r.Method {
	case http.MethodGet:
		prefix := r.URL.Query().Get("prefix")
		items, err := s.objects.ListObjects(r.Context(), bucket, prefix)
		if errors.Is(err, object.ErrBucketNotFound) {
			s.writeError(w, err)
			return
		}
		if err != nil {
			s.writeError(w, err)
			return
		}
		s.writeJSON(w, http.StatusOK, items)
	case http.MethodPut:
		if err := s.objects.CreateBucket(r.Context(), bucket); err != nil {
			s.writeError(w, err)
			return
		}
		s.writeJSON(w, http.StatusCreated, map[string]string{"bucket": bucket, "status": "created"})
	case http.MethodDelete:
		if err := s.objects.DeleteBucket(r.Context(), bucket); err != nil {
			s.writeError(w, err)
			return
		}
		s.writeJSON(w, http.StatusNoContent, nil)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Service) handleObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetObject(w, r, bucket, key)
	case http.MethodHead:
		s.handleHeadObject(w, r, bucket, key)
	case http.MethodPut:
		s.handlePutObject(w, r, bucket, key)
	case http.MethodDelete:
		s.handleDeleteObject(w, r, bucket, key)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Service) handleGetObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	body, meta, err := s.objects.GetObject(r.Context(), bucket, key)
	if err != nil {
		s.writeError(w, err)
		return
	}
	reqCtx := r.Context()
	defer func(ctx context.Context) {
		if closeErr := body.Close(); closeErr != nil {
			s.logger.WarnContext(ctx, "close object body failed", "error", closeErr)
		}
	}(reqCtx)
	writeObjectHeaders(w, meta)
	w.WriteHeader(http.StatusOK)
	if _, copyErr := io.Copy(w, body); copyErr != nil {
		s.logger.WarnContext(reqCtx, "copy object body failed", "error", copyErr)
	}
}

func (s *Service) handleHeadObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	meta, err := s.objects.StatObject(r.Context(), bucket, key)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeObjectHeaders(w, meta)
	w.WriteHeader(http.StatusOK)
}

func (s *Service) handlePutObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	meta, err := s.objects.PutObject(r.Context(), bucket, key, r.Body, object.PutOptions{
		ContentType: r.Header.Get("Content-Type"),
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, meta)
}

func (s *Service) handleDeleteObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	_, err := s.objects.DeleteObject(r.Context(), bucket, key)
	if err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := object.SearchQuery{}
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
			s.writeError(w, err)
			return
		}
	} else {
		query.Bucket = r.URL.Query().Get("bucket")
		query.Prefix = r.URL.Query().Get("prefix")
	}

	result, err := s.objects.Search(r.Context(), query)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Service) writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if value == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(value); err != nil {
		s.logger.Warn("encode response body failed", "error", err)
	}
}

func (s *Service) writeError(w http.ResponseWriter, err error) {
	msg := err.Error()
	if errors.Is(err, object.ErrBucketExists) {
		s.writeJSON(w, http.StatusConflict, map[string]string{"error": msg})
		return
	}
	if errors.Is(err, object.ErrBucketNotFound) || errors.Is(err, object.ErrNotFound) {
		s.writeJSON(w, http.StatusNotFound, map[string]string{"error": msg})
		return
	}
	s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": msg})
}

func contentTypeOrDefault(v string) string {
	if v == "" {
		return "application/octet-stream"
	}
	return v
}

func formatInt(v int64) string {
	return strconv.FormatInt(v, 10)
}

func writeObjectHeaders(w http.ResponseWriter, meta object.ObjectMeta) {
	w.Header().Set("ETag", meta.ETag)
	w.Header().Set("Content-Type", contentTypeOrDefault(meta.ContentType))
	w.Header().Set("Content-Length", formatInt(meta.Size))
}
