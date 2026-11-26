@echo off
setlocal
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0uninstall-windows.ps1"
if errorlevel 1 goto failed
echo.
echo Nexora was removed. Settings and credentials were kept unless you chose to delete them.
pause
exit /b 0

:failed
echo.
echo Uninstall failed. Read the message above and close any running Nexora service before retrying.
pause
exit /b 1
