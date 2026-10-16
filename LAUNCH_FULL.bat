@echo off
setlocal EnableExtensions EnableDelayedExpansion

title Requiem Full System Launcher

set "REQUIEM_HOME=D:\AI\Requiem"
set "REQUIEM_MASTER_KEY=199031aed769e5d3fa4bb6aa112b2e15107fd796f958650b0bef66c9eb042bcb"
set "REQUIEM_SECURE_COOKIES=false"
set "REQICODE_DAEMON_URL=http://localhost:8080"
set "REQICODE_HOME=D:\AI\Requiem\ReqiCode"
set "REQICODE_OUTPUT=D:\AI\Requiem\ReqiCode\spiral-output-daemon"

cd /d "%REQUIEM_HOME%"

echo ============================================================
echo   REQUIEM FULL SYSTEM LAUNCHER
echo   Starts: MongoDB, Ollama, ReQiCode llama.cpp, ReQiCode daemon, Requiem
echo ============================================================
echo.

REM === Step 0: Kill old processes ===
echo [STEP 0] Cleaning up old processes...
echo.

REM Kill requiem.exe
tasklist /FI "IMAGENAME eq requiem.exe" 2>NUL | find /I "requiem.exe" >NUL
if not errorlevel 1 (
    echo   Stopping requiem.exe...
    taskkill /F /IM requiem.exe >NUL 2>&1
    timeout /t 1 /nobreak >NUL
)

REM Kill spiral.exe (ReQiCode daemon)
tasklist /FI "IMAGENAME eq spiral.exe" 2>NUL | find /I "spiral.exe" >NUL
if not errorlevel 1 (
    echo   Stopping spiral.exe...
    taskkill /F /IM spiral.exe >NUL 2>&1
    timeout /t 1 /nobreak >NUL
)

