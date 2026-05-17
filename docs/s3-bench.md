# S3 benchmark

`maxio-s3bench` is a small live HTTP benchmark for the MaxIO S3 compatibility
path. It signs requests with Signature V4 when credentials are provided and
prints a JSON report.

Example:

```powershell
go run ./cmd/maxio-s3bench `
  -endpoint http://127.0.0.1:8080/s3 `
  -access-key maxio-smoke `
  -secret-key maxio-smoke-secret `
  -objects 1000 `
  -concurrency 32 `
  -object-bytes 65536
```

Covered scenarios:

- Concurrent object `PUT`, `HEAD`, full `GET`, range `GET`, and `DELETE`.
- Multipart initiate, upload parts, complete, head, and delete.
- Expected error paths for missing objects and invalid ranges.

The default settings are intentionally small enough for local smoke checks. Use
larger object counts, higher concurrency, and larger multipart parts for heavier
pressure tests.
