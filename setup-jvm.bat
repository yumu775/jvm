@echo off
setlocal DisableDelayedExpansion
rem 使用脚本所在目录，失败时保留真实退出码，不覆盖用户 PATH。
rem 命令行入口不暂停；双击安装请使用 install-jvm.bat。
if not exist "%~dp0jvm.exe" (
    echo Error: jvm.exe not found beside this script.
    exit /b 1
)
"%~dp0jvm.exe" setup %*
set "jvmSetupExit=%errorlevel%"
if not "%jvmSetupExit%"=="0" (
    echo JVM setup failed. See the error above.
    exit /b %jvmSetupExit%
)
echo JVM persistent PATH setup completed.
echo For immediate PowerShell activation, run quick-setup.ps1.
echo For a double-click installer with visible results, use install-jvm.bat.
exit /b 0
