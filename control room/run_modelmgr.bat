@echo off
REM Requiem Model Manager - Manual Runner
REM Runs the model manager CLI with common commands

setlocal enabledelayedexpansion

REM Find Ollama in PATH
set OLLAMA_PATH=C:\Users\%USERNAME%\AppData\Local\Programs\Ollama
if not exist "%OLLAMA_PATH%\ollama.exe" (
    set OLLAMA_PATH=C:\Program Files\Ollama
)
set PATH=%PATH%;%OLLAMA_PATH%

REM Model Manager executable
set MODEL_MGR=D:\AI\Requiem\modelmgr.exe
if not exist "%MODEL_MGR%" (
    echo Building model manager...
    cd /d D:\AI\Requiem
    go build -o modelmgr.exe ./cmd/modelmgr
    if errorlevel 1 (
        echo Build failed!
        pause
        exit /b 1
    )
)

echo ==========================================
echo Requiem Model Manager - Manual Control
echo ==========================================
echo.
echo Available commands:
echo   1) detect      - Detect hardware and Ollama runtime
echo   2) list        - List all registered models
echo   3) active      - Show active model for tier (default: general)
echo   4) bootstrap   - Acquire all three tiers (fast, general, high)
echo   5) recover     - Scan and repair failed/missing models
echo   6) test ^<id^>   - Run health test on a model
echo   7) activate ^<tier^> ^<id^> - Activate a model for a tier
echo   8) request     - Submit acquire_model request (JSON from stdin)
echo   9) custom      - Run arbitrary modelmgr command
echo   0) exit
echo.

:menu
set /p choice="Select option [0-9]: "
if "%choice%"=="" goto menu

if "%choice%"=="0" exit /b 0
if "%choice%"=="1" (
    echo.
    %MODEL_MGR% detect -json
    echo.
    goto menu
)
if "%choice%"=="2" (
    echo.
    %MODEL_MGR% list -json
    echo.
    goto menu
)
if "%choice%"=="3" (
    set /p tier="Enter tier (fast/general/high) [general]: "
    if "%tier%"=="" set tier=general
    echo.
    %MODEL_MGR% active %tier% -json
    echo.
    goto menu
)
if "%choice%"=="4" (
    echo.
    echo Starting bootstrap (this may take a while for downloads)...
    echo Use -parallel for concurrent tier acquisition
    set /p extra="Extra flags (e.g., -parallel -no-activate) []: "
    %MODEL_MGR% bootstrap %extra% -json -timeout 30m
    echo.
    goto menu
)
if "%choice%"=="5" (
    echo.
    echo Running recovery scan...
    %MODEL_MGR% recover -json -timeout 10m
    echo.
    goto menu
)
if "%choice%"=="6" (
    set /p model="Enter model ID: "
    if "%model%"=="" (
        echo Model ID required!
        goto menu
    )
    echo.
    %MODEL_MGR% test %model% -json
    echo.
    goto menu
)
if "%choice%"=="7" (
    set /p tier="Enter tier (fast/general/high): "
    set /p model="Enter model ID: "
    if "%tier%"=="" -o "%model%"=="" (
        echo Both tier and model ID required!
        goto menu
    )
    echo.
    %MODEL_MGR% activate %tier% %model% -json
    echo.
    goto menu
)
if "%choice%"=="8" (
    echo.
    echo Enter JSON request (Ctrl+Z then Enter to submit):
    echo Example: {"action":"acquire_model","capability":"software_engineering","tier":"high","reason":"complex task"}
    %MODEL_MGR% request -json
    echo.
    goto menu
)
if "%choice%"=="9" (
    set /p cmd="Enter modelmgr command (e.g., list -json): "
    if "%cmd%"=="" goto menu
    echo.
    %MODEL_MGR% %cmd%
    echo.
    goto menu
)

echo Invalid option.
goto menu