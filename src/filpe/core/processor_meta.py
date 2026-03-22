"""Processor metadata for API documentation. Kept separate from processors."""

PROCESSOR_METADATA: dict[str, dict] = {
    "excel.read": {
        "category": "excel",
        "description": "Read Excel file, return sheets as structured data (headers + rows).",
        "options": ["sheet_names", "max_rows", "header_row"],
    },
    "excel.write": {
        "category": "excel",
        "description": "Write JSON data to Excel file. Input: JSON (same format as excel.read output).",
        "options": ["output_filename", "sheet_order"],
    },
    "image.resize": {
        "category": "image",
        "description": "Proportional scaling (maintain aspect ratio).",
        "options": ["max_width", "max_height", "scale", "format", "quality"],
    },
    "image.crop": {
        "category": "image",
        "description": "Crop to specified region.",
        "options": ["left", "top", "width", "height", "right", "bottom", "format", "quality"],
    },
    "image.compress": {
        "category": "image",
        "description": "Compress/optimize image.",
        "options": ["quality", "max_width", "max_height", "format", "optimize"],
    },
}
