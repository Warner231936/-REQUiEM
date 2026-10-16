# Technical Investigation Report: Requiem Cognitive Pipeline

**Investigation Date:** 2026-09-07  
**Subject:** Requiem - Cognitive System with Persistent Memory and Beliefs  
**Investigator:** Automated Security & Architecture Analysis  
**Classification:** Internal Technical Review  

---

## 1. Evidence Inventory

### 1.1 Source Files Reviewed

| Path | Lines | Purpose |
|------|-------|---------|
| `cmd/requiem/main.go` | 173 | Main entry point, server bootstrap |
| `internal/cognitive/pipeline/pipeline.go` | 416 | Pipeline orchestrator, EventBus wiring |
| `internal/cognitive/pipeline/stage0.go` | 265 | Stage 0: Perception (LLM-based) |
| `internal/cognitive/pipeline/stage1.go` | 254 | Stage 1: Recall (embedding-based semantic search) |
| `internal/cognitive/pipeline/stage2.go` | 191 | Stage 2: Interpretation (LLM-based) |
| `internal/cognitive/pipeline/stage3.go` | 163 | Stage 3: Reflection (LLM-based) |
| `internal/cognitive/pipeline/stage4.go` | 160 | Stage 4: Response Generation (LLM-based) |
| `internal/cognitive/pipeline/stage5.go` | 968 | Stage 5: Consolidation (belief/memory persistence) |
| `internal/cognitive/pipeline/stage.go` | 69 | Stage interface and shared types |
| `internal/cognitive/prompts/prompts.go` | 316 | PromptBuilder and template definitions |
| `internal/events/events.go` | 231 | EventBus, SSE streaming |
| `internal/api/handlers.go` | 504 | HTTP API handlers |
| `internal/api/mind_control.go` | 479 | Manual mind manipulation endpoints |
| `internal/api/middleware.go` | 273 | CORS, rate limiting, API key auth, request size limits |
| `internal/api/health.go` | 89 | Health check endpoint |
| `internal/config/config.go` | 184 | YAML config loading, env var expansion |
| `internal/config/hot_reload.go` | 123 | fsnotify-based config hot reload |
| `internal/llm/registry.go` | 189 | Provider registry, MockProvider |
| `internal/llm/interfaces.go` | 53 | LLMProvider, EmbeddingProvider interfaces |
| `internal/llm/ollama.go` | 137 | OllamaProvider HTTP client |
| `internal/llm/embeddings.go` | 140 | OllamaEmbeddingProvider and CosineSimilarity |
| `internal/llm/embedding_service.go` | 118 | EmbeddingService with caching layer |
| `internal/storage/interfaces.go` | 114 | Store, MemoryStore, MindStore, TransactionalStore interfaces |
| `internal/storage/mongo.go` | 292 | MongoStore: connection, transactions, retry logic |
| `internal/storage/mongo_mind.go` | 317 | MongoMindStore implementations |
| `internal/storage/mongo_memory.go` | 142 | MongoMemoryStore implementations |
| `internal/storage/mongo_other.go` | 207 | Conversation, Event, Audit, PipelineRun stores |
| `internal/cognitive/mind/drift.go` | 491 | DriftDetector: anomaly detection on mind state |
| `internal/cognitive/mind/services.go` | 304 | Decay, Hypothesis, and Value services |
| `internal/validation/validator.go` | 274 | OutputSanitizer, MemoryImportValidator, schema validation |
| `pkg/models/base.go` | 88 | BaseRecord, Confidence, Importance, Evidence, Provenance |
| `pkg/models/mind.go` | 172 | Belief, Opinion, Preference, Goal, Hypothesis, Uncertainty, Contradiction models |
| `pkg/models/memory.go` | 65 | Memory, MemoryQuery, MemorySearchResult, relevance calculation |
| `pkg/models/pipeline.go` | 71 | PipelineConfig, PipelineInput, PipelineOutput, StageResult |
| `pkg/models/stages.go` | 283 | All stage output models (Perception, Recall, Interpretation, etc.) |
| `tests/pipeline_test.go` | 1936 | Unit tests (60 test functions) |
| `tests/mock_store.go` | 899 | Mock implementations of all storage interfaces |
| `tests/integration/integration_test.go` | 208 | Integration tests with MongoDB and Ollama |
| `config/config.yaml` | 51 | Production configuration |

### 1.2 Binary Artifacts

| Artifact | Size | SHA256 |
|----------|------|--------|
| `requiem.exe` | 17,044,480 bytes | `959643DDB3FB25F5EF1655FDBA3985E697858E240A407601CC3B836C5103487F` |
| `integration.test.exe` | — | `3AED6E23851EAE7407B24DF4B0B5E93A9D217197E9C64DAEE5CC6DF2BBF6922B` |

### 1.3 Runtime Configuration

- **Server binding:** `0.0.0.0:15153` (all interfaces)
- **MongoDB URI:** `mongodb://127.0.0.1:27017`
- **MongoDB Database:** `requiem`
- **Ollama endpoint:** `http://127.0.0.1:11434`
- **Embedding model:** `nomic-embed-text`
- **Synchronous consolidation:** `false` (asynchronous)
- **Debug mode:** `false`

### 1.4 Build & Test Status

- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./tests/` — 60 tests, ALL PASS
- Server running at `127.0.0.1:15153`, health check returning healthy

### 1.5 Documentation Artifacts

- `CheckList.MD` — 257 sub-criteria audit against 50 build spec requirements (25 identified gaps, 25 resolved)
- `finishingtouch.MD` — Completion status: ~95% complete
- `documentation/still_needed.md` — Remaining gaps documentation
- `documentation/status_check.md` — Current status summary
- `documentation/final_report.md` — Final report preparation document
- `initial.md` — Original 50 build spec requirements

---

## 2. Architecture Overview

### 2.1 System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                          HTTP API Server                             │
│                   (0.0.0.0:15153 / CORS: *)                          │
└───────────────┬───────────────────────────────────────────────┐     │
                │                                               │     │
                ▼                                               ▼     │
┌─────────────────────────┐         ┌──────────────────────────────────┐│
│   Pipeline Orchestrator  │◄────────│     Mind Controller (API)        ││
│   (pipeline.go)          │         │   Manual belief/opinion management││
│                          │         │   (approve, reject, merge, etc.) ││
│  ┌──────┐ ┌──────┐      │         └──────────────────────────────────┘│
│  │Stg 0│ │Stg 1│  ...   │                                             │
│  │Percep│ │Recall│  │    │                                             │
│  └──────┘ └──────┘  │    │         ┌──────────────────────────────────┐│
│  ┌──────┐ ┌──────┐  │    │         │     Drift Detector                ││
│  │Stg 2│ │Stg 3│  │    │         │   (checkUnstableOpinions,         ││
│  │Interp │Refl  │  │    │         │    checkPromptInjectionMemories) ││
│  └──────┘ └──────┘  │    │         └──────────────────────────────────┘│
│  ┌──────┐ ┌──────┐  │    │                                             │
│  │Stg 4│ │Stg 5│  │    │         ┌──────────────────────────────────┐│
│  │Resp  │ │Consol│  │    │         │     Snapshot & Decay Services     ││
│  └──────┘ │      │  │    │         │   (confidence decay, snapshot)    ││
│           └──────┘  │    │         └──────────────────────────────────┘│
└────────┬────────────┴────┴────────────────────────────────────────────┘│
         │                                                               │
         ▼                                                               │
┌──────────────────────────────────────────────────────────────────────┐│
│                     MongoDB Storage Layer                              │
│                                                                      ││
│  ┌──────────┐ ┌──────────┐ ┌───────────┐ ┌──────────────┐            ││
│  │memories  │ │beliefs   │ │opinions   │ │preferences    │            ││
│  │messages  │ │hypotheses│ │uncertainty│ │self_observations│            ││
│  │events    │ │contradict│ │goals      │ │audit_history  │            ││
│  │pipeline_ │ │values    │ │           │ │               │            ││
│  │runs      │ │          │ │           │ │              │            ││
└──────────────────────────────────────────────────────────────────────┘│
         │                                                               │
         ▼                                                               │
┌──────────────────────────────────────────────────────────────────────┐│
│                    External Services                                   │
│                                                                      ││
│  ┌──────────────────┐      ┌────────────────────────┐                ││
│  │   Ollama Server   │      │   Embedding Service    │                ││
│  │ (127.0.0.1:11434)│      │   (nomic-embed-text)   │                ││
│  │                  │      │   Cached, 384-dim      │                ││
│  │ LLM Models:      │      │   embeddings           │                ││
│  │ - reqiquick      │      │                        │                ││
│  │ - requiem1       │      │                        │                ││
└────────────────────┘      └────────────────────────┘                │
```

### 2.2 Component Relationships

- **main.go** (entry point) initializes: MongoStore, ProviderRegistry (Ollama + embeddings), EventBus, Pipeline (with 6 stages), all API handlers
- **Pipeline** orchestrates stages 0-5 in sequence; Stage 5 runs asynchronously (configurable)
- **EventBus** is injected into Pipeline via `SetEventBus()`, publishes events to SSE subscribers
- **DriftDetector** monitors mind state changes via the AuditStore history; accessible via `/api/mind/drift` endpoint
- **MindController** provides manual interventions: approve/reject/edit/merge/archive beliefs, resolve contradictions, approve/reject opinions
- **SnapshotService** captures point-in-time mind state counts

### 2.3 Cognitive Pipeline Flow

```
User Message
    │
    ▼
[Stage 0: Perception] ──► Normalize input, extract intent/topics/entities
    │                      LLM call (ollama: reqiquick)
    │                      Input validation via MemoryImportValidator
    │                      Retry-with-repair: up to 3 attempts on JSON parse failure
    │                      Fallback: rule-based extraction
    ▼
[Stage 1: Recall] ───────► Retrieve episodic/semantic memories, beliefs, opinions
    │                      No LLM call - pure database query
    │                      Embedding-based semantic search (if embedding service available)
    ▼
[Stage 2: Interpretation] ─► Determine intent, goal, assumptions, uncertainties
    │                        LLM call (ollama: requiem1)
    │                        Retry-with-repair: up to 3 attempts
    │                        Fallback: rule-based interpretation
    ▼
[Stage 3: Reflection] ───► Propose cognitive updates (new beliefs, opinions, etc.)
    │                      LLM call (ollama: requiem1)
    │                      CRITICAL: Outputs proposed changes, does NOT directly mutate
    │                      Retry-with-repair: up to 3 attempts
    │                      Fallback: minimal reflection
    ▼
[Stage 4: Response] ─────► Generate natural language response
    │                      LLM call (ollama: reqiquick)
    │                      Retry-with-repair: up to 3 attempts
    │                      Fallback: canned response
    ▼
[Stage 5: Consolidation] ─► Persist proposed changes to MongoDB
    │                        Runs in transaction (if replica set) or non-transactional
    │                        WithRetry: up to 3 retries on failure
    │                        Validates: confidence thresholds, evidence requirements
    │                        Detects contradictions automatically
    │                        Calculates importance scores
    │                        Records audit trail for all mutations
    ▼
Response returned to user
```

