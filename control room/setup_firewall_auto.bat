@echo off
REM Auto-generated firewall rules for Requiem
REM Run as Administrator

netsh advfirewall firewall add rule name="Requiem Server" dir=in action=allow protocol=TCP localport=15153 profile=private,domain
netsh advfirewall firewall add rule name="Ollama API" dir=in action=allow protocol=TCP localport=11434 profile=private,domain
netsh advfirewall firewall add rule name="MongoDB" dir=in action=allow protocol=TCP localport=27017 profile=private,domain
netsh advfirewall firewall add rule name="Requiem Dashboard WS" dir=in action=allow protocol=TCP localport=8081 profile=private,domain
netsh advfirewall firewall add rule name="Requiem Admin Portal" dir=in action=allow protocol=TCP localport=8080 profile=private,domain
netsh advfirewall firewall add rule name="Requiem Landing Page" dir=in action=allow protocol=TCP localport=3000 profile=private,domain
netsh advfirewall firewall add rule name="Requiem Model Downloads" dir=out action=allow protocol=TCP remoteport=80,443 profile=private,domain
