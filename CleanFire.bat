@echo off
setlocal EnableExtensions EnableDelayedExpansion

title Requiem Cleanup / Clean Boot

:CHECK
cls
echo ==========================================
echo   REQUIEM PROCESS / PORT CLEANUP
echo ==========================================
echo.
echo Checking for requiem.exe and port 15153...
echo.

set "FOUND=0"

REM ---- Find and kill requiem.exe ----
for /f "tokens=2 delims=," %%P in ('tasklist /FI "IMAGENAME eq requiem.exe" /FO CSV /NH 2^>nul') do (
    set "FOUND=1"
    echo [FOUND] requiem.exe - PID %%~P
    echo [KILL ] PID %%~P
    taskkill /F /PID %%~P >nul 2>&1
)

REM ---- Find processes using TCP/UDP port 15153 ----
for /f "tokens=1,2,3,4,5" %%A in ('netstat -ano -p TCP 2^>nul ^| findstr ":15153"') do (
    if "%%E" NEQ "" (
        set "FOUND=1"
        echo [FOUND] TCP port 15153 - PID %%E
        echo [KILL ] PID %%E
        taskkill /F /PID %%E >nul 2>&1
    )
)

for /f "tokens=1,2,3,4,5" %%A in ('netstat -ano -p UDP 2^>nul ^| findstr ":15153"') do (
    if "%%E" NEQ "" (
        set "FOUND=1"
        echo [FOUND] UDP port 15153 - PID %%E
        echo [KILL ] PID %%E
        taskkill /F /PID %%E >nul 2>&1
    )
)

echo.
echo Rechecking...
timeout /t 2 /nobreak >nul

REM ---- Verify requiem.exe is gone ----
tasklist /FI "IMAGENAME eq requiem.exe" 2>nul | find /I "requiem.exe" >nul
if not errorlevel 1 (
    echo [STILL RUNNING] requiem.exe
    goto CHECK
)

REM ---- Verify port 15153 is clear ----
netstat -ano -p TCP 2>nul | findstr ":15153" >nul
if not errorlevel 1 (
    echo [STILL ACTIVE] TCP port 15153
    goto CHECK
)

netstat -ano -p UDP 2>nul | findstr ":15153" >nul
if not errorlevel 1 (
    echo [STILL ACTIVE] UDP port 15153
    goto CHECK
)

echo.
echo ==========================================
echo   CLEAN
echo ==========================================
echo.
echo requiem.exe not found.
echo Port 15153 is clear.
echo.
echo Starting normal operation...
echo.

REM Put your normal startup command here.
REM Example:
REM start "" "D:\AI\Requiem\requiem.exe"

echo Clean boot confirmed.
timeout -t 5 >nul 
start "" "D:\AI\Requiem\requiem.exe"
exit /b 0