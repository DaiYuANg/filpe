// Package s3bench provides a live HTTP benchmark for MaxIO's S3 compatibility
// endpoint.
package s3bench

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

// Config controls the live S3 benchmark.
type Config struct {
	Endpoint           string
	AccessKey          string
	SecretKey          string
	Region             string
	Bucket             string
	Objects            int
	Concurrency        int
	ObjectBytes        int
	MultipartParts     int
	MultipartPartBytes int
	Timeout            time.Duration
	KeepObjects        bool
	SkipMultipart      bool
	SkipErrors         bool
}

func parseConfig(args []string, stderr io.Writer) (Config, error) {
	cfg := Config{}
	flags := flag.NewFlagSet("maxio-s3bench", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&cfg.Endpoint, "endpoint", "http://127.0.0.1:8080/s3", "S3 compatibility endpoint")
	flags.StringVar(&cfg.AccessKey, "access-key", "", "S3 access key; leave empty when server auth is disabled")
	flags.StringVar(&cfg.SecretKey, "secret-key", "", "S3 secret key; leave empty when server auth is disabled")
	flags.StringVar(&cfg.Region, "region", "us-east-1", "S3 signing region")
	flags.StringVar(&cfg.Bucket, "bucket", "maxio-s3bench", "bucket name used by the benchmark")
	flags.IntVar(&cfg.Objects, "objects", 100, "number of objects for the concurrent object scenario")
	flags.IntVar(&cfg.Concurrency, "concurrency", 8, "parallel workers for the object scenario")
	flags.IntVar(&cfg.ObjectBytes, "object-bytes", 16*1024, "bytes per object for the object scenario")
	flags.IntVar(&cfg.MultipartParts, "multipart-parts", 2, "multipart upload part count")
	flags.IntVar(&cfg.MultipartPartBytes, "multipart-part-bytes", 5*1024*1024, "bytes per multipart part")
	flags.DurationVar(&cfg.Timeout, "timeout", 2*time.Minute, "overall benchmark timeout")
	flags.BoolVar(&cfg.KeepObjects, "keep-objects", false, "keep benchmark objects and bucket after the run")
	flags.BoolVar(&cfg.SkipMultipart, "skip-multipart", false, "skip multipart upload scenario")
	flags.BoolVar(&cfg.SkipErrors, "skip-errors", false, "skip expected error-path scenario")
	if err := flags.Parse(args); err != nil {
		return Config{}, fmt.Errorf("parse flags: %w", err)
	}
	return cfg, cfg.validate()
}

func (c Config) validate() error {
	if _, err := url.ParseRequestURI(c.Endpoint); err != nil {
		return fmt.Errorf("invalid endpoint: %w", err)
	}
	if strings.TrimSpace(c.Bucket) == "" {
		return errors.New("bucket is required")
	}
	if c.Objects < 1 {
		return errors.New("objects must be greater than zero")
	}
	if c.Concurrency < 1 {
		return errors.New("concurrency must be greater than zero")
	}
	if c.ObjectBytes < 1 {
		return errors.New("object-bytes must be greater than zero")
	}
	if c.MultipartParts < 1 {
		return errors.New("multipart-parts must be greater than zero")
	}
	if c.MultipartPartBytes < 5*1024*1024 {
		return errors.New("multipart-part-bytes must be at least 5242880")
	}
	if (c.AccessKey == "") != (c.SecretKey == "") {
		return errors.New("access-key and secret-key must be provided together")
	}
	return nil
}
