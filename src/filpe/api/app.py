"""Filpe HTTP API. Dependencies wired at bootstrap via Injector."""

import json
import uuid
from contextlib import asynccontextmanager

import structlog
from fastapi import FastAPI, File, Form, HTTPException, UploadFile
from fastapi.responses import ORJSONResponse
from injector import Injector

from filpe.core.config import Config
from filpe.core.container import FilpeModule
from filpe.core.queue import QueueBackend
from filpe.core.registry import ProcessorRegistry
from filpe.models.job import JobRequest, JobStatus, SourceSpec, SourceType
from filpe.workers.worker import start_worker_thread

# Composition root: wire dependencies at module load (entry point)
_injector = Injector([FilpeModule()])
_config: Config = _injector.get(Config)
_queue: QueueBackend = _injector.get(QueueBackend)
_registry: ProcessorRegistry = _injector.get(ProcessorRegistry)

_worker_thread = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    global _worker_thread
    from filpe.core.logging import setup_logging

    setup_logging(_config.log_level)
    log = structlog.get_logger()
    base = (
        f"http://127.0.0.1:{_config.api_port}"
        if _config.api_host == "0.0.0.0"
        else f"http://{_config.api_host}:{_config.api_port}"
    )
    swagger_url = f"{base}/docs"
    processor_count = len(_registry.list_names())
    log.info("filpe_started", swagger_ui=swagger_url, processors=processor_count)
    print(f"\n  Swagger UI: {swagger_url}")
    print(f"  Processors: {processor_count} registered\n")
    if _config.backend == "memory":
        _worker_thread = start_worker_thread(_queue, _registry, _config)
    yield
    _worker_thread = None


app = FastAPI(
    title="Filpe",
    description="Stateless file-processing runtime",
    default_response_class=ORJSONResponse,
    lifespan=lifespan,
)


@app.post("/jobs")
async def create_job(body: JobRequest) -> dict:
    """Create a processing job (inline source)."""
    if body.source.type != SourceType.INLINE:
        raise HTTPException(400, "Use POST /jobs:upload for upload source")
    proc = _registry.get(body.processor)
    if not proc:
        raise HTTPException(400, f"Unknown processor: {body.processor}")
    job_id = str(uuid.uuid4())
    _queue.enqueue(job_id, body)
    return {"job_id": job_id, "status": "pending"}


@app.post("/jobs:upload")
async def create_job_upload(
    file: UploadFile = File(...),
    processor: str = Form(
        "excel.read",
        description="Processor name. Examples: excel.read, excel.write, image.resize, image.crop, image.compress",
    ),
    options: str = Form(
        default="{}",
        description='Processor options as JSON. e.g. {"max_width": 800} for image.resize',
    ),
) -> dict:
    """Create a job with uploaded file. Use GET /processors for available processors and options."""
    proc = _registry.get(processor)
    if not proc:
        raise HTTPException(400, f"Unknown processor: {processor}")
    content = await file.read()
    if len(content) > _config.max_file_size_bytes:
        raise HTTPException(413, "File too large")
    opts = json.loads(options) if options else {}
    request = JobRequest(
        processor=processor,
        source=SourceSpec(type=SourceType.UPLOAD, data={"key": "file"}),
        options=opts,
    )
    job_id = str(uuid.uuid4())
    uploads = {"file": (content, file.filename or "upload")}
    _queue.enqueue(job_id, request, uploads)
    return {"job_id": job_id, "status": "pending"}


@app.get("/jobs/{job_id}")
async def get_job(job_id: str) -> dict:
    """Get job status."""
    state = _queue.get_status(job_id)
    if not state:
        raise HTTPException(404, "Job not found")
    return {"job_id": state.job_id, "status": state.status.value}


@app.get("/jobs/{job_id}/result")
async def get_job_result(job_id: str) -> dict:
    """Get job result (if completed)."""
    state = _queue.get_status(job_id)
    if not state:
        raise HTTPException(404, "Job not found")
    if state.status != JobStatus.COMPLETED:
        raise HTTPException(409, f"Job not completed: {state.status.value}")
    if state.result is None:
        raise HTTPException(500, "Result missing")
    return {"job_id": job_id, "result": state.result}


@app.get("/processors")
async def list_processors() -> dict:
    """List available processors with metadata (name, category, description, options)."""
    return {"processors": _registry.list_with_metadata()}
