@echo off
REM Requiem Firewall Setup
REM Configures Windows Firewall for Requiem services

setlocal enabledelayedexpansion

echo ==========================================
echo Requiem Firewall Configuration
echo ==========================================
echo.

REM Check for admin rights
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo ERROR: This script requires Administrator privileges.
    echo Please run as Administrator.
    pause
    exit /b 1
)

set RULE_NAME_PREFIX="Requiem"
set OLLAMA_PORT=11434
set REQUIEM_API_PORT=8080
set REQUIEM_WS_PORT=8081

echo Adding firewall rules...

REM Ollama API (local only)
netsh advfirewall firewall add rule name=%RULE_NAME_PREFIX% Ollama API dir=in action=allow protocol=TCP localport=%OLLAMA_PORT% profile=private,domain description="Ollama LLM API (local network only)" >nul 2>&1
if errorlevel 1 echo Warning: Ollama API rule may already exist.

REM Requiem REST API (local network)
netsh advfirewall firewall add rule name=%RULE_NAME_PREFIX% REST API dir=in action=allow protocol=TCP localport=%REQUIEM_API_PORT% profile=private,domain description="Requiem Cognitive Architecture REST API" >nul 2>&1
if errorlevel 1 echo Warning: REST API rule may already exist.

REM Requiem WebSocket (local network)
netsh advfirewall firewall add rule name=%RULE_NAME_PREFIX% WebSocket dir=in action=allow protocol=TCP localport=%REQUIEM_WS_PORT% profile=private,domain description="Requiem WebSocket Interface" >nul 2>&1
if errorlevel 1 echo Warning: WebSocket rule may already exist.

REM Outbound for model downloads (HuggingFace, Ollama registry)
netsh advfirewall firewall add rule name=%RULE_NAME_PREFIX% Model Downloads dir=out action=allow protocol=TCP remoteport=80,443 profile=private,domain description="Outbound model downloads (HuggingFace, Ollama)" >nul 2>&1
if errorlevel 1 echo Warning: Outbound rule may already exist.

REM Optional: Allow Ollama executable full access
set OLLAMA_EXE=C:\Users\%USERNAME%\AppData\Local\Programs\Ollama\ollama.exe
if not exist "%OLLAMA_EXE%" set OLLAMA_EXE=C:\Program Files\Ollama\ollama.exe
if exist "%OLLAMA_EXE%" (
    netsh advfirewall firewall add rule name=%RULE_NAME_PREFIX% Ollama Executable dir=in action=allow program="%OLLAMA_EXE%" profile=private,domain description="Ollama LLM Runtime" >nul 2>&1
    netsh advfirewall firewall add rule name=%RULE_NAME_PREFIX% Ollama Executable Out dir=out action=allow program="%OLLAMA_EXE%" profile=private,domain description="Ollama LLM Runtime Outbound" >nul 2>&1
    if errorlevel 1 echo Warning: Ollama executable rules may already exist.
)

REM Optional: Allow Requiem executable
set REQUIEM_EXE=D:\AI\Requiem\requiem.exe
if exist "%REQUIEM_EXE%" (
    netsh advfirewall firewall add rule name=%RULE_NAME_PREFIX% Requiem Executable dir=in action=allow program="%REQUIEM_EXE%" profile=private,domain description="Requiem Cognitive Architecture" >nul 2>&1
    netsh advfirewall firewall add rule name=%RULE_NAME_PREFIX% Requiem Executable Out dir=out action=allow program="%REQUIEM_EXE%" profile=private,domain description="Requiem Cognitive Architecture Outbound" >nul 2>&1
    if errorlevel 1 echo Warning: Requiem executable rules may already exist.
)

echo.
echo Firewall rules configured successfully.
echo.
echo Rules added:
echo   - %RULE_NAME_PREFIX% Ollama API (port %OLLAMA_PORT%)
echo   - %RULE_NAME_PREFIX% REST API (port %REQUIEM_API_PORT%)
echo   - %RULE_NAME_PREFIX% WebSocket (port %REQUIEM_WS_PORT%)
echo   - %RULE_NAME_PREFIX% Model Downloads (outbound 80/443)
if exist "%OLLAMA_EXE%" echo   - %RULE_NAME_PREFIX% Ollama Executable
if exist "%REQUIEM_EXE%" echo   - %RULE_NAME_PREFIX% Requiem Executable
echo.
echo To verify rules: netsh advfirewall firewall show rule name=all | findstr /i "Requiem"
echo.

pause