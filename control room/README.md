# Requiem Control Room

Administrative scripts for managing the Requiem Model Manager and server.

## Scripts

### `run_modelmgr.bat`
Interactive menu for model management operations.

**Usage:**
```cmd
control room\run_modelmgr.bat
```

**Features:**
- Hardware detection
- Model listing (all / by tier)
- Active model查看
- Three-tier bootstrap (with parallel option)
- Recovery scan & repair
- Model health testing
- Model activation
- Custom model requests

### `setup_firewall.bat`
Configures Windows Firewall for Requiem services. **Must run as Administrator.**

**Usage:**
```cmd
# Right-click → Run as Administrator
control room\setup_firewall.bat
```

**Configures:**
- Ollama API (port 11434) - local network
- Requiem REST API (port 8080) - local network
- Requiem WebSocket (port 8081) - local network
- Outbound model downloads (ports 80/443)
- Ollama executable access
- Requiem executable access (if built)

### `manage_server.bat`
Checks if Requiem server is running, gracefully stops it if so, then (re)launches.

**Usage:**
```cmd
control room\manage_server.bat
```

**Behavior:**
1. Ensures Ollama is running (starts `ollama serve` if needed)
2. Checks for existing `requiem.exe` process
3. Gracefully stops it (15s timeout, then force kill)
3. Builds Requiem if executable missing
4. Launches Requiem with logging to `logs/requiem.log`
5. Verifies startup

## Quick Start

```cmd
# 1. Set up firewall (run as Admin once)
control room\setup_firewall.bat

# 2. Start/Restart server
control room\manage_server.bat

# 3. Manage models interactively
control room\run_modelmgr.bat
```

## Prerequisites

- **Ollama** installed and in PATH (or at `%LOCALAPPDATA%\Programs\Ollama`)
- **Go 1.21+** for building (if executables missing)
- **Administrator** rights for firewall script
- **Windows 10/11** with PowerShell

## Logs

- Model manager logs: JSON to stdout/stderr
- Server logs: `D:\AI\Requiem\logs\requiem.log`
- Model registry: `D:\AI\Requiem\.requiem\registry.json`
- Model manifests: `D:\AI\Requiem\.requiem\manifests\<id>\manifest.json`

## Model Tiers

| Tier | Model | Use Case |
|------|-------|----------|
| Fast | `llama3.2:3b` | Routing, classification, quick responses |
| General | `llama3.1:8b` | Coding, analysis, primary reasoning |
| High | `qwen2.5:32b` | Complex reasoning, architecture, debugging |

## Common Operations

**Bootstrap all tiers:**
```cmd
control room\run_modelmgr.bat
> 4  (bootstrap)
> -parallel  (optional, for concurrent downloads)
```

**Recover failed models:**
```cmd
control room\run_modelmgr.bat
> 5  (recover)
```

**Test a model:**
```cmd
control room\run_modelmgr.bat
> 6  (test)
> llama3.1-8b-general  (model ID)
```

**Request model from Requiem:**
```cmd
control room\run_modelmgr.bat
> 8  (request)
> {"action":"acquire_model","capability":"software_engineering","tier":"high","reason":"complex refactoring"}
```

## Troubleshooting

| Problem | Solution |
|---------|----------|
| "Access denied" on firewall | Run as Administrator |
| Ollama not found | Install Ollama, restart terminal |
| Model download fails | Check internet, update catalog URLs |
| Server won't start | Check `logs/requiem.log` for errors |
| Port already in use | Stop existing process via Task Manager |