$ErrorActionPreference = "Stop"

# Force UTF-8 for graphify on Windows consoles. Without this, Python output can
# fail on cp850/cp950 consoles when graphify emits non-ASCII repo text.
$env:PYTHONUTF8 = "1"
$env:PYTHONIOENCODING = "utf-8"

$utf8 = [System.Text.UTF8Encoding]::new($false)
$OutputEncoding = $utf8
[Console]::OutputEncoding = $utf8
try {
    [Console]::InputEncoding = $utf8
} catch {
    # Some non-interactive hosts do not allow changing input encoding.
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $repoRoot
try {
    python -X utf8 scripts/refresh_graphify.py
} finally {
    Pop-Location
}
