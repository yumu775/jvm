[CmdletBinding()]
param([switch]$Setup)

$ErrorActionPreference = 'Stop'
$projectRoot = $PSScriptRoot
$buildDirectory = Join-Path $projectRoot 'build'
$buildExecutable = Join-Path $buildDirectory 'jvm.exe'
$rootExecutable = Join-Path $projectRoot 'jvm.exe'
$token = [Guid]::NewGuid().ToString('N')
$buildTemporary = Join-Path $buildDirectory ".jvm-build-$token.exe"
$rootTemporary = Join-Path $projectRoot ".jvm-publish-$token.exe"

# 临时文件与目标位于同一目录；替换失败时不先删除旧程序。
function Publish-JvmExecutable {
    param([string]$Source, [string]$Destination)
    if (Test-Path -LiteralPath $Destination) {
        $existing = Get-Item -LiteralPath $Destination -Force
        if ($existing.PSIsContainer -or ($existing.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            throw "Refusing to replace a directory or linked executable: $Destination"
        }
        [IO.File]::Replace($Source, $Destination, [NullString]::Value)
    } else {
        [IO.File]::Move($Source, $Destination)
    }
}

Push-Location -LiteralPath $projectRoot
try {
    $targetOS = & go env GOOS
    if ($LASTEXITCODE -ne 0) { throw 'Could not read the Go target platform.' }
    if (($targetOS -join '').Trim() -ne 'windows') {
        throw 'build.ps1 publishes the Windows executable. Use go build directly for cross-compilation.'
    }
    if (Test-Path -LiteralPath $buildDirectory) {
        $directory = Get-Item -LiteralPath $buildDirectory -Force
        if (-not $directory.PSIsContainer -or ($directory.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            throw "Build directory must be a real directory: $buildDirectory"
        }
    } else {
        [IO.Directory]::CreateDirectory($buildDirectory) | Out-Null
    }

    & go build -trimpath -o $buildTemporary .
    if ($LASTEXITCODE -ne 0) {
        throw 'Go build failed. Existing jvm.exe files have not been replaced.'
    }
    if (-not (Test-Path -LiteralPath $buildTemporary -PathType Leaf)) {
        throw 'Go build produced no executable. Existing jvm.exe files have not been replaced.'
    }

    Publish-JvmExecutable -Source $buildTemporary -Destination $buildExecutable
    [IO.File]::Copy($buildExecutable, $rootTemporary, $false)
    try {
        Publish-JvmExecutable -Source $rootTemporary -Destination $rootExecutable
    } catch {
        throw "Built $buildExecutable, but could not replace $rootExecutable. Close any running root executable and retry. $($_.Exception.Message)"
    }
    Write-Host "Built: $buildExecutable"
    Write-Host "Updated: $rootExecutable"

    if ($Setup) {
        $setupScript = Join-Path $projectRoot 'quick-setup.ps1'
        if (-not (Test-Path -LiteralPath $setupScript -PathType Leaf)) {
            throw "Build succeeded, but setup script was not found: $setupScript"
        }
        & $setupScript
        if ($LASTEXITCODE -ne 0) { throw 'Build succeeded, but environment setup failed.' }
    } else {
        Write-Host 'Environment settings were not changed. Run .\quick-setup.ps1 to configure the tool.'
    }
} finally {
    foreach ($temporary in @($buildTemporary, $rootTemporary)) {
        if (Test-Path -LiteralPath $temporary -PathType Leaf) {
            Remove-Item -LiteralPath $temporary -Force -ErrorAction SilentlyContinue
        }
    }
    Pop-Location
}
