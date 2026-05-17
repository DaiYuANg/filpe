// Command maxio-s3bench runs live HTTP pressure checks against MaxIO's S3
// compatibility endpoint.
package main

import (
	"os"

	"github.com/lyonbrown4d/maxio/internal/s3bench"
)

func main() {
	os.Exit(s3bench.Run(os.Args[1:], os.Stdout, os.Stderr))
}
