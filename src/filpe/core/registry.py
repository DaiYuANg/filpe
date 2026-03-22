"""Processor registry for extensible file operations."""

from typing import Any, Protocol

from filpe.models.job import StagedInput


class Processor(Protocol):
    """Protocol for file processors."""

    name: str

    def run(self, staged: StagedInput, options: dict[str, Any] | None) -> dict[str, Any]:
        """Execute the processor. Returns structured result and may produce artifacts."""
        ...


class ProcessorRegistry:
    """Registry of available processors."""

    def __init__(self) -> None:
        self._processors: dict[str, Processor] = {}

    def register(self, processor: Processor) -> None:
        self._processors[processor.name] = processor

    def get(self, name: str) -> Processor | None:
        return self._processors.get(name)

    def list_names(self) -> list[str]:
        return list(self._processors.keys())

    def list_with_metadata(self) -> list[dict]:
        """List processors with name and metadata for API."""
        from filpe.core.processor_meta import PROCESSOR_METADATA

        result = []
        for name in self._processors:
            meta = PROCESSOR_METADATA.get(name, {})
            result.append(
                {
                    "name": name,
                    "category": meta.get("category", "general"),
                    "description": meta.get("description", ""),
                    "options": meta.get("options", []),
                }
            )
        return result


def get_default_registry() -> ProcessorRegistry:
    """Get registry with built-in processors registered."""
    from filpe.processors.excel import ExcelReadProcessor, ExcelWriteProcessor
    from filpe.processors.image import (
        ImageCompressProcessor,
        ImageCropProcessor,
        ImageResizeProcessor,
    )

    registry = ProcessorRegistry()
    registry.register(ExcelReadProcessor())
    registry.register(ExcelWriteProcessor())
    registry.register(ImageResizeProcessor())
    registry.register(ImageCropProcessor())
    registry.register(ImageCompressProcessor())
    return registry
