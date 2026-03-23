.PHONY: help run worker install lint format format-check typecheck test build docker clean

.DEFAULT_GOAL := help

help:
	@echo "Filpe - file processing runtime"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Development:"
	@echo "  run          Start API with memory backend (no Redis, default)"
	@echo "  worker       Start Celery worker for valkey backend (requires Redis)"
	@echo "  install      Install dependencies"
	@echo ""
	@echo "Code quality:"
	@echo "  lint         Run ruff linter"
	@echo "  format       Format code with ruff"
	@echo "  format-check Check formatting without changing"
	@echo "  typecheck    Run mypy"
	@echo ""
	@echo "Build & test:"
	@echo "  test         Run pytest"
	@echo "  build        Build wheel package"
	@echo ""
	@echo "Docker:"
	@echo "  docker       Build Docker image"
	@echo "  docker-compose Build with docker compose"
	@echo ""
	@echo "Other:"
	@echo "  clean        Remove build artifacts and caches"

run:
	FILPE_BACKEND=memory uv run filpe

worker:
	FILPE_BACKEND=valkey uv run filpe-worker

install:
	uv sync

lint:
	uv run ruff check src/

format:
	uv run ruff format src/

format-check:
	uv run ruff format --check src/

typecheck:
	uv run mypy src/

test:
	uv run pytest

build:
	uv build

docker:
	docker build -t filpe:latest .

docker-compose:
	docker compose build

clean:
	rm -rf build/
	rm -rf dist/
	rm -rf *.egg-info/
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
	find . -type d -name .pytest_cache -exec rm -rf {} + 2>/dev/null || true
	find . -type d -name .mypy_cache -exec rm -rf {} + 2>/dev/null || true
	find . -type d -name .ruff_cache -exec rm -rf {} + 2>/dev/null || true
