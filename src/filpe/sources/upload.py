"""Upload source adapter."""

import uuid
from pathlib import Path

from filpe.models.job import SourceSpec, StagedInput


def stage_upload(
    source: SourceSpec,
    temp_dir: str,
    max_size_bytes: int,
    uploads: dict[str, tuple[bytes, str]],
) -> StagedInput:
    """
    Stage file from upload.
    source.data should have 'key' pointing to the upload identifier.
    uploads[key] = (content_bytes, filename)
    """
    key = source.data.get("key") or "file"
    if key not in uploads:
        raise ValueError(f"Upload key '{key}' not found")
    content, filename = uploads[key]
    if len(content) > max_size_bytes:
        raise ValueError(f"File exceeds max size {max_size_bytes} bytes")
    path = Path(temp_dir) / f"{uuid.uuid4().hex}_{filename or 'upload'}"
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(content)
    return StagedInput(
        path=path,
        media_type=None,
        original_name=filename,
    )
