package s3

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/afero"
)

func completeParts(upload multipartUpload, requested []completeMultipartPart) ([]multipartPart, error) {
	if len(requested) == 0 {
		return nil, errInvalidPart
	}
	parts := make([]multipartPart, 0, len(requested))
	previous := 0
	for _, item := range requested {
		if item.PartNumber <= previous {
			return nil, errInvalidPartOrder
		}
		part, ok := upload.Parts[item.PartNumber]
		if !ok || !etagMatches(part.ETag, item.ETag) {
			return nil, errInvalidPart
		}
		parts = append(parts, part)
		previous = item.PartNumber
	}
	return parts, nil
}

func sortedMultipartParts(parts map[int]multipartPart) []multipartPart {
	numbers := make([]int, 0, len(parts))
	for number := range parts {
		numbers = append(numbers, number)
	}
	sort.Ints(numbers)
	result := make([]multipartPart, 0, len(numbers))
	for _, number := range numbers {
		result = append(result, parts[number])
	}
	return result
}

func decodeCompleteMultipartUpload(reader io.Reader) (completeMultipartUploadRequest, error) {
	request := completeMultipartUploadRequest{}
	if reader == nil {
		return request, errInvalidPart
	}
	if err := xml.NewDecoder(reader).Decode(&request); err != nil {
		return request, fmt.Errorf("%w: decode complete multipart upload: %w", errInvalidPart, err)
	}
	return request, nil
}

func parsePartNumber(query url.Values) (int, error) {
	value := queryValue(query, "partNumber")
	if value == "" {
		return 0, errInvalidPart
	}
	partNumber, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%w: parse part number: %w", errInvalidPart, err)
	}
	if partNumber < 1 || partNumber > 10000 {
		return 0, errInvalidPart
	}
	return partNumber, nil
}

func validateUploadID(uploadID string) error {
	uploadID = strings.TrimSpace(uploadID)
	if uploadID == "" || len(uploadID) > 128 || strings.IndexFunc(uploadID, invalidUploadIDRune) >= 0 {
		return errInvalidUploadID
	}
	return nil
}

func invalidUploadIDRune(value rune) bool {
	if (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') || (value >= '0' && value <= '9') {
		return false
	}
	return value != '-' && value != '_'
}

func hasQueryKey(query url.Values, name string) bool {
	for key := range query {
		if strings.EqualFold(key, name) {
			return true
		}
	}
	return false
}

func queryValue(query url.Values, name string) string {
	for key, values := range query {
		if !strings.EqualFold(key, name) || len(values) == 0 {
			continue
		}
		return values[0]
	}
	return ""
}

func quoteETag(value string) string {
	value = strings.Trim(value, `"`)
	return `"` + value + `"`
}

func etagMatches(stored, requested string) bool {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return true
	}
	return strings.Trim(stored, `"`) == strings.Trim(requested, `"`)
}

func (a assembledMultipart) close() error {
	if a.file == nil {
		return nil
	}
	if err := a.file.Close(); err != nil {
		return fmt.Errorf("close multipart assembly: %w", err)
	}
	return nil
}

func closeFile(file interface{ Close() error }) {
	if file == nil {
		return
	}
	if err := file.Close(); err != nil {
		_ = err.Error()
	}
}

func closeAferoFile(file afero.File) {
	closeFile(file)
}

func contextError(ctx context.Context, operation string) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s context: %w", operation, err)
	}
	return nil
}
