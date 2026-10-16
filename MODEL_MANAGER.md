# Requiem Model Manager Documentation

## Overview

The Requiem Model Manager is a complete model acquisition, recovery, registry, lifecycle, and expansion subsystem for the Requiem cognitive architecture. It enables Requiem to automatically detect hardware, select appropriate models, download/verify/import them, and manage their lifecycle without manual intervention.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Requiem Cognitive Layer                   │
│                    (requests models via API)                     │
└─────────────────────────────┬───────────────────────────────────┘
                              │ ModelRequest
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Model Manager Core                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐            │
│  │ Hardware │ │ Catalog  │ │ Selector │ │ Registry │            │
│  │ Detector │ │          │ │ (70% env)│ │          │            │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘            │
│         │          │          │           │                      │
│         ▼          ▼          ▼           ▼                      │
│  ┌──────────────────────────────────────────────────────┐       │
│  │              Acquisition Pipeline                     │       │
│  │  Download → Verify → Import → Health Test → Register  │       │
│  └──────────────────────────────────────────────────────┘       │
│                              │                                   │
│                              ▼                                   │
│                    ┌─────────────────┐                          │
│                    │   Ollama Runtime │                          │
│                    └─────────────────┘                          │
└─────────────────────────────────────────────────────────────────┘
```

## Three-Model Architecture

| Tier | Purpose | Typical Size | Selection Criteria |
|------|---------|--------------|-------------------|
| **Fast** | Routing, classification, lightweight ops | 2-3B params | Sub-second latency, <3GB VRAM |
| **General** | Primary reasoning, coding, analysis | 7-14B params | Balanced capability/speed, ~70% VRAM |
| **High** | Complex reasoning, architecture, debugging | 14-32B+ params | Maximum capability within 70% envelope |

## Hardware-Aware Selection

The manager automatically detects:
- CPU model, cores, threads
- System RAM (GB)
- GPU model, VRAM (GB) via nvidia-smi / wmic
- Available disk space
- Ollama availability, version, running status, installed models

Selection targets **70% of practical system capacity** leaving headroom for:
- OS, Ollama runtime, Requiem itself
- Embeddings, databases, background services
- Context memory, tool execution, compilation

## Model Manifest

Every installed model has a machine-readable manifest (`manifests/<id>/manifest.json`):

```json
{
  "id": "unique-model-id",
  "name": "Human readable name",
  "provider": "ollama|gguf",
  "source": "ollama_pull|file_gguf",
  "format": "GGUF",
  "quantization": "Q4_K_M",
  "parameter_count": 8000000000,
  "file_size": 5000000000,
  "sha256": "...",
  "context_length": 128000,
  "capability_tier": "fast|general|high",
  "runtime": "ollama",
  "runtime_name": "llama3.1:8b",
  "status": "healthy",
  "health": { "healthy": true, "last_test_at": "...", "latency_ms": 45 },
  "capabilities": { "coding_ability": 7, "reasoning_ability": 7 },
  "created": "2026-01-15T10:30:00Z",
  "updated": "2026-01-15T10:35:00Z",
  "vram_requirement_mb": 6000,
  "ram_requirement_mb": 9000
}
```

## Lifecycle States

```
absent → downloading → verifying → importing → testing → healthy
                                    ↓ (failure)
                                   failed → (recovery) → healthy
                                    ↓ (activation)
                                  active → (deactivation) → healthy
```

## Recovery Behavior

- **Idempotent**: Safe to run repeatedly
- **Atomic operations**: Temp files (`.tmp`) → atomic rename
- **Rollback**: Failed activation restores previous active model
- **Scan & Repair**: `recover` command detects missing/corrupt/unregistered models and re-acquires

## CLI Usage

```bash
# Hardware detection
modelmgr detect -json

# List models
modelmgr list -json

# Get active model for tier
modelmgr active general -json

# Bootstrap all three tiers
modelmgr bootstrap -json -timeout 30m

# With parallel acquisition
modelmgr bootstrap -parallel -json

# Recovery scan & repair
modelmgr recover -json

# Test specific model
modelmgr test llama3.1-8b-general -json

# Activate model for tier
modelmgr activate general llama3.1-8b-general

