package s3

import (
	"encoding/xml"
	"os"
	"sync"
	"time"
)

type multipartStore struct {
	root string
	mu   sync.Mutex
}

type multipartUpload struct {
	UploadID    string                `json:"upload_id"`
	Bucket      string                `json:"bucket"`
	Key         string                `json:"key"`
	ContentType string                `json:"content_type"`
	CreatedAt   time.Time             `json:"created_at"`
	Parts       map[int]multipartPart `json:"parts"`
}

type multipartPart struct {
	Number     int       `json:"number"`
	ETag       string    `json:"etag"`
	Size       int64     `json:"size"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type assembledMultipart struct {
	file *os.File
}

type initiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	XMLNS    string   `xml:"xmlns,attr,omitempty"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadID string   `xml:"UploadId"`
}

type completeMultipartUploadRequest struct {
	Parts []completeMultipartPart `xml:"Part"`
}

type completeMultipartPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

type completeMultipartUploadResult struct {
	XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
	XMLNS    string   `xml:"xmlns,attr,omitempty"`
	Location string   `xml:"Location"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	ETag     string   `xml:"ETag"`
}

type listPartsResult struct {
	XMLName     xml.Name         `xml:"ListPartsResult"`
	XMLNS       string           `xml:"xmlns,attr,omitempty"`
	Bucket      string           `xml:"Bucket"`
	Key         string           `xml:"Key"`
	UploadID    string           `xml:"UploadId"`
	IsTruncated bool             `xml:"IsTruncated"`
	Parts       []partItemResult `xml:"Part"`
}

type partItemResult struct {
	PartNumber   int    `xml:"PartNumber"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
}
