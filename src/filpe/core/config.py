"""Configuration for Filpe runtime."""

from pathlib import Path
from typing import Literal

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Config(BaseSettings):
    """Filpe configuration."""

    model_config = SettingsConfigDict(
        env_prefix="FILPE_",
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    backend: Literal["memory", "rq"] = Field(
        default="memory",
        description="Job queue backend: memory for local dev, rq for distributed.",
    )
    valkey_url: str = Field(
        default="redis://localhost:6379/0",
        description="Valkey/Redis URL for RQ backend.",
    )
    temp_dir: Path = Field(
        default_factory=lambda: Path("/tmp/filpe"),
        description="Directory for temporary files during processing.",
    )
    max_file_size_mb: int = Field(
        default=50,
        description="Maximum allowed file size in MB.",
    )
    log_level: str = Field(default="INFO", description="Logging level.")
    api_host: str = Field(default="0.0.0.0", description="API bind host.")
    api_port: int = Field(default=8000, description="API bind port.")

    @property
    def max_file_size_bytes(self) -> int:
        return self.max_file_size_mb * 1024 * 1024
