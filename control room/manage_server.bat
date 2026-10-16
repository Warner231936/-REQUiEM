@echo off
REM Requiem Server Manager
REM Checks if Requiem server is running, stops it if so, then (re)launches

setlocal enabledelayedexpansion

REM Configuration
set REQUIEM_DIR=D:\AI\Requiem
set REQUIEM_EXE=%REQUIEM_DIR%\requiem.exe
set CONFIG_FILE=%REQUIEM_DIR%\config\config.yaml
set LOG_FILE=%REQUIEM_DIR%\logs\requiem.log
set PID_FILE=%REQUIEM_DIR%\requiem.pid
set OLLAMA_PATH=C:\Users\%USERNAME%\AppData\Local\Programs\Ollama

REM Add Ollama to PATH
if exist "%OLLAMA_PATH%\ollama.exe" set PATH=%PATH%;%OLLAMA_PATH%

echo ==========================================
echo Requiem Server Manager
echo ==========================================
echo.

REM Function to check if process is running
:check_process
set PROC_NAME=%~1
tasklist /FI "IMAGENAME eq %PROC_NAME%" 2>NUL | find /I /N "%PROC_NAME%">NUL
if "%ERRORLEVEL%"=="0" (
    exit /b 0
) else (
    exit /b 1
)

REM Function to get PID of process
:get_pid
set PROC_NAME=%~1
for /f "tokens=2" %%i in ('tasklist /FI "IMAGENAME eq %PROC_NAME%" /NH /FO CSV 2^>NUL') do (
    set PID=%%~i
    exit /b 0
)
exit /b 1

REM Function to stop process gracefully
:stop_process
set PROC_NAME=%~1
set TIMEOUT_SEC=%~2
if "%TIMEOUT_SEC%"=="" set TIMEOUT_SEC=10

call :get_pid %PROC_NAME%
if defined PID (
    echo Stopping %PROC_NAME% (PID %PID%)...
    taskkill /PID %PID% >NUL 2>&1
    
    REM Wait for graceful shutdown
    set COUNT=0
    :wait_stop
    call :get_pid %PROC_NAME%
    if not defined PID (
        echo %PROC_NAME% stopped gracefully.
        exit /b 0
    )
    if %COUNT% geq %TIMEOUT_SEC% (
        echo Force killing %PROC_NAME%...
        taskkill /F /PID %PID% >NUL 2>&1
        timeout /t 2 >NUL
        call :get_pid %PROC_NAME%
        if defined PID (
            echo ERROR: Failed to kill %PROC_NAME%
            exit /b 1
        )
        echo %PROC_NAME% force killed.
        exit /b 0
    )
    timeout /t 1 >NUL
    set /a COUNT+=1
    goto wait_stop
) else (
    echo %PROC_NAME% not running.
    exit /b 0
)

REM Ensure log directory exists
if not exist "%REQUIEM_DIR%\logs" mkdir "%REQUIEM_DIR%\logs"

REM Check for Ollama
echo Checking Ollama...
where ollama >NUL 2>&1
if %errorlevel% neq 0 (
    echo WARNING: Ollama not in PATH. Adding default location...
    set PATH=%PATH%;%OLLAMA_PATH%
    where ollama >NUL 2>&1
    if %errorlevel% neq 0 (
        echo ERROR: Ollama not found. Please install Ollama first.
        pause
        exit /b 1
    )
)

REM Check if Ollama service is running
ollama list >NUL 2>&1
if %errorlevel% neq 0 (
    echo Starting Ollama service...
    start "" ollama serve
    echo Waiting for Ollama to start...
    timeout /t 5 >NUL
    ollama list >NUL 2>&1
    if %errorlevel% neq 0 (
        echo WARNING: Ollama may not be fully ready
    )
)

REM Check for existing Requiem process
echo Checking for existing Requiem server...
call :get_pid requiem.exe
if defined PID (
    echo Found running Requiem (PID %PID%).
    call :stop_process requiem.exe 15
    if errorlevel 1 (
        echo Failed to stop existing server.
        pause
        exit /b 1
    )
) else (
    echo No existing Requiem server found.
)

REM Verify executable exists
if not exist "%REQUIEM_EXE%" (
    echo Building Requiem...
    cd /d %REQUIEM_DIR%
    go build -o requiem.exe ./cmd/requiem
    if errorlevel 1 (
        echo Build failed!
        pause
        exit /b 1
    )
)

REM Launch Requiem
echo.
echo Launching Requiem...
cd /d %REQUIEM_DIR%

REM Start with logging
start "" cmd /c "title Requiem Server && %REQUIEM_EXE% -config %CONFIG_FILE% 2>&1 | tee %LOG_FILE%"

REM Give it a moment to start
timeout /t 3 >NUL

REM Verify it started
call :get_pid requiem.exe
if defined PID (
    echo.
    echo ==========================================
    echo Requiem server started successfully!
    echo PID: %PID%
    echo Log: %LOG_FILE%
    echo Config: %CONFIG_FILE%
    echo ==========================================
    echo.
    echo Server is running in background.
    echo Use 'tasklist /FI "IMAGENAME eq requiem.exe"' to verify.
    echo Use this script again to restart.
else (
    echo.
    echo ERROR: Requiem failed to start. Check log: %LOG_FILE%
    exit /b 1
)

echo.
echo Press any key to exit (server continues running)...
pause >NUL