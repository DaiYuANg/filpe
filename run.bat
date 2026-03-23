@echo off
REM Filpe dev launcher - runs API with memory backend (no Redis required).
REM Usage: run.bat
REM        run.bat worker   REM run Celery worker (requires Valkey/Redis)

set FILPE_BACKEND=memory
if "%1"=="worker" (
    set FILPE_BACKEND=valkey
    where uv >nul 2>&1
    if %errorlevel% equ 0 (
        uv run filpe-worker
    ) else (
        python -m filpe.workers.cli
    )
) else (
    where uv >nul 2>&1
    if %errorlevel% equ 0 (
        uv run filpe
    ) else (
        python -m filpe.cli
    )
)
