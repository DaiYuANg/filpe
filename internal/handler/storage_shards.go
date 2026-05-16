package handler

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

func (s *Service) handleStorageShardRoute(w http.ResponseWriter, r *http.Request, parts []string) bool {
	if !isStorageShardRoute(parts) {
		return false
	}
	s.handleStorageShard(w, r, parts[3], parts[4], parts[5])
	return true
}

func isStorageShardRoute(parts []string) bool {
	return len(parts) == 6 &&
		parts[0] == "_internal" &&
		parts[1] == "storage" &&
		parts[2] == "shards"
}

func (s *Service) handleStorageShard(w http.ResponseWriter, r *http.Request, rawShardDir, rawHash, rawIndex string) {
	if s.engine == nil {
		http.Error(w, "engine unavailable", http.StatusInternalServerError)
		return
	}

	shardDir, hash, index, err := parseStorageShardPath(rawShardDir, rawHash, rawIndex)
	if err != nil {
		http.Error(w, "invalid shard directory", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		s.handleStorageShardPut(w, r, shardDir, hash, index)
	case http.MethodGet:
		s.handleStorageShardGet(w, r, shardDir, hash, index)
	case http.MethodHead:
		s.handleStorageShardHead(w, r, shardDir, hash, index)
	case http.MethodDelete:
		s.handleStorageShardDelete(w, r, shardDir, hash, index)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func parseStorageShardPath(rawShardDir, rawHash, rawIndex string) (string, string, int, error) {
	shardDir, err := url.PathUnescape(rawShardDir)
	if err != nil {
		return "", "", 0, fmt.Errorf("unescape shard directory: %w", err)
	}
	hash, err := url.PathUnescape(rawHash)
	if err != nil {
		return "", "", 0, fmt.Errorf("unescape shard hash: %w", err)
	}
	index, err := strconv.Atoi(rawIndex)
	if err != nil {
		return "", "", 0, fmt.Errorf("parse shard index: %w", err)
	}
	return shardDir, hash, index, nil
}

func (s *Service) handleStorageShardPut(w http.ResponseWriter, r *http.Request, shardDir, hash string, index int) {
	data, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		http.Error(w, readErr.Error(), http.StatusBadRequest)
		return
	}
	if err := s.engine.WriteLocalShard(r.Context(), shardDir, hash, index, data); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleStorageShardGet(w http.ResponseWriter, r *http.Request, shardDir, hash string, index int) {
	data, readErr := s.engine.ReadLocalShard(r.Context(), shardDir, hash, index)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		s.writeError(w, readErr)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	if _, writeErr := io.Copy(w, bytes.NewReader(data)); writeErr != nil {
		s.logger.Warn("write shard response failed", "error", writeErr)
	}
}

func (s *Service) handleStorageShardHead(w http.ResponseWriter, r *http.Request, shardDir, hash string, index int) {
	if !s.engine.LocalShardExists(r.Context(), shardDir, hash, index) {
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Service) handleStorageShardDelete(w http.ResponseWriter, r *http.Request, shardDir, hash string, index int) {
	if err := s.engine.DeleteLocalShard(r.Context(), shardDir, hash, index); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
