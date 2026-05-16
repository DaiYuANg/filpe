package s3

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const (
	sigV4Algorithm = "AWS4-HMAC-SHA256"
	sigV4Request   = "aws4_request"
	s3ServiceName  = "s3"
)

var (
	errSigV4ConfigIncomplete = errors.New("s3 authentication is not fully configured")
	errSigV4MissingHeader    = errors.New("missing s3 authorization header")
	errSigV4InvalidHeader    = errors.New("invalid s3 authorization header")
	errSigV4AccessDenied     = errors.New("s3 signature verification failed")
	errSigV4UnsupportedScope = errors.New("unsupported s3 credential scope")
	errSigV4MissingDate      = errors.New("missing s3 request date")
)

type sigV4Authorization struct {
	accessKey     string
	date          string
	region        string
	service       string
	signedHeaders []string
	signature     string
}

func (s *Service) authorize(r *http.Request) error {
	accessKey, signingSecret, err := s.s3Credentials()
	if err != nil || accessKey == "" {
		return err
	}

	auth, err := parseSigV4Authorization(r.Header.Get("Authorization"))
	if err != nil {
		return err
	}
	validateErr := s.validateSigV4Authorization(auth, accessKey)
	if validateErr != nil {
		return validateErr
	}

	amzDate, err := sigV4RequestDate(r, auth.date)
	if err != nil {
		return err
	}
	if !validSigV4Signature(r, auth, signingSecret, amzDate) {
		return errSigV4AccessDenied
	}

	return nil
}

func (s *Service) s3Credentials() (string, string, error) {
	accessKey := strings.TrimSpace(s.cfg.S3AccessKey)
	signingSecret := strings.TrimSpace(s.cfg.S3SecretKey)
	if accessKey == "" && signingSecret == "" {
		return "", "", nil
	}
	if accessKey == "" || signingSecret == "" {
		return "", "", errSigV4ConfigIncomplete
	}
	return accessKey, signingSecret, nil
}

func (s *Service) validateSigV4Authorization(auth sigV4Authorization, accessKey string) error {
	if auth.accessKey != accessKey {
		return errSigV4AccessDenied
	}

	region := strings.TrimSpace(s.cfg.S3Region)
	if region == "" {
		region = "us-east-1"
	}
	if auth.region != region || auth.service != s3ServiceName {
		return errSigV4UnsupportedScope
	}
	return nil
}

func sigV4RequestDate(r *http.Request, credentialDate string) (string, error) {
	amzDate := strings.TrimSpace(r.Header.Get("X-Amz-Date"))
	if amzDate == "" || !strings.HasPrefix(amzDate, credentialDate) {
		return "", errSigV4MissingDate
	}
	return amzDate, nil
}

func validSigV4Signature(r *http.Request, auth sigV4Authorization, signingSecret, amzDate string) bool {
	canonicalRequest := canonicalSigV4Request(r, auth.signedHeaders)
	credentialScope := strings.Join([]string{auth.date, auth.region, auth.service, sigV4Request}, "/")
	stringToSign := strings.Join([]string{
		sigV4Algorithm,
		amzDate,
		credentialScope,
		sha256Hex(canonicalRequest),
	}, "\n")
	expected := sigV4Signature(signingSecret, auth.date, auth.region, auth.service, stringToSign)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(auth.signature))) == 1
}

func parseSigV4Authorization(header string) (sigV4Authorization, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return sigV4Authorization{}, errSigV4MissingHeader
	}
	if !strings.HasPrefix(header, sigV4Algorithm+" ") {
		return sigV4Authorization{}, errSigV4InvalidHeader
	}

	values := parseAuthorizationValues(strings.TrimSpace(strings.TrimPrefix(header, sigV4Algorithm)))
	credential := values["Credential"]
	signedHeaders := values["SignedHeaders"]
	signature := values["Signature"]
	if credential == "" || signedHeaders == "" || signature == "" {
		return sigV4Authorization{}, errSigV4InvalidHeader
	}

	parts := strings.Split(credential, "/")
	if len(parts) != 5 || parts[4] != sigV4Request {
		return sigV4Authorization{}, errSigV4InvalidHeader
	}

	headers := parseSignedHeaders(signedHeaders)
	if len(headers) == 0 {
		return sigV4Authorization{}, errSigV4InvalidHeader
	}

	return sigV4Authorization{
		accessKey:     parts[0],
		date:          parts[1],
		region:        parts[2],
		service:       parts[3],
		signedHeaders: headers,
		signature:     signature,
	}, nil
}

func parseAuthorizationValues(input string) map[string]string {
	values := make(map[string]string, 3)
	for segment := range strings.SplitSeq(input, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(segment), "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return values
}

func parseSignedHeaders(input string) []string {
	seen := make(map[string]struct{})
	headers := make([]string, 0, strings.Count(input, ";")+1)
	for header := range strings.SplitSeq(input, ";") {
		header = strings.ToLower(strings.TrimSpace(header))
		if header == "" {
			continue
		}
		if _, ok := seen[header]; ok {
			continue
		}
		seen[header] = struct{}{}
		headers = append(headers, header)
	}
	sort.Strings(headers)
	return headers
}

func canonicalSigV4Request(r *http.Request, signedHeaders []string) string {
	headers := make([]string, 0, len(signedHeaders))
	for _, header := range signedHeaders {
		headers = append(headers, header+":"+canonicalSigV4HeaderValue(r, header))
	}

	return strings.Join([]string{
		r.Method,
		canonicalSigV4URI(r),
		canonicalSigV4Query(r.URL.Query()),
		strings.Join(headers, "\n") + "\n",
		strings.Join(signedHeaders, ";"),
		payloadHash(r),
	}, "\n")
}

func canonicalSigV4URI(r *http.Request) string {
	path := r.URL.EscapedPath()
	if path == "" {
		return "/"
	}
	return path
}

func canonicalSigV4Query(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

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

func canonicalSigV4HeaderValue(r *http.Request, header string) string {
	if header == "host" {
		return strings.ToLower(strings.TrimSpace(r.Host))
	}
	return strings.Join(strings.Fields(strings.Join(r.Header.Values(header), ",")), " ")
}

func payloadHash(r *http.Request) string {
	hash := strings.TrimSpace(r.Header.Get("X-Amz-Content-Sha256"))
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

func sigV4Signature(signingSecret, date, region, service, stringToSign string) string {
	dateKey := hmacSHA256([]byte("AWS4"+signingSecret), date)
	regionKey := hmacSHA256(dateKey, region)
	serviceKey := hmacSHA256(regionKey, service)
	signingKey := hmacSHA256(serviceKey, sigV4Request)
	return hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	if _, err := mac.Write([]byte(data)); err != nil {
		return nil
	}
	return mac.Sum(nil)
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
