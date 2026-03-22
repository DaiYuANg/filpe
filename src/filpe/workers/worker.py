"""Worker runtime for job execution."""

import threading
import time
from pathlib import Path

import structlog

from filpe.core.config import Config
from filpe.core.queue import MemoryQueueBackend, QueueBackend
from filpe.core.registry import ProcessorRegistry
from filpe.models.job import JobRequest, JobStatus, SourceType, StagedInput

log = structlog.get_logger()


def run_job(
    job_id: str,
    request: JobRequest,
    queue: QueueBackend,
    registry: ProcessorRegistry,
    config: Config,
    uploads: dict[str, tuple[bytes, str]] | None = None,
) -> None:
    """Execute a single job."""
    queue.set_status(job_id, JobStatus.RUNNING)
    staged: StagedInput | None = None

    try:
        processor = registry.get(request.processor)
        if not processor:
            raise ValueError(f"Unknown processor: {request.processor}")

        Path(config.temp_dir).mkdir(parents=True, exist_ok=True)

        uploads = uploads or {}
        if request.source.type == SourceType.UPLOAD and not uploads:
            raise ValueError("Upload source requires uploads")

        staged = _stage_source(request, config, uploads)
        log.info("staged_input", job_id=job_id, path=str(staged.path))

        result = processor.run(staged, request.options)
        queue.set_status(job_id, JobStatus.COMPLETED, result=result)
        log.info("job_completed", job_id=job_id, processor=request.processor)
    except Exception as e:
        log.error("job_failed", job_id=job_id, error=str(e))
        queue.set_status(job_id, JobStatus.FAILED, error=str(e))
    finally:
        if staged and staged.path.exists():
            try:
                staged.path.unlink()
            except OSError:
                pass


def _stage_source(
    request: JobRequest,
    config: Config,
    uploads: dict[str, tuple[bytes, str]],
) -> StagedInput:
    """Stage input from source (inline or upload)."""
    from filpe.sources import stage_source

    return stage_source(
        request.source,
        str(config.temp_dir),
        config.max_file_size_bytes,
        uploads if request.source.type == SourceType.UPLOAD else None,
    )


def worker_loop(
    queue: QueueBackend,
    registry: ProcessorRegistry,
    config: Config,
    poll_interval: float = 0.5,
) -> None:
    """Poll queue and process jobs (for memory backend)."""
    if not isinstance(queue, MemoryQueueBackend):
        log.warning("worker_loop only supports MemoryQueueBackend")
        return

    while True:
        item = queue.pop_pending()
        if item:
            job_id, request, uploads = item
            run_job(job_id, request, queue, registry, config, uploads)
        else:
            time.sleep(poll_interval)


def start_worker_thread(
    queue: QueueBackend,
    registry: ProcessorRegistry,
    config: Config,
) -> threading.Thread:
    """Start worker in background thread (for memory backend with API)."""
    thread = threading.Thread(
        target=worker_loop,
        args=(queue, registry, config),
        daemon=True,
    )
    thread.start()
    return thread
