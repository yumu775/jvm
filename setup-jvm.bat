@echo off
REM JVM Tool One-Click Setup
REM This batch file automatically configures JVM tool environment

echo === JVM Tool One-Click Setup ===
echo.

REM Check if jvm.exe exists
if not exist "jvm.exe" (
    echo Error: jvm.exe not found in current directory
    echo Please run this script from the directory containing jvm.exe
    pause
    exit /b 1
)

echo Found JVM tool: %CD%\jvm.exe

REM Check if already configured
echo %PATH% | findstr /i "%CD%" >nul
if %errorlevel% == 0 (
    echo JVM tool is already in PATH!
    echo Test with: jvm --version
    pause
    exit /b 0
)

echo Configuring JVM tool environment...

REM Run JVM setup command
jvm.exe setup --force
if %errorlevel% == 0 (
    echo Setup completed successfully!
) else (
    echo Setup command failed, trying alternative method...
    
    REM Alternative: Add to user PATH via registry
    echo Adding to user PATH environment variable...
    
    REM Get current user PATH
    for /f "tokens=2*" %%a in ('reg query "HKCU\Environment" /v PATH 2^>nul') do set "userpath=%%b"
    
    REM Check if already in user PATH
    echo %userpath% | findstr /i "%CD%" >nul
    if %errorlevel% neq 0 (
        REM Add to user PATH
        if defined userpath (
            reg add "HKCU\Environment" /v PATH /t REG_EXPAND_SZ /d "%CD%;%userpath%" /f >nul
        ) else (
            reg add "HKCU\Environment" /v PATH /t REG_EXPAND_SZ /d "%CD%" /f >nul
        )
        
        if %errorlevel% == 0 (
            echo Successfully added to user PATH environment variable
        ) else (
            echo Failed to modify PATH. You may need to add manually.
        )
    ) else (
        echo Already in user PATH
    )
)

echo.
echo === Setup Complete ===
echo.
echo Next steps:
echo 1. Restart your command prompt or PowerShell
echo 2. Test with: jvm --version
echo 3. Start using: jvm list
echo.
echo Quick test (opens new command prompt):
echo   start cmd /k "jvm --version"
echo.

REM Ask if user wants to test immediately
set /p choice="Open new command prompt to test? (y/n): "
if /i "%choice%"=="y" (
    start cmd /k "jvm --version"
)

pause
