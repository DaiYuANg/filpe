"""Queue abstraction for job dispatch and status."""

from abc import ABC, abstractmethod
from typing import Any

from filpe.models.job import JobRequest, JobStatus, JobState


class QueueBackend(ABC):
    """Abstract queue backend for job dispatch and state."""

    @abstractmethod
    def enqueue(
        self,
        job_id: str,
        request: JobRequest,
        uploads: dict[str, tuple[bytes, str]] | None = None,
    ) -> str:
        """Enqueue a job. uploads: for upload source, field_key -> (bytes, filename)."""
        ...

    @abstractmethod
    def get_status(self, job_id: str) -> JobState | None:
        """Get current job state, or None if not found."""
        ...

    @abstractmethod
    def set_status(
        self,
        job_id: str,
        status: JobStatus,
        result: dict[str, Any] | None = None,
        error: str | None = None,
    ) -> None:
        """Update job state."""
        ...


class MemoryQueueBackend(QueueBackend):
    """In-memory queue backend for local development."""

    def __init__(self) -> None:
        self._jobs: dict[str, JobState] = {}
        self._pending: list[tuple[str, JobRequest, dict[str, tuple[bytes, str]] | None]] = []

    def enqueue(
        self,
        job_id: str,
        request: JobRequest,
        uploads: dict[str, tuple[bytes, str]] | None = None,
    ) -> str:
        self._jobs[job_id] = JobState(
            job_id=job_id,
            status=JobStatus.PENDING,
            request=request.model_dump(mode="json"),
        )
        self._pending.append((job_id, request, uploads))
        return job_id

    def get_status(self, job_id: str) -> JobState | None:
        return self._jobs.get(job_id)

    def set_status(
        self,
        job_id: str,
        status: JobStatus,
        result: dict[str, Any] | None = None,
        error: str | None = None,
    ) -> None:
        state = self._jobs.get(job_id)
        if state:
            self._jobs[job_id] = state.model_copy(
                update={
                    "status": status,
                    "result": result,
                    "error": error,
                }
            )

    def pop_pending(self) -> tuple[str, JobRequest, dict[str, tuple[bytes, str]] | None] | None:
        """Pop next pending job (for worker consumption)."""
        if not self._pending:
            return None
        return self._pending.pop(0)
