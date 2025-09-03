# JVM Tool Quick Setup Script
# Simple script to configure JVM tool environment

Write-Host "=== JVM Tool Quick Setup ===" -ForegroundColor Blue
Write-Host ""

# Check if jvm.exe exists
$jvmExe = ".\jvm.exe"
if (-not (Test-Path $jvmExe)) {
    Write-Host "Error: jvm.exe not found in current directory" -ForegroundColor Red
    Write-Host "Please run this script from the directory containing jvm.exe" -ForegroundColor Yellow
    exit 1
}

Write-Host "Found JVM tool: $(Get-Location)\jvm.exe" -ForegroundColor Green

# Check if already in PATH
$currentDir = Get-Location
$currentPath = $env:PATH
if ($currentPath -like "*$currentDir*") {
    Write-Host "JVM tool is already in PATH!" -ForegroundColor Green
    Write-Host "Test with: jvm --version" -ForegroundColor Cyan
    exit 0
}

Write-Host "Configuring JVM tool environment..." -ForegroundColor Blue

# Run JVM setup command
try {
    & $jvmExe setup --force
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Setup completed successfully!" -ForegroundColor Green
    } else {
        Write-Host "Setup command failed, trying manual configuration..." -ForegroundColor Yellow
        
        # Manual PowerShell profile configuration
        $profilePath = $PROFILE
        if (-not $profilePath) {
            $profilePath = "$env:USERPROFILE\Documents\WindowsPowerShell\Microsoft.PowerShell_profile.ps1"
        }
        
        # Create profile directory if it doesn't exist
        $profileDir = Split-Path $profilePath -Parent
        if (-not (Test-Path $profileDir)) {
            New-Item -ItemType Directory -Path $profileDir -Force | Out-Null
        }
        
        # Add to profile
        $pathLine = "`$env:PATH = `"$currentDir;`" + `$env:PATH"
        
        if (Test-Path $profilePath) {
            $content = Get-Content $profilePath -Raw -ErrorAction SilentlyContinue
            if ($content -notlike "*JVM Tool PATH*") {
                Add-Content $profilePath "`n# JVM Tool PATH Configuration`n$pathLine`n"
                Write-Host "Added to PowerShell profile: $profilePath" -ForegroundColor Green
            } else {
                Write-Host "Already configured in PowerShell profile" -ForegroundColor Green
            }
        } else {
            Set-Content $profilePath "# JVM Tool PATH Configuration`n$pathLine`n"
            Write-Host "Created PowerShell profile: $profilePath" -ForegroundColor Green
        }
    }
} catch {
    Write-Host "Error running setup: $_" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "=== Next Steps ===" -ForegroundColor Blue
Write-Host "1. Restart PowerShell or run: . `$PROFILE" -ForegroundColor Cyan
Write-Host "2. Test with: jvm --version" -ForegroundColor Cyan
Write-Host "3. Start using: jvm list" -ForegroundColor Cyan
Write-Host ""
Write-Host "Quick test in new PowerShell window:" -ForegroundColor Yellow
Write-Host "  Start-Process powershell -ArgumentList '-NoExit', '-Command', 'jvm --version'" -ForegroundColor White
