"""Injector module for application wiring. Used only at entry points."""

from injector import Binder, Module, provider, singleton

from filpe.core.config import Config
from filpe.core.queue import MemoryQueueBackend, QueueBackend
from filpe.core.registry import ProcessorRegistry, get_default_registry


class FilpeModule(Module):
    """Wires Config, QueueBackend, ProcessorRegistry for application assembly."""

    def configure(self, binder: Binder) -> None:
        binder.bind(Config, to=Config, scope=singleton)
        binder.bind(QueueBackend, to=self._provide_queue_backend, scope=singleton)
        binder.bind(ProcessorRegistry, to=get_default_registry, scope=singleton)

    @provider
    def _provide_queue_backend(self, config: Config) -> QueueBackend:
        if config.backend == "valkey":
            from filpe.core.queue_celery import CeleryQueueBackend

            return CeleryQueueBackend()
        return MemoryQueueBackend()
