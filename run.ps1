# Filpe dev launcher - runs API with memory backend (no Redis required).
# Usage: .\run.ps1
#        .\run.ps1 worker   # run Celery worker (requires Valkey/Redis)

$ErrorActionPreference = "Stop"
$env:FILPE_BACKEND = "memory"

if ($args[0] -eq "worker") {
    $env:FILPE_BACKEND = "valkey"
    if (Get-Command uv -ErrorAction SilentlyContinue) {
        uv run filpe-worker
    } else {
        python -m filpe.workers.cli
    }
} else {
    if (Get-Command uv -ErrorAction SilentlyContinue) {
        uv run filpe
    } else {
        python -m filpe.cli
    }
}
