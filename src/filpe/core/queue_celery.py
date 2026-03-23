"""Celery/Valkey queue backend for distributed execution."""

import base64
import json
from typing import Any

from celery import Celery
from redis import Redis

from filpe.models.job import JobRequest, JobState, JobStatus

UPLOADS_KEY_PREFIX = "filpe:uploads:"
UPLOADS_TTL = 3600
JOB_STATE_KEY_PREFIX = "filpe:job:"
JOB_STATE_TTL = 86400


def _get_celery_app() -> Celery:
    from filpe.core.config import Config

    config = Config()
    app = Celery("filpe", broker=config.valkey_url)
    app.conf.task_default_time_limit = 3600
    app.conf.task_serializer = "json"
    app.conf.accept_content = ["json"]
    app.conf.result_expires = 86400
    app.conf.task_ignore_result = True
    return app


# Module-level Celery app for worker CLI (celery -A filpe.core.queue_celery worker)
celery_app = _get_celery_app()


def _serialize_uploads(uploads: dict[str, tuple[bytes, str]] | None) -> str:
    if not uploads:
        return "{}"
    encoded = {k: [base64.b64encode(v[0]).decode(), v[1]] for k, v in uploads.items()}
    return json.dumps(encoded)


def _deserialize_uploads(data: str) -> dict[str, tuple[bytes, str]]:
    if not data:
        return {}
    decoded = json.loads(data)
    return {k: (base64.b64decode(v[0]), v[1]) for k, v in decoded.items()}


@celery_app.task(bind=True)
def celery_job_handler(
    self: Any, job_id: str, request_dict: dict, uploads_key: str
) -> None:
    """Celery job target. Runs in worker process."""
    from filpe.core.config import Config
    from filpe.core.registry import get_default_registry
    from filpe.workers.worker import run_job

    config = Config()
    redis = Redis.from_url(config.valkey_url)
    backend = CeleryQueueBackend(redis=redis)
    registry = get_default_registry()
    request = JobRequest.model_validate(request_dict)
    uploads_data = redis.get(uploads_key)
    uploads = _deserialize_uploads(uploads_data.decode() if uploads_data else "{}")
    try:
        run_job(job_id, request, backend, registry, config, uploads)
    finally:
        redis.delete(uploads_key)


class CeleryQueueBackend:
    """Queue backend using Celery and Valkey/Redis."""

    def __init__(self, redis: Redis | None = None) -> None:
        from filpe.core.config import Config

        self._config = Config()
        self._redis = redis or Redis.from_url(self._config.valkey_url)

    def enqueue(
        self,
        job_id: str,
        request: JobRequest,
        uploads: dict[str, tuple[bytes, str]] | None = None,
    ) -> str:
        uploads_key = f"{UPLOADS_KEY_PREFIX}{job_id}"
        if uploads:
            self._redis.setex(
                uploads_key,
                UPLOADS_TTL,
                _serialize_uploads(uploads),
            )
        self._redis.setex(
            f"{JOB_STATE_KEY_PREFIX}{job_id}",
            JOB_STATE_TTL,
            json.dumps(
                {
                    "job_id": job_id,
                    "status": JobStatus.PENDING.value,
                    "request": request.model_dump(mode="json"),
                }
            ),
        )
        celery_job_handler.delay(job_id, request.model_dump(mode="json"), uploads_key)
        return job_id

    def get_status(self, job_id: str) -> JobState | None:
        data = self._redis.get(f"{JOB_STATE_KEY_PREFIX}{job_id}")
        if not data:
            return None
        return JobState.model_validate(json.loads(data.decode()))

    def set_status(
        self,
        job_id: str,
        status: JobStatus,
        result: dict[str, Any] | None = None,
        error: str | None = None,
    ) -> None:
        data = self._redis.get(f"{JOB_STATE_KEY_PREFIX}{job_id}")
        if not data:
            return
        state = JobState.model_validate(json.loads(data.decode()))
        updated = state.model_copy(update={"status": status, "result": result, "error": error})
        self._redis.setex(
            f"{JOB_STATE_KEY_PREFIX}{job_id}",
            JOB_STATE_TTL,
            json.dumps(updated.model_dump(mode="json")),
        )
