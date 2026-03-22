"""Inline (base64) source adapter."""

import base64
import uuid
from pathlib import Path

from filpe.models.job import SourceSpec, StagedInput


def stage_inline(
    source: SourceSpec,
    temp_dir: str,
    max_size_bytes: int,
) -> StagedInput:
    """
    Stage file from inline base64 content.
    source.data should have 'content' (base64) and optionally 'filename'.
    """
    content_b64 = source.data.get("content")
    if not content_b64:
        raise ValueError("Inline source requires 'content' (base64)")
    content = base64.b64decode(content_b64)
    if len(content) > max_size_bytes:
        raise ValueError(f"File exceeds max size {max_size_bytes} bytes")
    filename = source.data.get("filename") or "inline"
    path = Path(temp_dir) / f"{uuid.uuid4().hex}_{filename}"
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(content)
    return StagedInput(
        path=path,
        media_type=None,
        original_name=filename,
    )
