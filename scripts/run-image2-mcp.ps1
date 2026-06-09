$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoDir = Split-Path -Parent $ScriptDir

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

Import-DotEnv (Join-Path $RepoDir ".env.local")
Import-DotEnv (Join-Path $RepoDir ".env")

& (Join-Path $RepoDir "dist\image2-mcp.exe")
exit $LASTEXITCODE