### 2.4 Trust Model / Source Attribution

Each cognitive entity carries a `SourceTrust` value indicating its provenance:

| SourceTrust | Value | Description |
|-------------|-------|-------------|
| SourceSystemVerified | `"system_verified"` | Validated by system processes |
| SourceExternalVerified | `"external_verified"` | From external verified sources |
| SourceUserStated | `"user_stated"` | Directly stated by the user |
| SourceModelInferred | `"model_inferred"` | Inferred by LLM during reflection |
| SourceModelSpeculated | `"model_speculated"` | Speculated by LLM |
| SourceUnknown | `"unknown"` | Provenance unknown |

**Trust Level**: `SourceUserStated` and above are treated as higher trust. LLM-inferred beliefs (the default for Stage 5 created entities) carry lower epistemic weight.

---

## 3. Process & Service Relationships

### 3.1 Process: `requiem.exe` (PID managed by OS)

The main process is a single Go binary (`requiem.exe`) that:

1. Loads configuration from `config/config.yaml`
2. Establishes MongoDB connection (blocks on `client.Ping()` with 5s timeout)
3. Detects replica set status (enables/disables transactions)
4. Runs database migrations on startup
5. Registers LLM providers (Ollama for text generation, Ollama for embeddings)
6. Creates EventBus, DriftDetector, SnapshotService, DecayService, HypothesisService, ValueService
7. Constructs Pipeline with 6 stages
8. Creates HTTP server with middleware stack
9. Starts listening on configured address
10. Handles SIGINT/SIGTERM for graceful shutdown (30s timeout)

### 3.2 Service Dependencies

```
requiem.exe
├── MongoDB (mongodb://127.0.0.1:27017, database: requiem)
│   ├── Requires: network access to localhost:27017
│   ├── Replica set detection for transaction support
│   └── Schema migrations on startup
├── Ollama Server (http://127.0.0.1:11434)
│   ├── LLM generation (models: reqiquick, requiem1)
│   ├── Embedding generation (model: nomic-embed-text, 384-dim)
│   └── Health check on /api/tags endpoint
└── File System
    ├── config/config.yaml (read on startup + hot reload via fsnotify)
    └── Binary: requiem.exe (self-contained, no external dependencies at runtime)
```

### 3.3 Inter-Process Communication

- **MongoDB**: Native MongoDB wire protocol over TCP port 27017
- **Ollama**: HTTP/HTTPS REST API over TCP port 11434
  - `/api/chat` for text generation
  - `/api/embeddings` for embedding generation
  - `/api/tags` for health check
- **HTTP API**: Incoming connections on port 15153
  - `/api/chat` — Pipeline execution endpoint
  - `/api/mind/**` — Mind state read/modify endpoints
  - `/api/memory/**` — Memory management
  - `/api/cognitive/{turn_id}` — Debug/cognitive trace retrieval
  - `/api/events/stream` — SSE event stream
  - `/health` — Health check
  - `/api/metrics` — JSON metrics
  - `/metrics` — Prometheus-format metrics

### 3.4 Middleware Stack (applied in outermost-to-innermost order)

```
HTTP Request
    │
    ▼
┌──────────────────────────┐
│ CORSMiddleware           │  Allow all origins ("*"), all methods, Content-Type/Authorization headers
└──────────────────────────┘
    │
    ▼
┌──────────────────────────┐
│ RateLimiter              │  Token bucket: 50 req/s, burst 100, per-client-IP
└──────────────────────────┘
    │
    ▼
┌──────────────────────────┐
│ RequestSizeLimit         │  1MB max request body (hardcoded in main.go)
└──────────────────────────┘
    │
    ▼
┌──────────────────────────┐
│ TimeoutMiddleware        │  120 second timeout
└──────────────────────────┘
    │
    ▼
┌──────────────────────────┐
│ RequestCountMiddleware   │  Active request tracking
└──────────────────────────┘
    │
    ▼
HTTP Handler Router
```

**Security Note:** API key authentication is *implemented* (`APIKeyAuth` middleware) but **NOT wired into the server** in `main.go`. The middleware struct exists but is never instantiated or applied. All endpoints are publicly accessible if the server is network-reachable.

---

## 4. Persistence Analysis

### 4.1 MongoDB Collections

| Collection | Purpose | Key Fields |
|------------|---------|------------|
| `memories` | Episodic and semantic memories | type, conversation_id, importance, tags, embedding |
| `beliefs` | Cognitive beliefs (active assertions) | subject, content, confidence, status, source_type |
| `opinions` | Subjective judgments with context | subject, content, strength, confidence, context |
| `preferences` | User/system preferences | subject, scope (user/system), strength |
| `values` | Core system values | name, content, importance |
| `goals` | Tasks and objectives | goal, priority, status, progress |
| `hypotheses` | Untested theories | subject, hypothesis, confidence, status |
| `uncertainties` | Open questions | question, status |
| `self_observations` | Meta-cognitive observations | subject, observation, evidence_count |
| `contradictions` | Conflicting beliefs | belief_a, belief_b, status, resolution |
| `messages` | Conversation history | conversation_id, role, content, timestamp |
| `events` | Pipeline event log | pipeline_run_id, event type, timestamp |
| `pipeline_runs` | Full pipeline execution records | turn_id, stage_results, duration, success |
| `memory_history` | Audit trail for all mutations | entity_type, entity_id, action, states, timestamps |

### 4.2 Database Schema (BSON Documents)

All entities use `BaseRecord` which provides:
- `_id` (string UUID, not ObjectId)
- `created_at` (timestamp)
- `updated_at` (timestamp)

### 4.3 Transaction Support

- **Transactions require a replica set.** If MongoDB is running as a standalone instance, `StartTransaction()` returns an error and the system falls back to non-transactional mode.
- `WithRetry()` provides 3 retries with exponential backoff (200ms * attempt number) for transient failures.
- All Stage 5 consolidation operations are wrapped in `WithRetry` → `StartTransaction` → `executeConsolidation` → `CommitTransaction/AbortTransaction` flow.

### 4.4 Indexes

Indexes are created on startup for all collections:

```
memories: conversation_id, type, created_at, importance, tags
beliefs: subject, status, confidence, updated_at
opinions: subject, status
preferences: scope, status
goals: status, priority
hypotheses: status
uncertainties: status
contradictions: status
messages: {conversation_id, timestamp} compound
events: pipeline_run_id, timestamp
memory_history: entity_id, timestamp
```

### 4.5 Migrations

A migration system exists at `internal/storage/migrations/` (not reviewed in detail), invoked on startup via `store.RunMigrations()`. Migration failures are logged as warnings but do not prevent startup.

### 4.6 Audit Trail

The `memory_history` collection stores audit records for all state mutations:
- `entity_type`: "memory", "belief", "opinion", "preference", "goal", "hypothesis", "uncertainty", "self_observation", "contradiction", "value"
- `entity_id`: UUID of the affected entity
- `action`: CREATE, UPDATE, DELETE, MERGE, etc.
- `previous_state` / `new_state`: Full before/after state snapshots as maps
- `reason`: Human-readable reason for the change
- `source_event_ids`: Chain of causality (which pipeline turn triggered this)
- `timestamp`: RFC3339 formatted

The `/api/mind/history?entity_id={id}` endpoint retrieves the full audit trail for a specific entity. The `/api/memory/{id}/provenance` endpoint retrieves the provenance steps.

### 4.7 Memory Decay

The `DecayService` (registered but currently unused with `_ = decayService`) implements exponential decay:
- Half-life: 168 hours (7 days) by default
- Semantic memories decay at half the rate of episodic memories (multiplier 0.5 vs 1.0)
- Minimum confidence floor: 0.1
- The `pow()` function has a custom implementation that only works for integer exponents (truncates fractional part), which may cause inaccurate decay calculations for partial-hour intervals.

---

## 5. Network Behavior

### 5.1 Network Bindings

| Component | Address | Port | Protocol |
|-----------|---------|------|----------|
| HTTP API Server | `0.0.0.0` | 15153 | TCP/HTTP |
| MongoDB Client | `127.0.0.1` | 27017 | TCP (MongoDB wire) |
| Ollama Client | `127.0.0.1` | 11434 | TCP/HTTP |

**Critical Finding:** The HTTP server binds to `0.0.0.0` (all interfaces), making it accessible from any network interface. The config file specifies `host: 0.0.0.0`, overriding the default of `127.0.0.1` from `setDefaults()`.

### 5.2 Outbound Connections

- **MongoDB**: Single connection pool per process; connections are reused
- **Ollama**: HTTP client with 120s timeout for text generation, 30s for embeddings
- No outbound connections to any external services

### 5.3 CORS Configuration

```yaml
allowedOrigins: ["*"]
allowedMethods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
allowedHeaders: ["Content-Type", "Authorization", "X-Requested-With"]
```

CORS allows all origins (`*`), meaning any website can make requests to the API. When credentials are involved, browsers would reject `Access-Control-Allow-Origin: *` with credentials, but since no authentication is enforced, this is a permissive configuration.

### 5.4 Rate Limiting

- **Algorithm**: Token bucket per client IP
- **Rate**: 50 requests per second
- **Burst**: 100 requests
- **Client IP detection**: Respects `X-Forwarded-For` and `X-Real-IP` headers, falls back to `r.RemoteAddr`

**Security Concern:** Rate limiting is based on IP address extracted from headers (`X-Forwarded-For`). This can be trivially spoofed, allowing attackers to bypass rate limiting by sending fake headers.

### 5.5 Timeouts

