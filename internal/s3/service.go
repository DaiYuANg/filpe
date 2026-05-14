package s3

import (
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/lyonbrown4d/maxio/object"
)

const (
	amzRequestIDHeader = "x-amz-request-id"
	contentTypeXML     = "application/xml"
	compatPrefix       = "/s3"
)

type Service struct {
	objects *object.Service
	logger  *slog.Logger
}

func NewService(objects *object.Service, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		objects: objects,
		logger:  logger,
	}
}

func (s *Service) Match(r *http.Request) bool {
	if r == nil {
		return false
	}
	if isCompatPrefix(r.URL.Path) {
		return true
	}
	if isReservedNativePath(r.URL.Path) {
		return false
	}
	if hasS3Query(r.URL.Query()) || hasS3Header(r.Header) {
		return true
	}
	auth := r.Header.Get("Authorization")
	return strings.HasPrefix(auth, "AWS ") || strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ")
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(amzRequestIDHeader, requestID())

	bucket, key, err := splitS3Path(r.URL.Path)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "InvalidURI", err.Error())
		return
	}
	switch {
	case bucket == "":
		s.handleService(w, r)
	case key == "":
		s.handleBucket(w, r, bucket)
	default:
		s.handleObject(w, r, bucket, key)
	}
}

func (s *Service) handleService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "method not allowed")
		return
	}
	buckets, err := s.objects.ListBuckets(r.Context())
	if err != nil {
		s.writeMappedError(w, err)
		return
	}
	result := listAllMyBucketsResult{
		XMLNS: defaultXMLNS,
		Owner: owner{
			ID:          "maxio",
			DisplayName: "maxio",
		},
		Buckets: make([]bucketResult, 0, len(buckets)),
	}
	for _, bucket := range buckets {
		result.Buckets = append(result.Buckets, bucketResult{
			Name:         bucket.Name,
			CreationDate: formatS3Time(bucket.CreatedAt),
		})
	}
	s.writeXML(w, http.StatusOK, result)
}

func (s *Service) handleBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	switch r.Method {
	case http.MethodHead:
		s.handleHeadBucket(w, r, bucket)
	case http.MethodGet:
		s.handleListObjects(w, r, bucket)
	case http.MethodPut:
		s.handleCreateBucket(w, r, bucket)
	case http.MethodDelete:
		s.handleDeleteBucket(w, r, bucket)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "method not allowed")
	}
}

func (s *Service) handleHeadBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	if _, err := s.objects.ListObjects(r.Context(), bucket, ""); err != nil {
		s.writeMappedError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Service) handleCreateBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	if err := s.objects.CreateBucket(r.Context(), bucket); err != nil {
		s.writeMappedError(w, err)
		return
	}
	w.Header().Set("Location", "/"+bucket)
	w.WriteHeader(http.StatusOK)
}

func (s *Service) handleDeleteBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	if err := s.objects.DeleteBucket(r.Context(), bucket); err != nil {
		s.writeMappedError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleListObjects(w http.ResponseWriter, r *http.Request, bucket string) {
	prefix := r.URL.Query().Get("prefix")
	objects, err := s.objects.ListObjects(r.Context(), bucket, prefix)
	if err != nil {
		s.writeMappedError(w, err)
		return
	}

	result := listBucketResult{
		XMLNS:       defaultXMLNS,
		Name:        bucket,
		Prefix:      prefix,
		KeyCount:    len(objects),
		MaxKeys:     maxKeys(r.URL.Query()),
		IsTruncated: false,
		Contents:    make([]objectResult, 0, len(objects)),
	}
	for _, meta := range objects {
		result.Contents = append(result.Contents, objectResult{
			Key:          meta.Key,
			LastModified: formatS3Time(meta.UpdatedAt),
			ETag:         meta.ETag,
			Size:         meta.Size,
			StorageClass: "STANDARD",
		})
	}
	s.writeXML(w, http.StatusOK, result)
}

func (s *Service) handleObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	switch r.Method {
	case http.MethodHead:
		s.handleHeadObject(w, r, bucket, key)
	case http.MethodGet:
		s.handleGetObject(w, r, bucket, key)
	case http.MethodPut:
		s.handlePutObject(w, r, bucket, key)
	case http.MethodDelete:
		s.handleDeleteObject(w, r, bucket, key)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "method not allowed")
	}
}

func (s *Service) handleHeadObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	meta, err := s.objects.StatObject(r.Context(), bucket, key)
	if err != nil {
		s.writeMappedError(w, err)
		return
	}
	writeObjectHeaders(w, meta)
	w.WriteHeader(http.StatusOK)
}

func (s *Service) handleGetObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	body, meta, err := s.objects.GetObject(r.Context(), bucket, key)
	if err != nil {
		s.writeMappedError(w, err)
		return
	}
	defer func() {
		if closeErr := body.Close(); closeErr != nil {
			s.logger.WarnContext(r.Context(), "close s3 object body failed", "error", closeErr)
		}
	}()

	writeObjectHeaders(w, meta)
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, body); err != nil {
		s.logger.WarnContext(r.Context(), "copy s3 object body failed", "error", err)
	}
}

func (s *Service) handlePutObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	meta, err := s.objects.PutObject(r.Context(), bucket, key, r.Body, object.PutOptions{
		ContentType: r.Header.Get("Content-Type"),
	})
	if err != nil {
		s.writeMappedError(w, err)
		return
	}
	writeObjectHeaders(w, meta)
	w.WriteHeader(http.StatusOK)
}

func (s *Service) handleDeleteObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if _, err := s.objects.DeleteObject(r.Context(), bucket, key); err != nil {
		s.writeMappedError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
