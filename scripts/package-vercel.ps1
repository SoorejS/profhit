param([string]$Destination)
$ErrorActionPreference = 'Stop'
$sourceRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
if (!$Destination) { $Destination = Join-Path $sourceRoot ('audit-artifacts/vercel-package-' + [guid]::NewGuid().ToString('N')) }
$packageRoot = [System.IO.Path]::GetFullPath($Destination)
if (!$packageRoot.StartsWith($sourceRoot + [System.IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Package must be inside this workspace' }
if (Test-Path -LiteralPath $packageRoot) { throw 'Use a fresh package directory to avoid stale deployment files' }
$deploymentFiles = git -C $sourceRoot ls-files --cached --others --exclude-standard
if ($LASTEXITCODE -ne 0) { throw 'Could not enumerate repository source' }
$selected = $deploymentFiles | Sort-Object -Unique | Where-Object {
    $_ -match '^([^/]+\.html|vercel\.json|Dockerfile\.vercel|\.dockerignore|css/.+|js/.+|assets/.+|backend/.+\.(go|mod|sum))$' -and $_ -notmatch '(_test\.go$|^backend/tests/)'
}
foreach ($relative in $selected) {
    $target = Join-Path $packageRoot $relative
    New-Item -ItemType Directory -Path (Split-Path $target) -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $sourceRoot $relative) -Destination $target
}
New-Item -ItemType Directory -Path (Join-Path $packageRoot '.vercel') -Force | Out-Null
Copy-Item -LiteralPath (Join-Path $sourceRoot '.vercel/project.json') -Destination (Join-Path $packageRoot '.vercel/project.json')
Write-Output $packageRoot
