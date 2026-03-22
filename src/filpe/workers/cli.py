"""RQ worker CLI. Run with: filpe-worker"""

from filpe.core.config import Config
from filpe.core.logging import setup_logging


def main() -> None:
    config = Config()
    setup_logging(config.log_level)
    from redis import Redis
    from rq import Worker

    redis = Redis.from_url(config.valkey_url)
    worker = Worker(["default"], connection=redis)
    worker.work(with_scheduler=False)