# Request model from Requiem (JSON stdin)
echo '{"action":"acquire_model","capability":"software_engineering","tier":"high","reason":"complex task"}' | modelmgr request -json
```

## HTTP API

```
GET  /api/models              # List all models
GET  /api/models/get?id=xxx   # Get model by ID
GET  /api/models/active?tier=general  # Get active model
POST /api/models/activate     # Activate model {tier, model_id}
POST /api/models/test         # Test model {id}
DELETE /api/models/remove?id=xxx      # Remove model
POST /api/models/repair?id=xxx        # Repair model
POST /api/models/update?id=xxx        # Update model
POST /api/models/recover      # Run recovery scan
POST /api/models/request      # Requiem model request
POST /api/models/bootstrap    # Bootstrap all tiers
GET  /api/hardware            # Hardware detection
```

## Requiem Integration

Requiem requests models via JSON:

```json
{
  "action": "acquire_model",
  "capability": "software_engineering",
  "tier": "high",
  "reason": "Current model insufficient for complex refactoring task",
  "max_vram_mb": 10000,
  "max_ram_mb": 30000
}
```

The manager handles the full lifecycle and returns:

```json
{
  "model_id": "qwen2.5-32b-high",
  "name": "Qwen 2.5 32B (Ollama native)",
  "tier": "high",
  "status": "healthy",
  "active": true,
  "message": "model acquired, verified, imported, and tested successfully",
  "duration": "145.2s"
}
```

## Configuration

`config/model-manager.yaml`:

```yaml
models_dir: "~/.requiem/models"
manifest_dir: "~/.requiem/manifests"
registry_file: "~/.requiem/registry.json"
download_retries: 3
download_timeout: "30m"
chunk_size: 1048576
max_concurrent_downloads: 2
verify_workers: 2
health_test_prompt: "Briefly confirm you are ready to assist."
utilization_target: 0.70
ollama_url: "http://127.0.0.1:11434"
temp_dir: "~/.requiem/tmp"
log_level: "info"
```

## Control Room Scripts

Located in `control room/`:

| Script | Purpose |
|--------|---------|
| `run_modelmgr.bat` | Interactive menu for model management |
| `setup_firewall.bat` | Configure Windows Firewall (run as Admin) |
| `manage_server.bat` | Check/stop/start Requiem server |

## Adding New Models

1. Add candidate to `internal/modelmgr/candidates.go`
2. Rebuild: `go build ./cmd/modelmgr`
3. Run bootstrap or request acquisition

## Security

- No arbitrary command execution from model requests
- Path traversal protection on all file operations
- Checksum verification before execution
- Models treated as data, not executable code
- Firewall rules restrict to local network

## Troubleshooting

| Issue | Resolution |
|-------|------------|
| Ollama not detected | Ensure `ollama serve` is running |
| Download 401/404 | Update GGUF URLs in catalog |
| Health test timeout | Increase `download_timeout` |
| VRAM OOM | Lower tier or use CPU offload |
| Registry corruption | Delete `registry.json` and re-bootstrap |

## Files

```
internal/modelmgr/
├── types.go              # Core types
├── manifest.go           # Manifest store
├── registry.go           # Model registry
├── config.go             # Configuration
├── catalog.go            # Model catalog
├── candidates.go         # Candidate definitions
├── selector.go           # Selection policy
├── downloader.go         # Resumable downloader
├── integrity.go          # SHA256 verification
├── ollama_client.go      # Ollama HTTP client
├── importer.go           # Model import (Modelfile)
├── health.go             # Health testing
├── manager.go            # Main orchestrator
├── bootstrap.go          # Bootstrap & recovery
├── api.go                # HTTP handlers
├── hardware/
│   ├── detect.go         # Cross-platform detection
│   ├── detect_windows.go # Windows specifics
│   ├── detect_unix.go    # Unix specifics
│   ├── types.go          # Hardware types
│   └── http_helpers.go   # HTTP utilities
├── os_helpers.go         # OS abstractions
└── errors.go             # Typed errors

cmd/modelmgr/main.go      # CLI entry point
control room/
├── run_modelmgr.bat      # Interactive model manager
├── setup_firewall.bat    # Firewall config (Admin)
└── manage_server.bat     # Server lifecycle manager
```