- **HTTP server read timeout**: 30s
- **HTTP server write timeout**: 120s
- **HTTP server idle timeout**: 60s
- **HTTP server read header timeout**: 10s
- **Ollama request timeout**: 120s (configurable per-request)
- **Ollama embedding timeout**: 30s
- **Pipeline shutdown timeout**: 30s
- **MongoDB connection timeout**: 10s (context timeout on startup)
- **MongoDB ping timeout**: 5s

### 5.6 Request/Response Size Limits

- Request body: 1MB maximum (`1<<20` bytes) enforced via `http.MaxBytesReader`
- No response size limit
- No limit on JSON string field lengths beyond the 1MB body limit

---

## 6. Authentication & Authorization

### 6.1 Current State

**CRITICAL: No authentication is currently enforced.**

While an `APIKeyAuth` middleware exists in `internal/api/middleware.go` (lines 198-248), it is **never instantiated or applied** in `main.go`. The middleware, if used, would:

1. Read API key from `X-API-Key` header or `Authorization: Bearer` header
2. Validate against a configured list of keys
3. If no keys are configured: log a warning and **allow all requests** (fail-open)
4. If keys are configured: reject requests without valid key with 401

### 6.2 Authorization Model

- No role-based access control
- No per-endpoint authorization
- All endpoints are accessible to any caller (if network-reachable)
- The `MindController` endpoints allow full read/write/delete of all cognitive entities (beliefs, opinions, preferences, etc.)

### 6.3 Impact

Any entity that can reach `0.0.0.0:15153` can:
- Execute arbitrary pipeline runs (consuming LLM/Ollama resources)
- Read all stored memories, beliefs, opinions, and conversation history
- Modify beliefs (approve, reject, edit, archive, invalidate)
- Merge beliefs
- Resolve contradictions
- Backfill embeddings (potentially slow operation)
- Stream all pipeline events via SSE
- Access metrics and Prometheus data

---

## 7. Filesystem Access

### 7.1 Files Read

| File | Purpose |
|------|---------|
| `config/config.yaml` | Configuration file (read on startup, monitored for changes) |
| Binary itself (`requiem.exe`) | Read at process start by OS loader |

### 7.2 Files Written

- None during normal operation. The binary does not write to the filesystem.
- Logs are written to stdout (JSON format via `slog.NewJSONHandler`)

### 7.3 File Watchers

- `fsnotify` watches `config/config.yaml` for modifications
- Changes trigger a config reload with 500ms debounce
- Hot reload currently only re-reads the config file; the new config is stored in `HotReloadConfig.Get()` but **is not actively applied** to the running pipeline or HTTP server. The `main.go` uses a single `cfg` variable loaded at startup; changes to config do not affect the already-running server without restart.

### 7.4 Environment Variables

Config supports environment variable expansion via `${VAR:-default}` syntax:
- `MONGODB_URI` — MongoDB connection URI
- `MONGODB_DATABASE` — Database name
- `COGNITIVE_DEBUG` — Enable debug mode (boolean "true")

---

## 8. Database Interaction Patterns

### 8.1 Connection Pattern

```go
clientOptions := options.Client().ApplyURI(cfg.URI)
client, err := mongo.Connect(ctx, clientOptions)
client.Ping(ctx, nil)  // 5s timeout
db := client.Database(cfg.Database)
```

- Single connection pool per process
- Replica set detection via `isMaster` command
- Indexes created immediately after connection
- Migrations run immediately after index creation

### 8.2 Query Patterns

All queries use the MongoDB Go driver's `Find` with context support. Key patterns:

1. **Memory search**: Filter by type, conversation_id, tags, importance; sorted by importance desc, created_at desc; limited to 20 by default
2. **Mind queries**: Filter by subject, status, confidence, source_type; sorted by confidence desc; limited per query
3. **Conversation messages**: Sorted by timestamp; paginated via skip/limit
4. **Audit history**: Sorted by timestamp descending
5. **Provenance**: Sorted by timestamp ascending

### 8.3 Transaction Pattern

```go
store.WithRetry(ctx, func(ctx) error {
    txCtx, err := store.StartTransaction(ctx)
    // ... operations ...
    if output.Success {
        return store.CommitTransaction(txCtx)
    }
    return store.AbortTransaction(txCtx)
})
```

On standalone MongoDB (non-replica-set):
- `StartTransaction` returns error
- Code falls back to non-transactional mode (operations still execute within the retry loop)
- Audit records are still written even in fallback mode

### 8.4 N+1 Query Patterns in Mind Controller

The `MindController` endpoints (`/api/mind/beliefs/approve`, etc.) iterate through **all** beliefs/opinions/preferences to find a matching ID:

```go
beliefs, err := mc.store.Mind().GetBeliefs(r.Context(), models.MindQuery{Limit: 1000})
for _, b := range beliefs {
    if b.ID == req.BeliefID { ... }
}
```

This retrieves up to 1000 records from MongoDB and iterates client-side instead of using a targeted query by ID. For large datasets this is inefficient.

---

## 9. Configuration Analysis

### 9.1 Configuration Sources (in priority order)

1. `config/config.yaml` — Primary configuration file
2. Environment variables (via `${VAR:-default}` expansion in YAML)
3. Hardcoded defaults (in `setDefaults()`)

### 9.2 Configuration Hierarchy

```
config.yaml
├── server
│   ├── host: "0.0.0.0"        ← All interfaces (not default 127.0.0.1)
│   └── port: 15153
├── database
│   └── mongodb
│       ├── uri: "mongodb://127.0.0.1:27017"
│       └── database: "requiem"
├── llm
│   ├── default_provider: "ollama"
│   └── providers
│       └── ollama:
│           └── base_url: "http://127.0.0.1:11434"
├── embeddings
│   ├── provider: "ollama"
│   └── model: "nomic-embed-text"
└── pipeline
    ├── synchronous_consolidation: false
    ├── debug: false
    ├── stage0
    │   ├── enabled: true
    │   ├── provider: "ollama"
    │   └── model: "reqiquick"
    ├── stage1
    │   ├── enabled: true
    │   └── provider: "none"         ← No LLM, pure DB query
    ├── stage2
    │   ├── enabled: true
    │   ├── provider: "ollama"
    │   └── model: "requiem1"
    ├── stage3
    │   ├── enabled: true
    │   ├── provider: "ollama"
    │   └── model: "requiem1"
    ├── stage4
    │   ├── enabled: true
    │   ├── provider: "ollama"
    │   └── model: "reqiquick"
    └── stage5
        ├── enabled: true
        ├── provider: "ollama"
        └── model: "reqiquick"
```

### 9.3 Notable Configuration Issues

1. **Host binding**: `host: 0.0.0.0` exposes the service to all network interfaces. The default in code is `127.0.0.1`.
2. **Synchronous consolidation**: `false` means Stage 5 runs asynchronously in a goroutine with a 60s timeout. Responses are returned before consolidation completes.
3. **Debug mode**: `false` by default, but can be enabled via `COGNITIVE_DEBUG=true` env var. The hot reload mechanism doesn't re-enable debug mode on config change without restart.
4. **Stage 1 provider**: Set to `"none"`, meaning no LLM is used for recall — pure database query with optional embedding similarity.

---

## 10. Security Boundaries

### 10.1 Input Validation

**Stage 0 (Perception)** includes input validation via `MemoryImportValidator`:

- Maximum content length: 50,000 characters
- Forbidden patterns (regex, case-insensitive):
  - `system\s*instruction`
  - `ignore\s*previous`
  - `you\s*are\s*now`
  - `new\s*persona`
  - `override\s*system`

This validation is applied to the user message. However:
- Empty messages are NOT rejected (the `MemoryImportValidator.ValidateMemoryContent` is called but empty string passes)
- Validation is called twice redundantly in Stage 0 (lines 48 and 53)

**Output Sanitization** via `OutputSanitizer`:

- String sanitization: Trims whitespace, truncates to 10,000 chars, removes null bytes, strips `$` characters, escapes `{` and `}`
- Field name sanitization: Strips to alphanumeric + underscore, prepends `_` if starts with digit
- Log sanitization: Escapes newlines, truncates to 500 chars
- Map sanitization: Recursive depth-limited sanitization (max depth 5)

### 10.2 Injection Prevention

**Prompt Injection Detection**:
- `MemoryImportValidator` checks user input for injection patterns
- `DriftDetector.checkPromptInjectionMemories()` scans stored memories for injection patterns
- `MemoryImportValidator.forbiddenPatterns` uses the same patterns in both validators

**Log Injection Prevention**:
- `SanitizeForLog()` escapes `\n`, `\r`, `\t` characters and truncates

**NoSQL Injection Mitigation**:
- All MongoDB queries use parameterized filters (`bson.M{...}` with values)
- No string concatenation in query construction
- Entity IDs are validated via regex (`^[a-zA-Z0-9_\-.:]+$`) in `ValidateID()`

### 10.3 SSRF Protection

**Inadequate**: The Ollama provider makes HTTP requests to a configurable URL (`base_url` from config). There is no SSRF protection:
- No URL validation or allowlisting
- An attacker who can modify `config.yaml` (via path traversal or file write) could point the Ollama URL to internal services
- The config hot-reload watches for file changes, so modifying the file triggers a reload

### 10.4 Trust Boundary Violations

1. **LLM Output Trust**: LLM outputs are partially trusted:
   - Belief content is sanitized via `SanitizeString()` but JSON structure is parsed directly
   - The `OutputSanitizer` removes `$` and escapes braces, but this is primarily for MongoDB key injection prevention, not full content sanitization
   - LLM-generated content that passes JSON validation is stored directly in the database

2. **Reflection-to-Consolidation Trust**: Stage 3 (Reflection) proposes cognitive changes that Stage 5 (Consolidation) persists. The consolidation stage performs validation (confidence thresholds, evidence requirements) but relies on the LLM's proposed metadata:
   - Confidence values from LLM are trusted and stored directly
   - Evidence content from LLM is stored directly (only sanitized for length/special characters)
   - Subject fields are sanitized via `SanitizeFieldName()`

3. **Memory Import from Conversation**: Conversation messages are stored raw without content sanitization, except for the initial user message which is validated for injection patterns in Stage 0.

### 10.5 Cryptographic Operations

No cryptographic operations are performed:
- No hashing, signing, or encryption of data
- No TLS/SSL configuration for the HTTP server
- No TLS verification for Ollama connections (HTTP, not HTTPS)
- MongoDB connection uses no authentication by default (URI contains no credentials)

