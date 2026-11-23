# Builds goonhub-worker for every supported target into dist\ (Windows PowerShell).
# Usage: .\build.ps1 [-Version v0.1.0]
# Needs Go 1.24+. The worker lives in the worker\ folder of the GoonHub repository and builds
# against the GoonHub module of the parent folder (go.mod replace goonhub => ../).
param([string]$Version = "")
$root = $PSScriptRoot

# git output, or $fallback when git fails (no commit yet, no git).
function Get-GitValue([string[]]$gitArgs, [string]$fallback) {
    $ErrorActionPreference = "Continue"
    try { $v = (& git -C $root @gitArgs 2>$null) } catch { $v = $null }
    if ($LASTEXITCODE -ne 0 -or -not $v) { return $fallback }
    return "$v".Trim()
}

if (-not $Version) { $Version = Get-GitValue @("describe", "--tags", "--always", "--dirty") "dev" }
$commit = Get-GitValue @("rev-parse", "--short", "HEAD") "unknown"
$ErrorActionPreference = "Stop"
$date = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags = "-s -w -X main.version=$Version -X main.commit=$commit -X main.date=$date"
$targets = @("windows/amd64", "darwin/arm64", "darwin/amd64", "linux/amd64", "linux/arm64")

$dist = Join-Path $root "dist"
New-Item -ItemType Directory -Force $dist | Out-Null
Get-ChildItem $dist -Filter "goonhub-worker_*" -ErrorAction SilentlyContinue | Remove-Item -Force
$env:CGO_ENABLED = "0"
foreach ($t in $targets) {
    $os, $arch = $t.Split("/")
    $out = "goonhub-worker_${os}_${arch}"
    if ($os -eq "windows") { $out += ".exe" }
    Write-Host "building $out ($Version)"
    $env:GOOS = $os; $env:GOARCH = $arch
    Push-Location $root
    try {
        go build -trimpath -ldflags $ldflags -o (Join-Path $dist $out) ./cmd/goonhub-worker
        if ($LASTEXITCODE -ne 0) { throw "go build failed for $t" }
    } finally { Pop-Location }
}
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED
Copy-Item (Join-Path $root "config.example.yaml") $dist
Get-ChildItem $dist -Filter "goonhub-worker_*" | ForEach-Object {
    "{0}  {1}" -f (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLower(), $_.Name
} | Set-Content -Encoding ascii (Join-Path $dist "SHA256SUMS")
Write-Host "done: $dist"
