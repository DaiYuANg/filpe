"""Celery worker CLI. Run with: filpe-worker"""

import subprocess
import sys

from filpe.core.config import Config
from filpe.core.logging import setup_logging


def main() -> None:
    config = Config()
    if config.backend == "memory":
        sys.exit(
            "filpe-worker is for valkey backend. "
            "With backend='memory', the API runs jobs in-process; no separate worker needed."
        )
    setup_logging(config.log_level)
    # Use solo pool on Windows (no fork), prefork on Unix
    pool = "solo" if sys.platform == "win32" else "prefork"
    args = [
        sys.executable,
        "-m",
        "celery",
        "-A",
        "filpe.core.queue_celery",
        "worker",
        f"--pool={pool}",
    ]
    subprocess.run(args, check=True)
