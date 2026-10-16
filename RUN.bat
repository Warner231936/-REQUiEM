@echo off
cd /d D:\AI\Requiem

REM Set environment variables
set REQUIEM_HOME=D:\AI\Requiem
set REQUIEM_MASTER_KEY=199031aed769e5d3fa4bb6aa112b2e15107fd796f958650b0bef66c9eb042bcb
set REQUIEM_SECURE_COOKIES=false

REM Start the server
echo Starting Requiem cognitive server on port 15153...
requiem.exe

pause
