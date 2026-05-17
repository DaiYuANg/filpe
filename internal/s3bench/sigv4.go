package s3bench

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	sigV4Algorithm = "AWS4-HMAC-SHA256"
	sigV4Request   = "aws4_request"
	s3ServiceName  = "s3"
	sigV4TimeFmt   = "20060102T150405Z"
)

func (b bench) sign(req *http.Request) {
	if b.cfg.AccessKey == "" && b.cfg.SecretKey == "" {
		return
	}

	amzDate := time.Now().UTC().Format(sigV4TimeFmt)
	date := amzDate[:8]
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")

	signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	signature := requestSignature(req, b.cfg.SecretKey, date, b.cfg.Region, amzDate, signedHeaders, req.URL.Query())
	credential := strings.Join([]string{b.cfg.AccessKey, date, b.cfg.Region, s3ServiceName, sigV4Request}, "/")
	req.Header.Set("Authorization", sigV4Algorithm+
		" Credential="+credential+
		", SignedHeaders="+strings.Join(signedHeaders, ";")+
		", Signature="+signature)
}

func requestSignature(
	req *http.Request,
	secret string,
	date string,
	region string,
	amzDate string,
	signedHeaders []string,
	query url.Values,
) string {
	canonicalRequest := canonicalRequest(req, signedHeaders, query)
	credentialScope := strings.Join([]string{date, region, s3ServiceName, sigV4Request}, "/")
	stringToSign := strings.Join([]string{
		sigV4Algorithm,
		amzDate,
		credentialScope,
		sha256Hex(canonicalRequest),
	}, "\n")
	return signature(secret, date, region, s3ServiceName, stringToSign)
}

func canonicalRequest(req *http.Request, signedHeaders []string, query url.Values) string {
	headers := make([]string, 0, len(signedHeaders))
	for _, header := range signedHeaders {
		headers = append(headers, header+":"+canonicalHeaderValue(req, header))
	}
	return strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		canonicalQuery(query),
		strings.Join(headers, "\n") + "\n",
		strings.Join(signedHeaders, ";"),
		payloadHash(req),
	}, "\n")
}

func canonicalQuery(values url.Values) string {
	pairs := make([]string, 0, len(values))
	for key, items := range values {
		if len(items) == 0 {
			pairs = append(pairs, sigV4Escape(key)+"=")
			continue
		}
		for _, value := range items {
			pairs = append(pairs, sigV4Escape(key)+"="+sigV4Escape(value))
		}
	}
	sort.Strings(pairs)
	return strings.Join(pairs, "&")
}

func canonicalHeaderValue(req *http.Request, header string) string {
	if header == "host" {
		return strings.ToLower(strings.TrimSpace(req.Host))
	}
	return strings.Join(strings.Fields(strings.Join(req.Header.Values(header), ",")), " ")
}

func payloadHash(req *http.Request) string {
	hash := strings.TrimSpace(req.Header.Get("X-Amz-Content-Sha256"))
	if hash == "" {
		return "UNSIGNED-PAYLOAD"
	}
	return hash
}

func sigV4Escape(value string) string {
	escaped := url.QueryEscape(value)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	escaped = strings.ReplaceAll(escaped, "%7E", "~")
	return escaped
}

func signature(secret, date, region, service, stringToSign string) string {
	dateValue := hmacSHA256([]byte("AWS4"+secret), date)
	regionValue := hmacSHA256(dateValue, region)
	serviceValue := hmacSHA256(regionValue, service)
	signingValue := hmacSHA256(serviceValue, sigV4Request)
	return hex.EncodeToString(hmacSHA256(signingValue, stringToSign))
}

func hmacSHA256(input []byte, data string) []byte {
	mac := hmac.New(sha256.New, input)
	if _, err := mac.Write([]byte(data)); err != nil {
		return nil
	}
	return mac.Sum(nil)
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
