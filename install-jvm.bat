@echo off
setlocal DisableDelayedExpansion
rem 独立双击入口：保留成功和失败结果，命令行入口 setup-jvm.bat 不暂停。
if not exist "%~dp0quick-setup.ps1" (
    echo Error: quick-setup.ps1 not found beside this installer.
    pause
    exit /b 1
)
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0quick-setup.ps1" %*
set "jvmInstallExit=%errorlevel%"
if "%jvmInstallExit%"=="0" (
    echo.
    echo JVM installation completed. Open PowerShell to start using jvm.
) else (
    echo.
    echo JVM installation failed. Review the error above before retrying.
)
pause
exit /b %jvmInstallExit%
