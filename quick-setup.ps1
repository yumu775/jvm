param(
    [string]$InstallPath,
    [switch]$Force = $true,
    [switch]$NoProfile
)
$ErrorActionPreference = 'Stop'
# 使用参数数组传递路径，所有持久配置由工具统一处理。
$jvmExe = Join-Path $PSScriptRoot 'jvm.exe'
if (-not (Test-Path -LiteralPath $jvmExe -PathType Leaf)) {
    throw 'jvm.exe not found beside this script.'
}
$targetDirectory = $PSScriptRoot
$setupArguments = @('setup')
if ($InstallPath) {
    $targetDirectory = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($InstallPath)
    $setupArguments += @('--path', $targetDirectory)
}
if ($Force) { $setupArguments += '--force' }
if (-not $NoProfile) {
    $profilePath = $PROFILE.CurrentUserAllHosts
    if (-not $profilePath) { throw 'PowerShell CurrentUserAllHosts profile path could not be determined.' }
    $setupArguments += @('--powershell-profile', $profilePath)
}

& $jvmExe @setupArguments
if ($LASTEXITCODE -ne 0) { throw "JVM setup failed (exit code $LASTEXITCODE)." }

$installedExe = Join-Path $targetDirectory 'jvm.exe'
if (-not (Test-Path -LiteralPath $installedExe -PathType Leaf)) {
    throw "Setup returned success but the installed executable is missing: $installedExe"
}

# 仅在配置成功后刷新调用此脚本的 PowerShell 环境，不合并旧的机器 PATH。
$normalizedTarget = $targetDirectory.TrimEnd([char[]]'\/')
$remainingPath = @($env:PATH -split ';' | Where-Object {
    $_ -and -not [string]::Equals($_.Trim().Trim('"').TrimEnd([char[]]'\/'), $normalizedTarget, [StringComparison]::OrdinalIgnoreCase)
})
$env:PATH = (@($targetDirectory) + $remainingPath) -join ';'
$integration = & $installedExe init powershell | Out-String
if ($LASTEXITCODE -ne 0) { throw "PowerShell integration generation failed (exit code $LASTEXITCODE)." }
if ([string]::IsNullOrWhiteSpace($integration)) { throw 'PowerShell integration was empty.' }
Invoke-Expression $integration
if (-not (Get-Command jvm -CommandType Function -ErrorAction SilentlyContinue)) {
    throw 'PowerShell integration did not define the jvm function.'
}
jvm --version
if ($LASTEXITCODE -ne 0) { throw "Installed JVM verification failed (exit code $LASTEXITCODE)." }
Write-Host "JVM is ready in this PowerShell session: $installedExe"
if ($NoProfile) {
    Write-Host 'Profile integration skipped. This session is ready; future sessions can load jvm init powershell.'
} else {
    Write-Host "Future PowerShell sessions will load JVM integration from: $profilePath"
}
