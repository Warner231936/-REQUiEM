@echo off
setlocal EnableExtensions EnableDelayedExpansion

title Requiem Full System Stop

echo ============================================================
echo   REQUIEM FULL SYSTEM STOP
echo   Stops: Requiem, ReQiCode daemon, llama.cpp, Ollama
echo ============================================================
echo.

echo [STEP 1] Stopping Requiem server...
taskkill /F /IM requiem.exe >NUL 2>&1
if errorlevel 1 (echo   requiem.exe not running) else (echo   requiem.exe stopped)

echo.
echo [STEP 2] Stopping ReQiCode daemon...
taskkill /F /IM spiral.exe >NUL 2>&1
if errorlevel 1 (echo   spiral.exe not running) else (echo   spiral.exe stopped)

echo.
echo [STEP 3] Stopping llama.cpp server...
taskkill /F /IM llama-server.exe >NUL 2>&1
if errorlevel 1 (echo   llama-server.exe not running) else (echo   llama-server.exe stopped)
taskkill /F /IM llama.exe >NUL 2>&1

echo.
echo [STEP 4] Stopping Ollama...
taskkill /F /IM ollama.exe >NUL 2>&1
if errorlevel 1 (echo   ollama.exe not running) else (echo   ollama.exe stopped)

echo.
echo ============================================================
echo   All services stopped.
echo ============================================================
endlocal
