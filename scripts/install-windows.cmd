@echo off
setlocal
set "SCRIPT=%~dp0install-windows.ps1"
set "SOURCE=%~dp0Nexora.exe"
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT%" -Source "%SOURCE%"
if errorlevel 1 goto failed
echo.
echo Installation completed. Check the messages above for the Start Menu shortcut status and direct launch path.
pause
exit /b 0

:failed
echo.
echo Installation failed. Read the message above and close any running Nexora service before retrying.
pause
exit /b 1