REM Kill llama-server.exe (ReQiCode's llama.cpp)
tasklist /FI "IMAGENAME eq llama-server.exe" 2>NUL | find /I "llama-server.exe" >NUL
if not errorlevel 1 (
    echo   Stopping llama-server.exe...
    taskkill /F /IM llama-server.exe >NUL 2>&1
    timeout /t 1 /nobreak >NUL
)

REM Kill older llama-server variants
tasklist /FI "IMAGENAME eq llama.exe" 2>NUL | find /I "llama.exe" >NUL
if not errorlevel 1 (
    echo   Stopping llama.exe...
    taskkill /F /IM llama.exe >NUL 2>&1
    timeout /t 1 /nobreak >NUL
)

echo   Cleanup complete.
echo.

REM === Step 1: Start MongoDB ===
echo [STEP 1] Checking MongoDB service...
echo.

sc query MongoDB >NUL 2>&1
if errorlevel 1060 (
    echo   MongoDB service not found. Attempting to start via command...
    where mongod >NUL 2>&1
    if errorlevel 1 (
        echo   ERROR: mongod not found on PATH and MongoDB service not installed.
        echo   Please install MongoDB or start it manually on port 27017.
        echo.
        goto :SKIP_MONGODB
    )
    start "" mongod --port 27017 --dbpath "%REQUIEM_HOME%\data\db"
    echo   MongoDB started as process.
) else (
    sc query MongoDB | find "RUNNING" >NUL
    if errorlevel 1 (
        echo   Starting MongoDB service...
        net start MongoDB >NUL 2>&1
        timeout /t 3 /nobreak >NUL
    ) else (
        echo   MongoDB service is already running.
    )
)

:SKIP_MONGODB
echo.

REM === Step 2: Start Ollama ===
echo [STEP 2] Checking Ollama...
echo.

set "OLLAMA_EXE="
where ollama >NUL 2>&1
if errorlevel 1 (
    if exist "C:\Users\m\AppData\Local\Programs\Ollama\ollama.exe" (
        set "OLLAMA_EXE=C:\Users\m\AppData\Local\Programs\Ollama\ollama.exe"
    ) else (
        echo   WARNING: Ollama not found. Requiem pipeline LLM will not be available.
        echo   Install Ollama from https://ollama.com/download
        goto :SKIP_OLLAMA
    )
) else (
    for /f "delims=" %%i in ('where ollama') do set "OLLAMA_EXE=%%i"
)

REM Check if Ollama is already running
powershell -Command "try { $c = New-Object System.Net.Sockets.TcpClient; $c.Connect('127.0.0.1',11434); $c.Close(); exit 0 } catch { exit 1 }" >NUL 2>&1
if not errorlevel 1 (
    echo   Ollama is already running on port 11434.
) else (
    echo   Starting Ollama server...
    start "Ollama" "" "!OLLAMA_EXE!" serve
    timeout /t 3 /nobreak >NUL
    REM Wait for Ollama to be ready
    set "OLLAMA_READY=0"
    for /l %%i in (1,1,10) do (
        powershell -Command "try { $c = New-Object System.Net.Sockets.TcpClient; $c.Connect('127.0.0.1',11434); $c.Close(); exit 0 } catch { exit 1 }" >NUL 2>&1
        if not errorlevel 1 (
            set "OLLAMA_READY=1"
            goto :OLLAMA_DONE
        )
        timeout /t 2 /nobreak >NUL
    )
    :OLLAMA_DONE
    if "!OLLAMA_READY!"=="1" (
        echo   Ollama is ready.
    ) else (
        echo   WARNING: Ollama did not become ready in time. Some features may not work.
    )
)

:SKIP_OLLAMA
echo.

REM === Step 3: Start ReQiCode llama.cpp server ===
echo [STEP 3] Starting ReQiCode llama.cpp server (port 8090)...
echo.

set "LLAMA_SERVER=%REQICODE_HOME%\third_party\llama-cpp-9b\llama-server.exe"
set "LLAMA_MODEL=Qwen2.5-Coder-14B-Instruct-Q4_K_M.gguf"

if not exist "!LLAMA_SERVER!" (
    echo   llama-server.exe not found at !LLAMA_SERVER!
    echo   ReQiCode will use mock/template mode only (no LLM inference).
    goto :SKIP_LLAMA
)

REM Check if already running
powershell -Command "try { $c = New-Object System.Net.Sockets.TcpClient; $c.Connect('127.0.0.1',8090); $c.Close(); exit 0 } catch { exit 1 }" >NUL 2>&1
if not errorlevel 1 (
    echo   llama.cpp server is already running on port 8090.
    goto :LLAMA_DONE
)

set "MODEL_PATH=%REQICODE_HOME%\models-cache\!LLAMA_MODEL!"
if not exist "!MODEL_PATH!" (
    echo   Model not found at !MODEL_PATH!
    echo   Trying 7B model instead...
    set "LLAMA_MODEL=Qwen2.5-7B-Instruct-Q4_K_M.gguf"
    set "MODEL_PATH=%REQICODE_HOME%\models-cache\!LLAMA_MODEL!"
)

if not exist "!MODEL_PATH!" (
    echo   No models found. ReQiCode will use mock/template mode only.
    goto :SKIP_LLAMA
)

echo   Loading model: !LLAMA_MODEL! (~9GB, this may take 1-3 minutes on CPU)
echo   GPU acceleration: auto-detected

start "llama.cpp" "" "!LLAMA_SERVER!" --model "!MODEL_PATH!" --port 8090 --n-gpu-layers 999 --ctx-size 4096 --threads -1

REM Wait for llama.cpp server to be ready
set "LLAMA_READY=0"
for /l %%i in (1,1,30) do (
    powershell -Command "try { $c = New-Object System.Net.Sockets.TcpClient; $c.Connect('127.0.0.1',8090); $c.Close(); exit 0 } catch { exit 1 }" >NUL 2>&1
    if not errorlevel 1 (
        set "LLAMA_READY=1"
        goto :LLAMA_DONE
    )
    timeout /t 5 /nobreak >NUL
)

:LLAMA_DONE
if "!LLAMA_READY!"=="1" (
    echo   llama.cpp server is ready on port 8090.
) else (
    echo   WARNING: llama.cpp server did not become ready in time.
    echo   ReQiCode will fall back to mock/template mode.
)

:SKIP_LLAMA
echo.

REM === Step 4: Start ReQiCode daemon ===
echo [STEP 4] Starting ReQiCode daemon (port 8080)...
echo.

set "REQICODE_EXE=%REQICODE_HOME%\bin\spiral.exe"
if not exist "!REQICODE_EXE!" (
    echo   spiral.exe not found at !REQICODE_EXE!
    echo   Building it now...
    pushd "%REQICODE_HOME%"
    go build -o bin\spiral.exe ./cmd/spiral 2>&1
    popd
    timeout /t 2 /nobreak >NUL
)

if not exist "!REQICODE_EXE!" (
    echo   ERROR: Could not build or find spiral.exe.
    echo   ReQiCode daemon will not start. The reqicode tool will return errors.
    goto :SKIP_DAEMON
)

REM Ensure output directory and models.json exist
if not exist "!REQICODE_OUTPUT!" mkdir "!REQICODE_OUTPUT!"
if not exist "!REQICODE_OUTPUT!\models.json" (
    echo   models.json not found, copying from template...
    copy /Y "%REQICODE_HOME%\spiral-output-daemon\models.json" "!REQICODE_OUTPUT!\models.json" >NUL 2>&1
)

REM Check if daemon is already running
powershell -Command "try { $c = New-Object System.Net.Sockets.TcpClient; $c.Connect('127.0.0.1',8080); $c.Close(); exit 0 } catch { exit 1 }" >NUL 2>&1
if not errorlevel 1 (
    echo   ReQiCode daemon is already running on port 8080.
    goto :DAEMON_DONE
)

echo   Starting ReQiCode daemon...
cd /d "%REQICODE_HOME%"
start "ReQiCode Daemon" "" "!REQICODE_EXE!" daemon --port :8080 --output "!REQICODE_OUTPUT!"
cd /d "%REQUIEM_HOME%"

REM Wait for daemon
set "DAEMON_READY=0"
for /l %%i in (1,1,15) do (
    powershell -Command "try { $c = New-Object System.Net.Sockets.TcpClient; $c.Connect('127.0.0.1',8080); $c.Close(); exit 0 } catch { exit 1 }" >NUL 2>&1
    if not errorlevel 1 (
        set "DAEMON_READY=1"
        goto :DAEMON_DONE
    )
    timeout /t 2 /nobreak >NUL
)

:DAEMON_DONE
if "!DAEMON_READY!"=="1" (
    echo   ReQiCode daemon is ready on port 8080.
) else (
    echo   WARNING: ReQiCode daemon did not become ready in time.
    echo   The reqicode tool will return errors until it starts.
)

:SKIP_DAEMON
echo.

REM === Step 5: Start Requiem server ===
echo [STEP 5] Starting Requiem cognitive server (port 15153)...
echo.

REM Ensure data directory exists
if not exist "%REQUIEM_HOME%\data" mkdir "%REQUIEM_HOME%\data"

REM Check if server is already running
powershell -Command "try { $c = New-Object System.Net.Sockets.TcpClient; $c.Connect('127.0.0.1',15153); $c.Close(); exit 0 } catch { exit 1 }" >NUL 2>&1
if not errorlevel 1 (
    echo   Requiem server is already running on port 15153.
    goto :SERVER_DONE
)

echo   Starting Requiem cognitive server...
start "Requiem" "" "%REQUIEM_HOME%\requiem.exe"

REM Wait for server
set "SERVER_READY=0"
for /l %%i in (1,1,30) do (
    powershell -Command "try { $c = New-Object System.Net.Sockets.TcpClient; $c.Connect('127.0.0.1',15153); $c.Close(); exit 0 } catch { exit 1 }" >NUL 2>&1
    if not errorlevel 1 (
        set "SERVER_READY=1"
        goto :SERVER_DONE
    )
    timeout /t 2 /nobreak >NUL
)

:SERVER_DONE
if "!SERVER_READY!"=="1" (
    echo   Requiem server is ready on port 15153.
) else (
    echo   WARNING: Requiem server did not become ready in time.
    echo   Check console output for errors.
)

echo.

REM === Step 6: Final status ===
echo ============================================================
echo   SYSTEM STATUS
echo ============================================================
echo.

REM Check each service
powershell -Command "try { $c = New-Object System.Net.Sockets.TcpClient; $c.Connect('127.0.0.1',27017); $c.Close(); exit 0 } catch { exit 1 }" >NUL 2>&1
if not errorlevel 1 (echo   MongoDB:         RUNNING on port 27017) else (echo   MongoDB:         NOT RUNNING)

powershell -Command "try { $c = New-Object System.Net.Sockets.TcpClient; $c.Connect('127.0.0.1',11434); $c.Close(); exit 0 } catch { exit 1 }" >NUL 2>&1
if not errorlevel 1 (echo   Ollama:          RUNNING on port 11434) else (echo   Ollama:          NOT RUNNING)

powershell -Command "try { $c = New-Object System.Net.Sockets.TcpClient; $c.Connect('127.0.0.1',8090); $c.Close(); exit 0 } catch { exit 1 }" >NUL 2>&1
if not errorlevel 1 (echo   llama.cpp:       RUNNING on port 8090) else (echo   llama.cpp:       NOT RUNNING)

powershell -Command "try { $c = New-Object System.Net.Sockets.TcpClient; $c.Connect('127.0.0.1',8080); $c.Close(); exit 0 } catch { exit 1 }" >NUL 2>&1
if not errorlevel 1 (echo   ReQiCode daemon: RUNNING on port 8080) else (echo   ReQiCode daemon: NOT RUNNING)

powershell -Command "try { $c = New-Object System.Net.Sockets.TcpClient; $c.Connect('127.0.0.1',15153); $c.Close(); exit 0 } catch { exit 1 }" >NUL 2>&1
if not errorlevel 1 (echo   Requiem server:  RUNNING on port 15153) else (echo   Requiem server:  NOT RUNNING)

echo.
echo ============================================================
echo   QUICK LINKS
echo ============================================================
echo   Chat UI:        http://127.0.0.1:15153/chat
echo   Health check:   http://127.0.0.1:15153/health
echo   Admin portal:   http://127.0.0.1:15153/admin
echo   Dashboard:      http://127.0.0.1:15153/dashboard/
echo   ReQiCode UI:    http://localhost:8080/dashboard/
echo.
echo   API Endpoints:
echo     POST /api/chat                         - Chat (no auth)
echo     POST /api/auth/signup                  - Register user
echo     POST /api/auth/login                   - Login
echo     GET  /api/auth/me                      - Current user (with session)
echo     GET  /api/mind/beliefs                 - List beliefs (auth required)
echo.

REM Open browser to chat
echo Opening browser to chat interface...
start "" "http://127.0.0.1:15153/chat"

echo.
echo All services started. Close the respective console windows to stop services.
echo ============================================================
endlocal