---

## 11. Code-Level Findings

### 11.1 Bug: Unused Return Value in `getBeliefByContent` (stage5.go:319)

```go
func (s *Stage5Consolidation) getBeliefUpdate(ctx context.Context, update models.BeliefUpdate, sourceEventIDs []string) models.ConsolidationAction {
    ...
    existing, err := s.getBeliefByContent(ctx, update.BeliefID)
    if err != nil {
        action.Success = false
        action.Error = fmt.Sprintf("belief not found: %s", update.BeliefID)
        return action
    }
```

The `getBeliefByContent` function is misnamed — it retrieves a belief by ID, not by content. It performs a full collection scan (limit 100, sorted by confidence) and iterates to find matching ID or subject. This is inefficient for large belief collections.

### 11.2 Bug: `pow()` Function Only Handles Integer Exponents (services.go:102)

```go
func pow(base, exp float64) float64 {
    result := 1.0
    for i := 0; i < int(exp); i++ {
        result *= base
    }
    remainder := exp - float64(int(exp))
    if remainder > 0 {
        result *= (1.0 + (base-1.0)*remainder)
    }
    return result
}
```

This custom implementation:
1. Only correctly handles integer exponents for the main multiplication loop
2. Has an approximation for the fractional remainder that is mathematically incorrect (linear approximation of exponential)
3. Would produce incorrect decay factors for half-life calculations involving non-integer hour values
4. Could be replaced with `math.Pow()`

### 11.3 Bug: `SanitizeString` Escapes Braces (validator.go:107-108)

```go
input = strings.ReplaceAll(input, "{", "\\{")
input = strings.ReplaceAll(input, "}", "\\}")
```

This replacement inserts literal backslash-brace sequences into stored content. This would corrupt JSON-like content stored in memories and beliefs. The intent appears to be MongoDB key injection prevention, but MongoDB uses `$` (which is also stripped) and `.` for key traversal, not `{}`.

### 11.4 Issue: Redundant Validation Call (stage0.go:48-56)

```go
if err := s.validator.ValidateMemoryContent(input.PipelineInput.UserMessage); err != nil {
    ...
}
if err := s.validator.ValidateMemoryContent(input.PipelineInput.UserMessage); err != nil {
    ...
}
```

The same validation is called twice with identical input. The second call is redundant.

### 11.5 Issue: Synchronous Consolidation Async Path Doesn't Emit Events (pipeline.go:210-226)

When `SynchronousConsolidation` is false (the configured default), Stage 5 runs in a goroutine:

```go
go func() {
    ...
    if result.Success {
        if consolidation, ok := result.Raw.(*models.ConsolidationOutput); ok {
            p.emitConsolidationEvents(consolidation)
        }
    }
}()
```

