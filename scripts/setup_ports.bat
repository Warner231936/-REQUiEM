@echo off
REM Requiem Port Configuration Setup
REM Runs the Python port configuration tool

setlocal enabledelayedexpansion

set PROJECT_ROOT=D:\AI\Requiem
set PYTHON=python

REM Check if Python is available
where %PYTHON% >nul 2>&1
if %errorlevel% neq 0 (
    echo Python not found in PATH. Please install Python 3.
    pause
    exit /b 1
)

cd /d %PROJECT_ROOT%

echo ==========================================
echo Requiem Port Configuration Setup
echo ==========================================
echo.

if "%~1"=="" (
    echo Running in automatic mode...
    %PYTHON% scripts/setup_ports.py --non-interactive
) else (
    %PYTHON% scripts/setup_ports.py %*
)

echo.
pause