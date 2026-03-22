"""CLI entry point. Config wired via Injector at bootstrap."""

import uvicorn
from injector import Injector

from filpe.core.config import Config
from filpe.core.container import FilpeModule


def main() -> None:
    injector = Injector([FilpeModule()])
    config = injector.get(Config)
    uvicorn.run(
        "filpe.api.app:app",
        host=config.api_host,
        port=config.api_port,
        reload=True,
        log_level=config.log_level.lower(),
    )
