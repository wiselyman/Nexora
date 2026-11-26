@echo off
setlocal
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0rollback-windows.ps1"
if errorlevel 1 goto failed
echo.
echo The previous Nexora version was restored.
pause
exit /b 0

:failed
echo.
echo Rollback failed. Read the message above and close any running Nexora service before retrying.
pause
exit /b 1