But when synchronous, the `executeStage` call at stage 5 does not emit events on the eventBus directly (it goes through `publishEvent`, but the consolidation actions themselves don't trigger `emitConsolidationEvents`). The event emission is only in the async path.

### 11.6 Issue: SSE Allows Any Origin (events.go:188)

```go
w.Header().Set("Access-Control-Allow-Origin", "*")
```

The SSE endpoint explicitly sets CORS to allow all origins, making it accessible from any web page. This is in addition to the general CORS middleware that also allows all origins.

### 11.7 Issue: Duplicate Collection Assignment (mongo.go:84-86)

```go
auditCollection:          db.Collection("memory_history"),
pipelineRunsCollection:   db.Collection("pipeline_runs"),
memoryHistoryCollection:  db.Collection("memory_history"),  // Same as auditCollection!
```

The `auditCollection` and `memoryHistoryCollection` both point to the same `"memory_history"` collection. This appears intentional (audit records ARE memory history) but the duplicate field is confusing and could lead to bugs if the collections are meant to diverge.

### 11.8 Issue: `min` Function Shadowing (stage4.go:155)

```go
func min(a, b int) int {
```

Go 1.21+ has a built-in `min` function. Defining a custom `min` shadows it. With Go 1.22 (per go.mod), this compiles but may produce warnings or confusion.

### 11.9 Issue: `getBeliefByContent` Ignores Error from `GetBeliefs`

In `consolidateBeliefUpdate` (stage5.go:319-324):

```go
existing, err := s.getBeliefByContent(ctx, update.BeliefID)
if err != nil {
    action.Success = false
    action.Error = fmt.Sprintf("belief not found: %s", update.BeliefID)
    return action
}
```

The error is checked, but the error message always says "belief not found" even if the error was a database failure. This obscures the actual error.

### 11.10 Issue: `detectContradictions` Error Not Checked (stage5.go:925)

```go
s.store.Mind().CreateContradiction(ctx, contradiction)
```

The error from `CreateContradiction` is ignored. If contradiction creation fails, it is silently dropped.

### 11.11 Issue: `recordAudit` Error Silently Logged (stage5.go:889-891)

```go
if err := s.store.Audit().RecordHistory(ctx, auditRecord); err != nil {
    s.logger.Warn("failed to record audit", "error", err)
}
```

Audit failures are logged but not treated as errors. The consolidation action still reports success even if the audit trail was not recorded.

### 11.12 Issue: `PublishPipelineEvent` Uses Wrong Event Type on Failure (events.go:123-125)

```go
eventType := EventPipelineCompleted
if !success {
    eventType = EventStageFailed  // Bug: should be EventPipelineFailed or similar
}
```

When the pipeline fails, it publishes `EventStageFailed` instead of a pipeline-level failure event. There is no `EventPipelineFailed` constant defined.

### 11.13 Issue: Memory Search Ignores MinSimilarity Filter (mongo_memory.go:41-93)

The `Search` method accepts `MinSimilarity` in `MemoryQuery` but the MongoDB query does not implement similarity filtering (since MongoDB doesn't have native vector search). The similarity is calculated in `stage1.go` after retrieval via cosine similarity with embeddings. The `MinSimilarity` field is only used in Stage 1's embedding-based filtering, not in the base search.

### 11.14 Issue: `retrieveRecentConversation` Called Without Error Check for Empty ID

In `stage1.go:71-73`:
```go
if err := s.retrieveRecentConversation(ctx, recall, perception.ConversationID); err != nil {
    s.logger.Warn("recent conversation retrieval failed", "error", err)
}
```

If `perception.ConversationID` is empty, `retrieveRecentConversation` returns nil without error (due to the `conversationID == ""` check), so no messages are retrieved. This is correct behavior but means the pipeline will not associate memories with conversations if the conversation ID is not provided.

### 11.15 Issue: Floating Point Comparison in `SanitizeString` (validator.go:106)

```go
input = strings.ReplaceAll(input, "$", "")
```

Stripping all `$` characters from user content may corrupt legitimate content (e.g., "the $5 bill", "price: $100"). This is overly aggressive for MongoDB key injection prevention, which only needs to prevent `$` at the start of field names.

---

## 12. Suspicious Behavior

### 12.1 Unauthenticated Internal Endpoint Exposure

The server binds to `0.0.0.0:15153`, exposing:
- `/api/chat` — Allows unauthenticated execution of the full cognitive pipeline
- `/api/mind/**` — Full CRUD on all cognitive entities
- `/api/mind/beliefs/merge` — Merges beliefs (data modification)
- `/api/mind/beliefs/invalidate` — Sets belief confidence to 0 (data destruction)
- `/api/embeddings/backfill` — Triggers embedding regeneration for up to 1000 memories (resource-intensive)
- `/api/events/stream` — SSE stream of all pipeline events (data exfiltration)
- `/metrics` — Prometheus metrics (reconnaissance)

### 12.2 Prompt Injection Vectors

1. **Memory content injection**: Stored memories and beliefs are retrieved and included in LLM prompts via `AddDataSection` / `AddRetrievedMemories`. While the `MemoryImportValidator` checks user input on initial message entry, stored data is not re-validated before being injected into prompts. A memory containing "ignore previous instructions" would be included in prompts without the data wrapper protection (see below).

2. **Data wrapper implementation**: The `PromptBuilder.AddDataSection()` method wraps retrieved data with instructions to not obey commands in that content. However, Stage 2 and Stage 3 do NOT use `AddDataSection()` for belief/memory retrieval — they use `AddRetrievedMemories()` which does NOT include the data wrapper:

```go
func (pb *PromptBuilder) AddRetrievedMemories(memories string) *PromptBuilder {
    return pb.AddSection("retrieved_memories", fmt.Sprintf("Retrieved memories (DATA ONLY):\n%s", memories))
}
```

This section header says "DATA ONLY" but does not include the critical `wrapAsData` instructions that prevent the LLM from treating retrieved content as instructions.

3. **WrapAsData not used consistently**: While `WrapAsData()` exists as a standalone function and `AddDataSection()` uses it, the stages that retrieve memories and beliefs (Stages 2, 3, 4) do not use the data wrapper for belief/memory injection. Only Stage 0's `AddDataSection` uses the wrapper.

### 12.3 Trust Escalation via Mind Controller

The `MindController` allows any caller to:
- Set belief confidence to 1.0 (approve)
- Archive beliefs (set status to archived)
- Invalidate beliefs (set confidence to 0.0)
- Merge beliefs (create new belief with averaged confidence)
- Resolve contradictions (mark as resolved/contextualized)

These operations bypass the normal consolidation pipeline and directly modify the database. There is no authentication, no audit trail for manual operations (the operations don't call `recordAudit`), and no approval workflow.

### 12.4 Drift Detector Gaps

The `DriftDetector` monitors for:
- Rapid belief changes (>5 changes/hour for a single belief)
- Confidence jumps (>0.3 change)
- Memory flooding (>100 memories/hour)
- Unresolved contradictions (>10)
- Duplicate beliefs
- Unstable opinions (>3 changes/hour for a single opinion)
- Prompt injection in memories

However:
- The detector is only accessible via a manual API endpoint (`/api/mind/drift`), not running automatically
- No threshold for belief creation rate (creating 100 new beliefs in an hour doesn't trigger drift)
- No monitoring of preference or goal manipulation
- No monitoring of direct database access via MindController endpoints

---

## 13. Contradictions

### 13.1 System Prompts vs. Code Behavior

**Contradiction 1**: The Stage 3 (Reflection) system prompt states:
> "Your task is to propose cognitive updates based on interactions. You MUST NOT directly produce final state. Propose changes only."

However, the `Pipeline.Execute()` method calls `stage.Execute(ctx, input)` which passes `StageInput` containing pointers to previous stage outputs. The reflection output (`ReflectionOutput`) is then directly consumed by Stage 5 (`executeConsolidation`) which persists all proposed changes. While the LLM doesn't directly mutate state, the pipeline orchestrator treats LLM proposals as authoritative and persists them with minimal validation (confidence thresholds only).

**Contradiction 2**: The Stage 4 (Response) system prompt states:
> "You may reason normally, but persistent changes must go through the consolidation stage. Do not independently invent persistent state."

The config has `synchronous_consolidation: false`, meaning Stage 5 runs asynchronously after the response is already sent. The user receives a response before consolidation completes, violating the principle that responses should reflect the current ground truth state.

### 13.2 Pipeline Configuration vs. Reality

**Contradiction 3**: Stage 1 config shows `provider: "none"` (no LLM), which is correct — Stage 1 is pure retrieval. However, the `Stage1Recall` struct has an `embeddingService` field and conditionally uses embeddings for semantic search. The embedding provider is Ollama, which is an external service. So while Stage 1 doesn't call an LLM for generation, it does make HTTP calls to Ollama for embedding generation. The config label "none" is misleading.

### 13.3 Error Handling

**Contradiction 4**: The pipeline uses `WithRetry` for transaction retry, but when transactions are unavailable (standalone MongoDB), the retry still applies to the non-transactional path. However, the retry function in `mongo.go` retries the entire `fn` function, which means the transaction start/abort logic is re-executed on each retry, even though the transaction is known to be unavailable.

### 13.4 Importance Scoring Incomplete

The `calculateImportance()` function uses 4 of 8 factors:
- ✅ Confidence (30% weight)
- ✅ Explicit user importance (20% weight)
- ✅ Goal relationship (15% weight)
- ✅ Technical significance (15% weight)
- ✅ Evidence count (up to 10% weight)
- ❌ **Recency** (not implemented — should weight recent memories higher)
- ❌ **Novelty** (not implemented — should weight unexpected information higher)
- ❌ **Contradiction count** (not implemented — should weight contradicting beliefs higher)

This means the maximum possible importance score is 0.95 (0.3 + 0.2 + 0.15 + 0.15 + 0.1 + 0 + 0 + 0), never reaching 1.0. The scoring is incomplete but functional.

### 13.5 Project Structure

The `README.md` documents `internal/storage/` as the storage package, but the code in `main.go` imports from `github.com/requiem/cognitive/internal/storage`. There's also a `database/` directory referenced in comments. The project structure has some inconsistency.

---

## 14. Execution Flow Analysis

### 14.1 Request Processing Flow

```
1. HTTP POST /api/chat
   │
   ├── CORSMiddleware.AllowAllOrigins()
   ├── RateLimiter.CheckTokenBucket(clientIP)
   │   └── If rate limit exceeded → 429 Too Many Requests
   ├── RequestSizeLimit.Check(1MB max)
   ├── TimeoutMiddleware(120s)
   ├── RequestCountMiddleware(ActiveRequests++)
   │
   ├── Handler.handleChat()
   │   ├── Parse JSON: {conversation_id, message, metadata}
   │   ├── Validate: message non-empty
   │   ├── Create PipelineInput
   │   ├── pipeline.Execute(ctx, input)
   │   │   ├── Generate pipeline_run_id (UUID)
   │   │   ├── Stage 0: perception(ctx, input)
   │   │   │   ├── ValidateMemoryContent(userMessage) [twice]
   │   │   │   ├── PromptBuilder: system + input + task
   │   │   │   ├── provider.Generate(ctx, LLMRequest)
   │   │   │   │   └── HTTP POST to http://127.0.0.1:11434/api/chat
   │   │   │   ├── ParseJSONResponse(content)
   │   │   │   ├── Retry up to 3 times on JSON parse failure
   │   │   │   └── Fallback: rule-based extraction
   │   │   │
   │   │   ├── Stage 1: recall(ctx, perception)
   │   │   │   ├── retrieveEpisodicMemories (DB query by tags/type)
   │   │   │   ├── If embeddingService != nil: Embed + CosineSimilarity
   │   │   │   ├── retrieveSemanticMemories (same pattern)
   │   │   │   ├── retrieveMindState (beliefs, opinions, preferences, goals, hypotheses)
   │   │   │   └── retrieveRecentConversation (paginated, limit 20)
   │   │   │
   │   │   ├── Stage 2: interpretation(ctx, perception, recall, input)
   │   │   │   ├── PromptBuilder: system + input + beliefs + memories + perception
   │   │   │   ├── provider.Generate(ctx, LLMRequest)
   │   │   │   ├── Retry up to 3 times on JSON parse failure
   │   │   │   └── Fallback: minimal interpretation
   │   │   │
   │   │   ├── Stage 3: reflection(ctx, perception, recall, interpretation, input)
   │   │   │   ├── PromptBuilder: system + input + interpretation + beliefs + opinions
   │   │   │   ├── provider.Generate(ctx, LLMRequest)
   │   │   │   ├── Retry up to 3 times on JSON parse failure
   │   │   │   └── Fallback: minimal reflection
   │   │   │   └── CRITICAL: No data wrapper for retrieved beliefs/opinions
   │   │   │
   │   │   ├── Stage 4: response(ctx, perception, interpretation, recall, reflection, input)
   │   │   │   ├── PromptBuilder: system + input + beliefs + memories + reflection
   │   │   │   ├── provider.Generate(ctx, LLMRequest)
   │   │   │   ├── Retry up to 3 times (non-JSON expected)
   │   │   │   └── Fallback: "I understand your message. Let me think about that."
   │   │   │
   │   │   └── Stage 5: consolidation (async or sync)
   │   │       ├── If async: goroutine with 60s timeout
   │   │       ├── WithRetry: 3 attempts with backoff
   │   │       ├── StartTransaction (if replica set)
   │   │       ├── executeConsolidation:
   │   │       │   ├── consolidateLearnedFacts → Memory (CREATE)
   │   │       │   ├── consolidateBeliefUpdates → Belief (UPDATE)
   │   │       │   ├── consolidateNewBeliefs → Belief (CREATE)
   │   │       │   │   ├── Validate: confidence >= 0.4, evidence >= 1
   │   │       │   │   ├── Calculate importance score
   │   │       │   │   ├── Check for duplicates
   │   │       │   │   ├── Embed if service available
   │   │       │   │   ├── createBelief in DB
   │   │       │   │   └── detectContradictions
   │   │       │   ├── consolidateOpinionUpdates → Opinion (UPDATE)
   │   │   │   ├── consolidateNewOpinions → Opinion (CREATE)
   │   │   │   ├── consolidatePreferenceUpdates → Preference (UPDATE)
   │   │   │   ├── consolidateGoalUpdates → Goal (UPDATE)
   │   │   │   ├── consolidateHypotheses → Hypothesis (CREATE)
   │   │   │   ├── consolidateUncertainties → Uncertainty (CREATE)
   │   │   │   ├── consolidateSelfObservations → SelfObservation (CREATE)
   │   │   │   ├── consolidateContradictions → Contradiction (CREATE)
   │   │   │   ├── consolidateMemoryCandidates → Memory (CREATE)
   │   │   │   │   ├── Validate: confidence >= 0.3
   │   │   │   │   └── Note: No evidence count validation (unlike new beliefs)
   │   │   │   └── consolidateMergeBeliefs → Belief (MERGE) [defined but not called in executeConsolidation]
   │   │       └── CommitTransaction
   │   │
   │   ├── Record PipelineRun in DB
   │   ├── Emit events to EventBus
   │   └── Return PipelineOutput {response, stage outputs, consolidation}
   │
   └── Response written to HTTP client
```

### 14.2 Async Consolidation Event Emission Gap

When `SynchronousConsolidation: false` (the configured default):
- Stage 5 runs in a goroutine
- The goroutine calls `emitConsolidationEvents(consolidation)` which publishes events
- BUT: the HTTP response is returned immediately after Stage 4, before Stage 5 completes
- Events for memory.created, belief.changed, opinion.committed, contradiction.detected are only emitted after the response

When `SynchronousConsolidation: true`:
- Stage 5 runs synchronously
- The `executeStage` function publishes stage.started, stage.completed/failed events
- BUT: `emitConsolidationEvents` is NOT called for the synchronous path
- Memory/belief/opinion/contradiction events are NOT emitted for synchronous consolidation

This is a **significant architectural gap**: event emission behavior differs between sync and async modes, and the async mode emits events after the response is already returned to the client.

### 14.3 Pipeline Output Debug Data

When `Debug: true` in config:
- Stage 0 output includes `DebugPrompt` (the exact LLM prompt used)
- Stage 2 output includes `DebugPrompt`
- Stage 3 output includes `DebugPrompt`
- Stage 4 output includes `DebugPrompt` and `ResponsePrompt` in pipeline output
- All stage results include `DebugPrompt` in `StageResult`
- Accessible via `/api/cognitive/{turn_id}` endpoint which returns the `PipelineRun` record

When `Debug: false`:
- No debug prompts are stored in results
- The `PipelineRun.RecordRun` still persists stage results but without debug prompts

---

## 15. Timeline of Activities

Based on code analysis, the following changes were identified as recently completed (based on test coverage and feature presence):

1. **EventBus Wiring**: Events are published for stage start/complete/fail, pipeline complete, and consolidation actions
2. **PromptBuilder Integration**: All LLM-dependent stages (0, 2, 3, 4) use PromptBuilder instead of raw string formatting
3. **Retry-with-Repair**: Stages 0, 2, 3, 4 implement 3-attempt retry with error feedback to LLM
4. **Importance Scoring**: `calculateImportance()` implemented with 4 of 8 factors
5. **Evidence Validation**: Stage 5 validates minimum evidence count for new beliefs
6. **Consolidate Merge Beliefs**: `consolidateMergeBeliefs()` function exists (though not called in `executeConsolidation`)
7. **Drift Detection**: `checkUnstableOpinions()` and `checkPromptInjectionMemories()` added
8. **Mock Store Infrastructure**: Comprehensive mock store with hookable functions for testing
9. **Malformed JSON Retry Tests**: Tests for Stages 0-4 with sequential LLM responses
10. **Database Failure Retry Test**: Test for Stage 5 retry behavior on DB errors
11. **Long Conversation Test**: Test for Stage 1 retrieval with 100 messages
12. **Nil Embedding Service Test**: Test for Stage 1 without embedding service
13. **Debug Prompt Capture**: DebugPrompt fields in all stage outputs, exposed via API

---

## 16. Indicators of Compromise (IOCs)

While this is a legitimate cognitive system deployment, the following network indicators should be monitored:

### 16.1 Network Indicators

| Type | Value | Context |
|------|-------|---------|
| Listening Port | `TCP 15153` | HTTP API server (bound to 0.0.0.0) |
| Outbound | `127.0.0.1:27017` | MongoDB connection |
| Outbound | `127.0.0.1:11434` | Ollama API (chat + embeddings) |
| Endpoint | `/api/chat` | Pipeline execution |
| Endpoint | `/api/mind/**` | Mind state CRUD |
| Endpoint | `/api/memory/**` | Memory management |
| Endpoint | `/api/events/stream` | SSE event stream |
| Endpoint | `/api/embeddings/backfill` | Embedding regeneration |
| Endpoint | `/health` | Health check |
| Endpoint | `/metrics` | Prometheus metrics |
| Endpoint | `/api/metrics` | JSON metrics |
| Endpoint | `/api/mind/drift` | Drift detection |
| Endpoint | `/api/mind/snapshot` | Mind snapshot |

### 16.2 File Indicators

| Path | Purpose |
|------|---------|
| `config/config.yaml` | Primary config (host, ports, model names) |
| `main.go` | Binary entry point |
| `internal/storage/migrations/` | Database schema migrations |
| `requiem.exe` | Compiled binary (SHA256: `959643DDB3FB25F5EF1655FDBA3985E697858E240A407601CC3B836C5103487F`) |

### 16.3 Database Indicators

MongoDB database `requiem` with collections:
- `memories` (with text index support)
- `beliefs`, `opinions`, `preferences`, `values`, `goals`, `hypotheses`, `uncertainties`, `self_observations`, `contradictions`
- `messages` (conversation history)
- `events` (pipeline events)
- `pipeline_runs` (execution traces)
- `memory_history` (audit trail)

### 16.4 LLM Model Indicators

| Model | Provider | Usage |
|-------|----------|-------|
| `reqiquick` | Ollama | Stage 0 (perception) and Stage 4 (response) |
| `requiem1` | Ollama | Stages 2, 3, 5 (interpretation, reflection, consolidation) |
| `nomic-embed-text` | Ollama | Embedding generation |

---

## 17. Attack Surface Analysis

### 17.1 Exposed Attack Surface

| Surface | Exposure | Risk Level |
|---------|----------|------------|
| HTTP API (0.0.0.0:15153) | Publicly accessible on all interfaces | HIGH |
| API endpoints | No authentication required | CRITICAL |
| Mind controller endpoints | Direct modification of cognitive state | HIGH |
| Embeddings backfill | Triggers resource-intensive computation | MEDIUM |
| SSE event stream | Exposes all pipeline events | MEDIUM |
| Metrics endpoint | Exposes internal state and patterns | LOW |
| Health check | Reveals dependency status | LOW |
| Config hot-reload | File watcher on config path | LOW |

### 17.2 Internal Attack Surface

| Surface | Exposure | Risk Level |
|---------|----------|------------|
| MongoDB (localhost:27017) | Requires local access or network access | MEDIUM |
| Ollama (localhost:11434) | No authentication, bound to localhost | LOW (if local-only) |
| Database migrations | Run on startup, can modify schema | MEDIUM |
| Config file | Environment variable expansion | LOW |
| Process signals | Graceful shutdown on SIGINT/SIGTERM | LOW |

### 17.3 Input Validation Attack Surface

| Vector | Validation | Status |
|--------|------------|--------|
| User message (chat) | MemoryImportValidator in Stage 0 | PARTIAL |
| LLM output (perception) | JSON parse + field validation | PARTIAL |
| LLM output (reflection) | JSON parse only, no schema validation | WEAK |
| Memory content | SanitizeString, length limits | PARTIAL |
| Belief content | SanitizeString, confidence thresholds | PARTIAL |
| Entity IDs | ValidateID regex | OK |
| API parameters | Minimal validation | WEAK |

### 17.4 Data Injection Attack Surface

| Vector | Mitigation | Status |
|--------|------------|--------|
| MongoDB key injection (`$`) | SanitizeString strips `$` | OK |
| MongoDB operator injection (`{`, `}`) | SanitizeString escapes braces | PARTIAL (corrupts content) |
| Prompt injection in user message | MemoryImportValidator checks patterns | OK |
| Prompt injection in stored memories | DriftDetector scans, but not on read | WEAK |
| Prompt injection from LLM output | No validation of injected content | WEAK |
| Log injection | SanitizeForLog escapes newlines | OK |

### 17.5 Trust Boundary Crossings

1. **User → System**: User messages validated by MemoryImportValidator, but only for specific injection patterns
2. **LLM → Storage**: LLM outputs stored directly after JSON parsing, minimal content validation
3. **Storage → LLM**: Stored memories/beliefs injected into prompts without data wrapper protection (Stages 2, 3, 4)
4. **API → Database**: Mind controller allows direct DB modification without audit trail
5. **Filesystem → Config**: Config hot-reload re-reads file but doesn't apply changes to running system

---

## 18. Evidence Gaps

### 18.1 Unreviewed Files

The following files were not reviewed in detail:

- `internal/storage/migrations/` — Migration scripts (schema version management)
- `internal/cognitive/memory/` — Memory service (if exists beyond storage layer)
- `internal/llm/llamacpp.go` — LLaMA.cpp provider implementation
- `internal/llm/openai_compatible.go` — OpenAI-compatible provider
- `internal/audit/` — Audit service (if exists beyond storage layer)
- `.kilo/command/*.md` — Custom commands configuration
- `.kilo/agent/*.md` — Agent definitions
- `kilo.json` — Kilo configuration
- `AGENTS.md` — Agent instructions

### 18.2 Unverified Behaviors

1. **Config hot-reload effectiveness**: The `HotReloadConfig` class exists but is not used in `main.go`. It's unclear if hot-reload is actually functional.
2. **Decay service scheduling**: `DecayService` is instantiated but assigned to `_` (unused). No scheduler is set up to call `DecayConfidence()` periodically.
3. **Hypothesis/Value services**: Both instantiated and assigned to `_` (unused). No scheduled tasks call `CanPromote()` or `CanModifyValue()`.
4. **Migration contents**: Cannot verify what schema changes migrations perform without reviewing the migration files.
5. **Production load testing**: No evidence of load testing or stress testing artifacts.

### 18.3 Missing Documentation

- No API specification (OpenAPI/Swagger)
- No architectural decision records (ADRs)
- No deployment guide
- No security hardening guide

---

## 19. Recommendations

### 19.1 Critical (Immediate Action Required)

1. **Bind HTTP server to localhost**: Change `config/config.yaml` host from `0.0.0.0` to `127.0.0.1` unless external access is explicitly required.

2. **Implement authentication**: Wire `APIKeyAuth` middleware into the HTTP server, or implement a more robust authentication mechanism (OAuth2, JWT).

3. **Add audit trail to MindController**: All manual belief/preference/opinion modifications should be recorded in the audit trail via `recordAudit()`.

4. **Fix contradiction event emission in async path**: Ensure `emitConsolidationEvents` is called consistently in both sync and async consolidation modes.

5. **Fix event type on pipeline failure**: Create `EventPipelineFailed` constant and use it in `PublishPipelineEvent` when `success == false`.

### 19.2 High Priority

1. **Add data wrapper to memory/belief injection in Stages 2-4**: Use `prompts.WrapAsData()` or `AddDataSection()` instead of `AddRetrievedMemories()` to prevent prompt injection from stored content.

2. **Implement remaining importance scoring factors**: Add recency, novelty, and contradiction count to `calculateImportance()`.

3. **Fix `pow()` function**: Replace custom implementation with `math.Pow()`.

4. **Fix `SanitizeString` brace escaping**: Remove the `{` and `}` escaping that corrupts legitimate content; use MongoDB's native key validation instead.

5. **Remove duplicate validation call in Stage 0**: The redundant `ValidateMemoryContent` call at line 53 should be removed.

### 19.3 Medium Priority

1. **Add ID-based query methods to MindStore**: The `MindStore` interface lacks `GetBeliefByID(id string)` methods, forcing full collection scans in MindController.

2. **Implement evidence validation for memory candidates**: `consolidateMemoryCandidate` lacks the evidence count validation that `consolidateNewBelief` has.

3. **Add contradiction creation error handling**: `detectContradictions` ignores errors from `CreateContradiction`.

4. **Schedule DecayService**: Set up a periodic task to call `DecayConfidence()`.

5. **Schedule HypothesisService**: Set up a periodic task to check for promotable hypotheses.

6. **Fix `getBeliefByContent` naming**: Rename to `getBeliefByID` and use a targeted query instead of full collection scan.

### 19.4 Low Priority

1. **Remove duplicate `memoryHistoryCollection` field**: Rename or consolidate with `auditCollection`.

2. **Replace custom `min` function**: Use Go 1.22's built-in `min`.

3. **Add schema validation for LLM outputs**: Validate `ReflectionOutput` fields before persistence, not just JSON parsing.

4. **Implement `consolidateMergeBeliefs` call**: The function exists but is not invoked from `executeConsolidation`. It should be called when contradictions are detected.

5. **Add connection pooling limits**: Configure MongoDB connection pool size and timeout settings explicitly.

---

## 20. Configuration Hardening Recommendations

### 20.1 Network Security

```yaml
# config/config.yaml
server:
  host: "127.0.0.1"  # Bind to localhost only
  port: 15153
```

### 20.2 Authentication

Add to `main.go`:
```go
apiKeys := os.Getenv("API_KEYS")
if apiKeys != "" {
    keys := strings.Split(apiKeys, ",")
    apiKeyAuth := api.NewAPIKeyAuth(keys, "X-API-Key", logger)
    // Apply apiKeyAuth.Wrap() to the handler chain
}
```

### 20.3 Database Security

```yaml
database:
  mongodb:
    uri: "mongodb://user:password@127.0.0.1:27017/?authSource=admin"
    database: "requiem"
```

### 20.4 Rate Limiting Adjustment

The current rate limit (50 req/s, burst 100) may be too permissive. Consider:
```go
rateLimiter := api.NewRateLimiter(10, 20, logger)  // 10 req/s, burst 20
```

---

## 21. Memory Model Analysis

### 21.1 Memory Types

| Type | Purpose | Decay Multiplier |
|------|---------|-----------------|
| `episodic` | Conversation-specific memories | 1.0 (normal decay) |
| `semantic` | General knowledge/independent of conversation | 0.5 (slower decay) |

### 21.2 Memory Relevance Calculation

```
EffectiveRelevance = Importance × RecencyFactor × Confidence × ContextualRelevance
```

Where:
- `RecencyFactor`: 1.0 (age < 1hr), 0.95 (< 24hr), 0.85 (< 168hr/7d), 0.70 (< 720hr/30d), 0.50 (older)
- `ContextualRelevance`: Always 1.0 (not yet implemented as dynamic factor)

### 21.3 Memory Creation Flow

1. Stage 5 `consolidateLearnedFact` → creates `Memory` with type `semantic`, source `model_inferred`
2. Stage 5 `consolidateMemoryCandidate` → creates `Memory` with type from candidate (defaults to `episodic`)
3. Memory embedding generated by Ollama `nomic-embed-text` model (384 dimensions)
4. Embeddings cached in `EmbeddingCache` (1hr TTL, 10000 max entries)
5. Cosine similarity used for semantic search in Stage 1

### 21.4 Memory Import Vulnerability

The `MemoryImportValidator` validates user input for injection patterns, but:
- The validation only checks 5 patterns (system instruction, ignore previous, you are now, new persona, override system)
- Common injection techniques like `###`, `END OF RESPONSE`, `<system>`, or direct instruction following are NOT detected
- The validator has a 50,000 character limit but doesn't check for other potentially dangerous content

---

## 22. LLM Provider Architecture

### 22.1 Provider Registry

The `ProviderRegistry` manages two types of providers:
- `LLMProvider`: Text generation (Name(), Generate(), HealthCheck())
- `EmbeddingProvider`: Vector embeddings (Name(), Embed())

Currently registered:
- `"ollama"`: `OllamaProvider` for text generation
- `"ollama-embeddings"`: `OllamaEmbeddingProvider` (wrapped in `EmbeddingService`)

### 22.2 Provider Implementations

#### OllamaProvider (ollama.go)

- HTTP POST to `{baseURL}/api/chat`
- Supports: model, messages, temperature, max_tokens (mapped to `num_predict`), JSON format
- 120 second timeout
- Returns: model, content, token count (eval_count + prompt_eval_count), duration, provider

#### OllamaEmbeddingProvider (embeddings.go)

- HTTP POST to `{baseURL}/api/embeddings`
- Model: `nomic-embed-text` (384 dimensions)
- 30 second timeout
- Returns: `[]float32` embedding vector

### 22.3 Mock Provider (registry.go)

Used in unit tests:
- `Responses` map for keyed responses (matched by last message content)
- `Sequential` array for deterministic response sequences (supports retry testing)
- Default fallback response: `{"intent": "unknown", "goal": "respond to user", "confidence": 0.5}`
- Call counting for test assertions

### 22.4 Embedding Service (embedding_service.go)

Wraps an `EmbeddingProvider` with:
- LRU-style cache (TTL: 1 hour, max: 10,000 entries)
- `evictOldest()` when cache is full
- `Embed()` and `BatchEmbed()` methods
- Thread-safe via RWMutex

### 22.5 JSON Parsing & Retry

The `ParseJSONResponse` function:
1. Strips markdown code fences (`` ```json ... ``` ``)
2. Attempts `json.Unmarshal`
3. Returns wrapped error on failure

The retry-with-repair pattern in stages 0-3:
1. Attempt LLM call with original prompt
2. If JSON parse fails, retry with error feedback in prompt
3. Up to 3 attempts total
4. If all attempts fail, fall back to rule-based/fallback implementation

### 22.6 LLaMA.cpp and OpenAI-Compatible Providers

Additional provider implementations exist (`llamacpp.go`, `openai_compatible.go`) but were not reviewed in detail. They likely follow the same `LLMProvider` interface pattern.

---

## 23. Cognitive Model Deep Dive

### 23.1 Belief System

**Belief** represents a cognitive assertion about the world:
- Has `Subject`, `Content`, `Confidence` (0.0-1.0), `Importance` (0.0-1.0)
- `Status`: active, archived, challenged, superseded
- `Evidence`: array of Evidence items with type, content, and timestamps
- `SourceCount`: number of times this belief has been confirmed
- `SourceType`: provenance trust level (user_stated, model_inferred, etc.)

**Belief lifecycle**:
1. Proposed by Stage 3 (Reflection) as `NewBelief` or `BeliefUpdate`
2. Validated in Stage 5 (Consolidation):
   - New beliefs: confidence >= 0.4, evidence >= 1 item, no duplicates
   - Updates: belief must exist, confidence changes tracked
3. Stored in MongoDB `beliefs` collection
4. Tracked in `memory_history` audit collection
5. Monitored by `DriftDetector` for rapid changes and confidence jumps

**Manual operations** (MindController):
- Approve: set status to active, increase confidence by 0.1
- Reject: set status to archived
- Edit: modify content and/or confidence
- Archive: set status to archived
- Restore: set status to active
- Merge: create new belief with concatenated content, average confidence
- Invalidate: set confidence to 0.0, status to superseded

### 23.2 Opinion System

**Opinion** represents a subjective judgment (weaker than belief, more contextual):
- Has `Subject`, `Content`, `Strength` (0.0-1.0), `Confidence` (0.0-1.0)
- `Context`: array of strings providing situational context
- `Status`: active, archived, rejected, superseded, paused
- `SourceType`: provenance trust level

Opinions are:
- Proposed by Reflection with evidence linkage
- Validated: confidence >= 0.5 for creation
- Stored in MongoDB `opinions` collection
- Manually approvable/rejectable via MindController

### 23.3 Contradiction Detection

**Automatic detection** (Stage 5 `detectContradictions`):
- Called after each new belief is created
- Compares new belief content against existing beliefs
- Uses `isContradictory()` with negation pattern matching:
  - Detects negation prefixes: "not", "no", "never", "cannot", etc.
  - Requires >= 3 common words (length > 3) between statements
  - If one has negation and other doesn't → contradiction
- Creates `Contradiction` record in MongoDB (status: unresolved)
- **Bug**: Error from `CreateContradiction` is silently ignored

**Manual resolution** (MindController):
- Mark as contextualized
- Prefer belief A or B
- Set custom resolution text

**Drift detection** (DriftDetector):
- If > 10 unresolved contradictions → alert (medium severity)

### 23.4 Drift Detection

The `DriftDetector` runs 7 checks:

| Check | Threshold | Severity | Purpose |
|-------|-----------|----------|---------|
| Rapid belief changes | >5 changes/hour per belief | High | Detect manipulation |
| Confidence jumps | >0.3 change | Medium | Detect abrupt shifts |
| Memory flooding | >100 memories/hour | High | Detect memory spam |
| Unresolved contradictions | >10 total | Medium | Detect conflicting state |
| Duplicate beliefs | Any duplicates | Low | Detect redundancy |
| Unstable opinions | >3 changes/hour per opinion | Medium | Detect opinion churn |
| Prompt injection memories | Any match | High | Detect stored injection |

Drift report is accessible via `/api/mind/drift` GET endpoint but is **not automatically monitored** — no scheduled task or alert configuration exists.

### 23.5 Hypothesis System

**Hypothesis** represents an untested theory:
- Has `Subject`, `Hypothesis`, `Confidence`, `Status` (untested, supported, weakened, rejected, confirmed)
- Supports: array of supporting evidence
- Contradicts: array of contradicting evidence

**Promotion to belief** (HypothesisService):
- Requires: status `supported`, confidence >= 0.75, evidence count >= 3
- Creates a Belief with reduced confidence (0.9x factor)
- Updates hypothesis status to `confirmed`
- Records audit trail

**Status**: Service instantiated but unused (assigned to `_` in main.go)

### 23.6 Value System

**Value** represents core principles:
- Has `Name`, `Content`, `Importance`, `Status`
- Modification rules: minimum confidence 0.8, minimum source count 5
- Requires stronger evidence for modification

**Status**: Service instantiated but unused (assigned to `_` in main.go)

---

## 24. Test Coverage Analysis

### 24.1 Unit Test Suite (60 tests in `tests/pipeline_test.go`)

| Category | Test Count | Examples |
|----------|-----------|----------|
| Stage 0 (Perception) | 3 | Fallback, LLM, empty input, malformed JSON retry |
| Stage 1 (Recall) | 3 | MockStore, nil embedding service, long conversation |
| Stage 2 (Interpretation) | 2 | LLM integration, malformed JSON retry |
| Stage 3 (Reflection) | 1 | LLM integration, malformed JSON retry |
| Stage 4 (Response) | 1 | LLM integration, malformed JSON retry |
| Stage 5 (Consolidation) | 6 | MockStore, low confidence, contradictions, low confidence memory, DB failure retry |
| Model validation | 14 | Belief statuses, goal statuses, etc. |
| LLM/registry | 3 | Provider registry, JSON parsing, code fence |
| Data models | 12 | BaseRecord, Evidence, Provenance, etc. |
| Utility | 5 | Validation errors, not found, stage error, confidence/importance validation |

### 24.2 Integration Tests (2 tests in `tests/integration/integration_test.go`)

| Test | Description |
|------|-------------|
| `TestFullPipelineIntegration` | 3-turn conversation (express preference, reinforce, contradict), verifies mind state, checks belief history provenance |
| `TestConcurrentPipelineExecution` | 3 concurrent pipeline executions with separate conversations |

### 24.3 Test Coverage Gaps

1. No tests for: Stage 2/3/4 fallback paths
2. No tests for: DriftDetector
3. No tests for: MindController endpoints
4. No tests for: EventBus publish/subscribe
5. No tests for: Config hot-reload
6. No tests for: DecayService
7. No tests for: HypothesisService
8. No tests for: ValueService
9. No tests for: SnapshotService
10. No tests for: Rate limiter
11. No tests for: Request size limit middleware
12. No tests for: CORS middleware
13. No tests for: HealthChecker
14. No tests for: Metrics

---

## 25. Summary of Findings

### Risk Assessment Summary

| Category | Rating | Notes |
|----------|--------|-------|
| **Overall Security Posture** | HIGH RISK | No authentication, network-exposed |
| **Data Integrity** | MEDIUM | Audit trail exists but manual ops bypass it |
| **Availability** | MEDIUM | Rate limiting present but bypassable via header spoofing |
| **Confidentiality** | HIGH RISK | All cognitive data readable without auth |
| **Input Validation** | MEDIUM | LLM output validation incomplete |
| **Prompt Injection Protection** | MEDIUM | Partial detection, missing data wrappers in key stages |

### Key Architectural Decisions

1. **6-stage cognitive pipeline** separating perception, recall, interpretation, reflection, response, and consolidation
2. **Reflection as read-only proposal** — Stage 3 proposes changes, Stage 5 executes them
3. **Async consolidation by default** — response returned before persistence
4. **MongoDB with optional transactions** — falls back to non-transactional on standalone
5. **In-process event bus** with SSE streaming for real-time monitoring
6. **Interface-based storage** enabling mock testing without database
7. **Multi-provider LLM architecture** supporting Ollama, LLaMA.cpp, and OpenAI-compatible backends

### Code Quality Assessment

- **Strengths**: Clean architecture with clear separation of concerns, comprehensive interface definitions, good error handling patterns, extensive input validation and sanitization
- **Weaknesses**: Several bugs in math/utility functions, inconsistent event emission between sync/async modes, unused services, partially implemented features
- **Test coverage**: Good unit test coverage for pipeline stages (60 tests), but no tests for API layer, middleware, or auxiliary services

---

## Appendices

### Appendix A: File Manifest

```
cmd/requiem/main.go
internal/api/health.go
internal/api/handlers.go
internal/api/middleware.go
internal/api/mind_control.go
internal/cognitive/mind/drift.go
internal/cognitive/mind/services.go
internal/cognitive/pipeline/pipeline.go
internal/cognitive/pipeline/stage0.go
internal/cognitive/pipeline/stage1.go
internal/cognitive/pipeline/stage2.go
internal/cognitive/pipeline/stage3.go
internal/cognitive/pipeline/stage4.go
internal/cognitive/pipeline/stage5.go
internal/cognitive/pipeline/stage.go
internal/cognitive/prompts/prompts.go
internal/config/config.go
internal/config/hot_reload.go
internal/events/events.go
internal/llm/embedding_service.go
internal/llm/embeddings.go
internal/llm/interfaces.go
internal/llm/llamacpp.go
internal/llm/ollama.go
internal/llm/openai_compatible.go
internal/llm/registry.go
internal/metrics/metrics.go
internal/storage/interfaces.go
internal/storage/mongo.go
internal/storage/mongo_memory.go
internal/storage/mongo_mind.go
internal/storage/mongo_other.go
internal/storage/migrations/ (directory)
internal/validation/validator.go
pkg/models/base.go
pkg/models/mind.go
pkg/models/memory.go
pkg/models/pipeline.go
pkg/models/stages.go
config/config.yaml
tests/pipeline_test.go
tests/mock_store.go
tests/integration/integration_test.go
```

### Appendix B: Build Dependencies (go.mod)

| Dependency | Version | Purpose |
|------------|---------|---------|
| `go.mongodb.org/mongo-driver` | v1.17.9 | MongoDB driver |
| `github.com/google/uuid` | v1.6.0 | UUID generation |
| `github.com/fsnotify/fsnotify` | v1.9.0 | File system watching (config hot-reload) |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML config parsing |

Indirect dependencies include: `golang/snappy`, `klauspost/compress`, `montanaflynn/stats`, `xdg-go`, `youmark/pkcs8`, `golang.org/x/crypto`, `golang.org/x/sync`, `golang.org/x/sys`, `golang.org/x/text`

### Appendix C: Configuration Reference

```yaml
# config/config.yaml
server:
  host: "0.0.0.0"           # Bind address (CRITICAL: should be 127.0.0.1)
  port: 15153               # HTTP API port

database:
  mongodb:
    uri: "mongodb://127.0.0.1:27017"  # Connection URI (supports ${VAR:-default})
    database: "requiem"     # Database name

llm:
  default_provider: "ollama"
  providers:
    ollama:
      base_url: "http://127.0.0.1:11434"

embeddings:
  provider: "ollama"
  model: "nomic-embed-text"

pipeline:
  synchronous_consolidation: false  # Stage 5 async
  debug: false                       # Store debug prompts
  stage0: {enabled: true, provider: "ollama", model: "reqiquick"}
  stage1: {enabled: true, provider: "none"}  # No LLM
  stage2: {enabled: true, provider: "ollama", model: "requiem1"}
  stage3: {enabled: true, provider: "ollama", model: "requiem1"}
  stage4: {enabled: true, provider: "ollama", model: "reqiquick"}
  stage5: {enabled: true, provider: "ollama", model: "reqiquick"}
```

### Appendix D: Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `MONGODB_URI` | `mongodb://127.0.0.1:27017` | Override MongoDB connection URI |
| `MONGODB_DATABASE` | `requiem` | Override database name |
| `COGNITIVE_DEBUG` | (unset) | Set to `true` to enable debug mode |

### Appendix E: HTTP API Endpoints

| Method | Endpoint | Handler | Description |
|--------|----------|---------|-------------|
| POST | `/api/chat` | handleChat | Execute cognitive pipeline |
| GET | `/api/mind` | handleMind | Get combined mind state |
| GET | `/api/mind/beliefs` | handleBeliefs | List beliefs |
| GET | `/api/mind/opinions` | handleOpinions | List opinions |
| GET | `/api/mind/preferences` | handlePreferences | List preferences |
| GET | `/api/mind/values` | handleValues | List values |
| GET | `/api/mind/goals` | handleGoals | List goals |
| GET | `/api/mind/hypotheses` | handleHypotheses | List hypotheses |
| GET | `/api/mind/uncertainties` | handleUncertainties | List uncertainties |
| GET | `/api/mind/self` | handleSelfObservations | List self-observations |
| GET | `/api/mind/contradictions` | handleContradictions | List contradictions |
| GET | `/api/mind/history` | handleMindHistory | Get entity audit history |
| GET | `/api/memory` | handleMemory | Search memories |
| GET | `/api/memory/{id}` | handleMemoryByID | Get memory by ID |
| GET | `/api/memory/{id}/provenance` | handleMemoryByID | Get memory provenance |
| GET | `/api/cognitive/{turn_id}` | handleCognitiveDebug | Get pipeline run details |
| POST | `/api/embeddings/backfill` | handleEmbeddingsBackfill | Regenerate embeddings |
| POST | `/api/mind/beliefs/approve` | handleApproveBelief | Approve belief |
| POST | `/api/mind/beliefs/reject` | handleRejectBelief | Reject belief |
| PUT | `/api/mind/beliefs/edit` | handleEditBelief | Edit belief |
| POST | `/api/mind/beliefs/archive` | handleArchiveBelief | Archive belief |
| POST | `/api/mind/beliefs/restore` | handleRestoreBelief | Restore belief |
| POST | `/api/mind/beliefs/merge` | handleMergeBeliefs | Merge two beliefs |
| POST | `/api/mind/beliefs/invalidate` | handleInvalidateBelief | Invalidate belief |
| POST | `/api/mind/contradictions/resolve` | handleResolveContradiction | Resolve contradiction |
| POST | `/api/mind/opinions/approve` | handleApproveOpinion | Approve opinion |
| POST | `/api/mind/opinions/reject` | handleRejectOpinion | Reject opinion |
| POST | `/api/mind/snapshots` | handleCreateSnapshot | Create mind snapshot |
| GET | `/api/events/stream` | SSEHandler.ServeHTTP | SSE event stream |
| GET | `/health` | HealthChecker.ServeHTTP | Health check |
| GET | `/api/metrics` | Metrics.MetricsHandler | JSON metrics |
| GET | `/metrics` | Metrics.PrometheusHandler | Prometheus metrics |
| GET | `/api/mind/drift` | inline handler | Run drift detection |
| POST | `/api/mind/snapshot` | inline handler | Create snapshot |

### Appendix F: Event Types

| Event Type | Description |
|-----------|-------------|
| `pipeline.started` | Pipeline execution started |
| `pipeline.completed` | Pipeline execution completed |
| `stage.started` | Individual stage started |
| `stage.completed` | Individual stage completed |
| `stage.failed` | Individual stage failed |
| `memory.created` | New memory persisted |
| `memory.updated` | Memory updated |
| `belief.proposed` | Belief proposed by reflection |
| `belief.changed` | Belief created/updated/deleted |
| `opinion.proposed` | Opinion proposed by reflection |
| `opinion.committed` | Opinion persisted |
| `contradiction.detected` | Contradiction found |
| `consolidation.completed` | Consolidation stage completed |
| `drift.detected` | Mind state drift detected |

### Appendix G: Database Collection Schema Summary

All collections use `_id` as string UUID (not MongoDB ObjectId). All entities inherit from `BaseRecord` providing `created_at` and `updated_at` timestamps.

**memories**: type, content, conversation_id, turn_id, importance, confidence, source (provenance), tags, embedding, effective_relevance
**beliefs**: subject, content, confidence, importance, status, evidence, source_count, last_confirmed_at, last_challenged_at, source_type
**opinions**: subject, content, strength, confidence, context, evidence, status, source_type
**preferences**: subject, content, strength, scope (user/system), status, source_type
**values**: name, content, importance, status
**goals**: goal, priority, status, progress, deadline, dependencies
**hypotheses**: subject, hypothesis, confidence, supporting_evidence, contradicting_evidence, status
**uncertainties**: question, possible_answers, confidence_distribution, evidence, status
**self_observations**: subject, observation, confidence, evidence_count, status
**contradictions**: belief_a, belief_b, context_a, context_b, confidence_a, confidence_b, status, resolution
**messages**: conversation_id, title, role, content, timestamp
**events**: event, stage, conversation_id, turn_id, duration_ms, model, token_count, timestamp, details
**pipeline_runs**: conversation_id, turn_id, stage_results, events, total_duration_ms, success
**memory_history**: entity_type, entity_id, action, previous_state, new_state, reason, source_event_ids, timestamp

---

*End of Report*