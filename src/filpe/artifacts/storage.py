"""Artifact storage utilities."""

import base64
import uuid
from pathlib import Path

from pydantic import BaseModel, Field


class ArtifactMetadata(BaseModel):
    """Metadata for a generated artifact."""

    name: str
    media_type: str
    size: int
    location: str
    checksum: str | None = None


def write_artifact(
    output_dir: str,
    name: str,
    content_base64: str,
    media_type: str = "application/octet-stream",
) -> dict:
    """Write artifact to output dir, return metadata dict."""
    content = base64.b64decode(content_base64)
    path = Path(output_dir) / f"{uuid.uuid4().hex}_{name}"
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(content)
    return {
        "name": name,
        "media_type": media_type,
        "size": len(content),
        "location": str(path),
    }
