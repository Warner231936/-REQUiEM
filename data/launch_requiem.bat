@echo off
cd /d D:\AI\Requiem
set REQUiEM_AUTH_DISABLED=true
set MONGODB_DATABASE=requiem-integration-test
set REQICODE_DAEMON_URL=http://localhost:8080
set OLLAMA_HOST=127.0.0.1
start "requiem" "D:\AI\Requiem\data\requiem.exe"
exit
