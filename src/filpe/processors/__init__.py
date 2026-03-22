"""File processors."""

from filpe.processors.excel import ExcelReadProcessor, ExcelWriteProcessor
from filpe.processors.image import (
    ImageCompressProcessor,
    ImageCropProcessor,
    ImageResizeProcessor,
)

__all__ = [
    "ExcelReadProcessor",
    "ExcelWriteProcessor",
    "ImageCompressProcessor",
    "ImageCropProcessor",
    "ImageResizeProcessor",
]
