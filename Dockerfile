# Multi-stage build for minimal image
FROM python:3.14-slim AS builder

WORKDIR /app

# Install uv for fast, minimal dependency install
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

# Copy dependency manifests (README required by pyproject)
COPY pyproject.toml uv.lock README.md ./
COPY src ./src

# Install production deps + project into .venv
ENV UV_COMPILE_BYTECODE=0
RUN uv sync --no-dev --frozen

# Runtime stage
FROM python:3.14-slim

WORKDIR /app

# Runtime deps for Pillow (image processing)
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        libjpeg62-turbo \
        zlib1g \
        libwebp7 \
    && rm -rf /var/lib/apt/lists/*

# Copy venv and app from builder
COPY --from=builder /app/.venv /app/.venv
COPY --from=builder /app/src ./src
COPY --from=builder /app/pyproject.toml ./
ENV PATH="/app/.venv/bin:$PATH"

# Non-root user + temp dir for processing
RUN useradd --create-home --shell /bin/bash app \
    && mkdir -p /tmp/filpe && chown app:app /tmp/filpe
USER app

EXPOSE 8000
ENV FILPE_API_HOST=0.0.0.0
ENV FILPE_API_PORT=8000

CMD ["uvicorn", "filpe.api.app:app", "--host", "0.0.0.0", "--port", "8000"]
