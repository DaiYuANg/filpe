package s3bench

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type initiateMultipartUploadResult struct {
	UploadID string `xml:"UploadId"`
}

type completeMultipartUploadRequest struct {
	XMLName xml.Name                `xml:"CompleteMultipartUpload"`
	Parts   []completeMultipartPart `xml:"Part"`
}

type completeMultipartPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

func (b bench) runMultipartScenario(ctx context.Context) {
	key := "multipart/object.bin"
	uploadID, ok := b.initiateMultipartUpload(ctx, key)
	if !ok {
		return
	}
	parts, ok := b.uploadMultipartParts(ctx, key, uploadID)
	if !ok {
		return
	}
	if !b.completeMultipartUpload(ctx, key, uploadID, parts) {
		return
	}
	b.expectStatus(ctx, http.MethodHead, key, nil, nil, "multipart_head", http.StatusOK)
	if !b.cfg.KeepObjects {
		b.expectStatus(ctx, http.MethodDelete, key, nil, nil, "multipart_delete", http.StatusNoContent)
	}
}

func (b bench) initiateMultipartUpload(ctx context.Context, key string) (string, bool) {
	initURL := b.resourceURL(b.cfg.Bucket, key, url.Values{"uploads": []string{""}})
	resp, err := b.do(ctx, http.MethodPost, initURL, nil, nil, "multipart_initiate")
	if err != nil {
		b.metrics.recordError("multipart initiate: " + err.Error())
		return "", false
	}
	defer b.closeResponse("multipart_initiate", resp.Body)
	if resp.StatusCode != http.StatusOK {
		b.metrics.recordFailure("multipart_initiate", fmt.Sprintf("status %d", resp.StatusCode))
		return "", false
	}

	result := initiateMultipartUploadResult{}
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		b.metrics.recordFailure("multipart_initiate", "decode initiate result: "+err.Error())
		return "", false
	}
	if strings.TrimSpace(result.UploadID) == "" {
		b.metrics.recordFailure("multipart_initiate", "empty upload id")
		return "", false
	}
	return result.UploadID, true
}

func (b bench) uploadMultipartParts(ctx context.Context, key, uploadID string) ([]completeMultipartPart, bool) {
	parts := make([]completeMultipartPart, 0, b.cfg.MultipartParts)
	for partNumber := 1; partNumber <= b.cfg.MultipartParts; partNumber++ {
		part, ok := b.uploadMultipartPart(ctx, key, uploadID, partNumber)
		if !ok {
			return nil, false
		}
		parts = append(parts, part)
	}
	return parts, true
}

func (b bench) uploadMultipartPart(
	ctx context.Context,
	key string,
	uploadID string,
	partNumber int,
) (completeMultipartPart, bool) {
	payload := deterministicBytes(b.cfg.MultipartPartBytes, partNumber)
	query := url.Values{
		"partNumber": []string{strconv.Itoa(partNumber)},
		"uploadId":   []string{uploadID},
	}
	resp, err := b.do(
		ctx,
		http.MethodPut,
		b.resourceURL(b.cfg.Bucket, key, query),
		bytes.NewReader(payload),
		nil,
		"multipart_upload_part",
	)
	if err != nil {
		b.metrics.recordError("multipart upload part: " + err.Error())
		return completeMultipartPart{}, false
	}
	defer b.closeResponse("multipart_upload_part", resp.Body)
	if resp.StatusCode != http.StatusOK {
		b.metrics.recordFailure("multipart_upload_part", fmt.Sprintf("status %d", resp.StatusCode))
		return completeMultipartPart{}, false
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		b.metrics.recordFailure("multipart_upload_part", "empty part etag")
		return completeMultipartPart{}, false
	}
	return completeMultipartPart{PartNumber: partNumber, ETag: etag}, true
}

func (b bench) completeMultipartUpload(
	ctx context.Context,
	key string,
	uploadID string,
	parts []completeMultipartPart,
) bool {
	var completeBody bytes.Buffer
	if err := xml.NewEncoder(&completeBody).Encode(completeMultipartUploadRequest{Parts: parts}); err != nil {
		b.metrics.recordFailure("multipart_complete", "encode complete request: "+err.Error())
		return false
	}
	query := url.Values{"uploadId": []string{uploadID}}
	return b.expectStatus(ctx, http.MethodPost, key, &completeBody, query, "multipart_complete", http.StatusOK)
}
