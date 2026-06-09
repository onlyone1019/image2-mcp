param(
  [switch]$Interactive,
  [switch]$ConfigureCodex,
  [string]$BaseUrl = $(if ($env:OPENAI_IMAGE_BASE_URL) { $env:OPENAI_IMAGE_BASE_URL } else { "https://api.schyler.top" }),
  [switch]$SkipTests,
  [switch]$Smoke,
  [switch]$Help
)

$ErrorActionPreference = "Stop"

function Show-Help {
  @"
Usage: powershell -ExecutionPolicy Bypass -File .\install.ps1 [options]

Build and optionally configure the Image2 MCP server for Codex on Windows.

Options:
  -Interactive       Prompt for base URL and API key, then write .env.local.
  -ConfigureCodex    Append an image2 MCP server block to ~/.codex/config.toml.
  -BaseUrl URL       Set OPENAI_IMAGE_BASE_URL in .env.local or Codex config.
  -SkipTests         Build without running go test ./...
  -Smoke             Run a real image-generation smoke test after build.
  -Help              Show this help.

Environment:
  OPENAI_IMAGE_API_KEY   Required by Codex at runtime, and required for -Smoke.
  OPENAI_IMAGE_BASE_URL  Optional default base URL; defaults to https://api.schyler.top.
"@
}

function ConvertTo-DotEnvValue([string]$Value) {
  $Escaped = $Value.Replace('\', '\\').Replace('"', '\"')
  return '"' + $Escaped + '"'
}

function Import-DotEnv([string]$Path) {
  if (-not (Test-Path $Path)) {
    return
  }
  foreach ($Line in Get-Content $Path) {
    $Trimmed = $Line.Trim()
    if ($Trimmed -eq "" -or $Trimmed.StartsWith("#")) {
      continue
    }
    $Parts = $Trimmed.Split("=", 2)
    if ($Parts.Count -ne 2) {
      continue
    }
    $Name = $Parts[0].Trim()
    $Value = $Parts[1].Trim()
    if (($Value.StartsWith('"') -and $Value.EndsWith('"')) -or ($Value.StartsWith("'") -and $Value.EndsWith("'"))) {
      $Value = $Value.Substring(1, $Value.Length - 2)
    }
    [Environment]::SetEnvironmentVariable($Name, $Value, "Process")
  }
}

if ($Help) {
  Show-Help
  exit 0
}

$RepoDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $RepoDir

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  throw "go is required but was not found in PATH"
}

if ($Interactive) {
  Write-Host "==> Interactive configuration"
  $InputBaseUrl = Read-Host "OPENAI_IMAGE_BASE_URL [$BaseUrl]"
  if (-not [string]::IsNullOrWhiteSpace($InputBaseUrl)) {
    $BaseUrl = $InputBaseUrl
  }

  if ($env:OPENAI_IMAGE_API_KEY) {
    $SaveCurrent = Read-Host "OPENAI_IMAGE_API_KEY is already set in this shell. Save it to .env.local? [y/N]"
    if ($SaveCurrent -match '^[Yy]$') {
      $ApiKey = $env:OPENAI_IMAGE_API_KEY
    } else {
      $ApiKey = ""
    }
  } else {
    $Secret = Read-Host "OPENAI_IMAGE_API_KEY" -AsSecureString
    $Ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($Secret)
    try {
      $ApiKey = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($Ptr)
    } finally {
      [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($Ptr)
    }
  }

  $EnvLines = @("OPENAI_IMAGE_BASE_URL=$(ConvertTo-DotEnvValue $BaseUrl)")
  if (-not [string]::IsNullOrWhiteSpace($ApiKey)) {
    $EnvLines += "OPENAI_IMAGE_API_KEY=$(ConvertTo-DotEnvValue $ApiKey)"
  }
  Set-Content -Path ".env.local" -Value $EnvLines -Encoding UTF8
  Write-Host "==> Wrote local environment file: $RepoDir\.env.local"
}

Import-DotEnv (Join-Path $RepoDir ".env.local")
Import-DotEnv (Join-Path $RepoDir ".env")
if ($env:OPENAI_IMAGE_BASE_URL) {
  $BaseUrl = $env:OPENAI_IMAGE_BASE_URL
}

New-Item -ItemType Directory -Force -Path "dist" | Out-Null

if (-not $SkipTests) {
  Write-Host "==> Running tests"
  go test ./...
}

Write-Host "==> Building dist/image2-mcp.exe"
go build -o ".\dist\image2-mcp.exe" ".\cmd\image2-mcp"

if ($Smoke) {
  if (-not $env:OPENAI_IMAGE_API_KEY -and -not (Test-Path ".env.local")) {
    throw "-Smoke requires OPENAI_IMAGE_API_KEY or .env.local"
  }
  Write-Host "==> Running real image-generation smoke test"
  $env:RUN_IMAGE2_SMOKE = "1"
  $env:OPENAI_IMAGE_BASE_URL = $BaseUrl
  go test ./internal/image2 -run TestRealGenerateImage2Smoke -count=1 -v
}

if ($ConfigureCodex) {
  $ConfigDir = Join-Path $HOME ".codex"
  $ConfigFile = Join-Path $ConfigDir "config.toml"
  New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null
  if (-not (Test-Path $ConfigFile)) {
    New-Item -ItemType File -Path $ConfigFile | Out-Null
  }

  $Existing = Get-Content $ConfigFile -Raw
  if ($Existing -match '(?m)^\[mcp_servers\.image2\]') {
    Write-Host "==> Codex MCP config already contains [mcp_servers.image2]; leaving it unchanged: $ConfigFile"
  } else {
    Write-Host "==> Adding image2 MCP config to $ConfigFile"
    $RunScript = (Join-Path $RepoDir "scripts\run-image2-mcp.ps1") -replace '\\', '\\'
    Add-Content -Path $ConfigFile -Value @"

[mcp_servers.image2]
command = "powershell.exe"
args = ["-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "$RunScript"]
"@
  }
}

Write-Host ""
Write-Host "Image2 MCP is ready."
Write-Host "Binary: $RepoDir\dist\image2-mcp.exe"
Write-Host "Runner: $RepoDir\scripts\run-image2-mcp.ps1"
Write-Host "Base URL: $BaseUrl"
if (-not (Test-Path ".env.local") -and -not $env:OPENAI_IMAGE_API_KEY) {
  Write-Host "Note: set OPENAI_IMAGE_API_KEY or run .\install.ps1 -Interactive before Codex uses the MCP server."
}
