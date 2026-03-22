"""Source adapters for input file resolution."""

from filpe.models.job import SourceSpec, SourceType, StagedInput
from filpe.sources.inline import stage_inline
from filpe.sources.upload import stage_upload


def stage_source(
    source: SourceSpec,
    temp_dir: str,
    max_size_bytes: int,
    uploads: dict[str, tuple[bytes, str]] | None = None,
) -> StagedInput:
    """
    Resolve source to staged input.
    uploads: for upload source, dict of field_key -> (content_bytes, filename)
    """
    if source.type == SourceType.UPLOAD:
        if not uploads:
            raise ValueError("Upload source requires uploads dict")
        return stage_upload(source, temp_dir, max_size_bytes, uploads)
    if source.type == SourceType.INLINE:
        return stage_inline(source, temp_dir, max_size_bytes)
    raise ValueError(f"Unsupported source type: {source.type}")
