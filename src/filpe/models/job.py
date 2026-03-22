"""Job request, state, and result models."""

from enum import Enum
from pathlib import Path
from typing import Any

from pydantic import BaseModel, Field


class SourceType(str, Enum):
    """Supported input source types."""

    UPLOAD = "upload"
    INLINE = "inline"
    URL = "url"
    OBJECT_STORAGE = "object_storage"


class SourceSpec(BaseModel):
    """Specification of where input file comes from."""

    type: SourceType
    # For upload: key is the form field / identifier
    # For inline: base64-encoded content
    # For url: fetch URL
    # For object_storage: bucket + key
    data: dict[str, str] = Field(default_factory=dict)


class JobRequest(BaseModel):
    """Request to process a file."""

    processor: str = Field(..., description="Processor name, e.g. excel.read")
    source: SourceSpec = Field(..., description="Input source specification")
    options: dict[str, Any] = Field(default_factory=dict, description="Processor options")


class JobStatus(str, Enum):
    """Job execution status."""

    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"


class StagedInput(BaseModel):
    """Normalized input ready for processor consumption."""

    path: Path = Field(..., description="Local path to staged file")
    media_type: str | None = Field(default=None, description="Detected or provided media type")
    original_name: str | None = Field(default=None, description="Original filename if known")


class JobState(BaseModel):
    """Current state of a job."""

    job_id: str
    status: JobStatus
    request: dict[str, Any]
    result: dict[str, Any] | None = None
    error: str | None = None
