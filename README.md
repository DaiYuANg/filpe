# Filpe

Stateless file-processing runtime.

## Quick Start

```bash
# Start API (includes in-memory worker)
uv run filpe
# Or directly with uvicorn:
PYTHONPATH=src uv run uvicorn filpe.api.app:app --reload
```

## Docker

```bash
# Build (multi-stage, minimal image)
docker build -t filpe:latest .

# Run
docker run -p 8000:8000 filpe:latest

# Or with docker-compose
docker compose up -d
```

Env: `FILPE_API_HOST`, `FILPE_API_PORT`, `FILPE_BACKEND`, `FILPE_MAX_FILE_SIZE_MB`

## Excel Processing

### Upload endpoint

```bash
curl -X POST http://localhost:8000/jobs:upload \
  -F "file=@sample.xlsx" \
  -F "processor=excel.read"
```

### Get job result

```bash
curl http://localhost:8000/jobs/{job_id}/result
```

### List processors

```bash
curl http://localhost:8000/processors
```

## Available Processors

- **excel.read** — Read Excel file, return sheets as structured data (headers + rows).
  Options: `sheet_names`, `max_rows`, `header_row`.

- **excel.write** — Write JSON data to Excel file. Input: JSON file (same format as excel.read output).
  Options: `output_filename`, `sheet_order`.
  Returns: `artifacts` with base64-encoded xlsx in the result.

- **image.resize** — Proportional scaling (maintain aspect ratio).
  Options: `max_width`, `max_height`, `scale`, `format`, `quality`.

- **image.crop** — Crop to region.
  Options: `left`, `top`, `width`, `height` (or `right`, `bottom`), `format`, `quality`.

- **image.compress** — Compress/optimize image.
  Options: `quality`, `max_width`, `max_height`, `format`, `optimize`.
