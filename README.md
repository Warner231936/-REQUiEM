# Requiem Cognitive Pipeline

A Go-based cognitive architecture implementing a 7-stage pipeline with persistent memory (MongoDB), LLM integration (Ollama/llama.cpp/OpenAI-compatible), security hardening, and Prometheus observability.

## Architecture

```
User Input → HTTP API → CORS / RateLimit / SizeLimit / Timeout Middleware
              │
              ▼
   Stage 0: Perception (reqiquick)
     - MemoryImportValidator (prompt injection detection)
     - OutputSanitizer (input sanitization)
     - Language/topic/entity extraction
     - Intent classification

   Stage 1: Recall (no LLM)
     - Episodic & semantic memory retrieval (embedding-based, MinSimilarity=0.5)
     - Mind state retrieval (beliefs, opinions, preferences, goals, hypotheses)
     - Recent conversation history

   Stage 2: Interpretation (requiem1)
     - Intent and goal determination
     - Context extraction
     - Ambiguity and uncertainty identification
     - DATA/INSTRUCTIONS delimiter separation

   Stage 3: Reflection (requiem1)
     - Cognitive update proposals (beliefs, opinions, preferences, goals)
     - Contradiction detection
     - Memory candidates
     - Read-only — does NOT mutate database

    Stage 4: Response (requiem1)
      - User-facing response generation
      - Tool selection and execution (search memories, list beliefs, etc.)
      - DATA/INSTRUCTIONS delimiter separation
      - Does NOT invent persistent state

    Stage 5: Response Humanization (requiem1)
      - Tone and style refinement
      - Personality alignment
      - Readability post-processing

    Stage 6: Consolidation (requiem1)
      - Validates Stage 3 proposals
      - MongoDB transactions (with retry + standalone fallback)
      - OutputSanitizer before DB writes
      - Embedding generation for new memories
      - Audit history recording
      - Automatic contradiction detection
```

## Prerequisites

| Component | Version | Notes |
|---|---|---|
| Go | 1.22+ | |
| MongoDB | 7.0+ | Standalone or replica set |
| Ollama | latest | With models: `reqiquick`, `requiem1`, `nomic-embed-text` |
| Ports | — | 15153 (API), 27017 (MongoDB), 11434 (Ollama) |

## Installation

### 1. Build

```bash
go build -o requiem.exe ./cmd/requiem
```

### 2. Start MongoDB

**Standalone (development):**
```bash
mongod --dbpath /data/db
```

**Replica set (production, enables transactions):**
```bash
mongod --replSet rs0 --dbpath /data/db
# Then in mongosh:
rs.initiate({ _id: "rs0", members: [{ _id: 0, host: "127.0.0.1:27017" }] })
```

### 3. Start Ollama

```bash
ollama serve
ollama pull nomic-embed-text
ollama pull llama3.2:3b
```

### 4. Create Models

```bash
# Main model for stages 0, 2, 3, 4, 5, 6
ollama create requiem1 -f Modelfile.requiem1
```

Example `Modelfile.requiem1`:
```
FROM llama3.2:3b
PARAMETER temperature 0.3
PARAMETER num_ctx 8192
PARAMETER num_predict 2048
```

> You can also use existing Ollama models directly. Update `config/config.yaml` to point to your models. For LlamaCpp or OpenAI-compatible backends, see the configuration section below.

### 5. Configure

Edit `config/config.yaml`:

```yaml
server:
  host: 127.0.0.1
  port: 15153

database:
  mongodb:
    uri: mongodb://127.0.0.1:27017
    database: requiem

llm:
  default_provider: ollama
  providers:
    ollama:
      base_url: http://127.0.0.1:11434
    # Uncomment to use LlamaCpp
    # llamacpp:
    #   base_url: http://127.0.0.1:8080
    # Uncomment to use OpenAI-compatible API
    # openai:
    #   base_url: https://api.openai.com
    #   api_key: ${OPENAI_API_KEY}

embeddings:
  provider: ollama
  model: nomic-embed-text

pipeline:
  synchronous_consolidation: false
  debug: false
  stage0:
    enabled: true
    provider: ollama
    model: reqiquick
  stage1:
    enabled: true
    provider: none
  stage2:
    enabled: true
    provider: ollama
    model: requiem1
  stage3:
    enabled: true
    provider: ollama
    model: requiem1
  stage4:
    enabled: true
    provider: ollama
    model: requiem1
  stage5:
    enabled: true
    provider: ollama
    model: requiem1
  stage6:
    enabled: true
    provider: ollama
    model: requiem1
```

**Environment variables:**
- `MONGODB_URI` — Override MongoDB connection string
- `MONGODB_DATABASE` — Override database name
- `OPENAI_API_KEY` — API key for OpenAI-compatible provider
- `COGNITIVE_DEBUG` — Set to `true` for debug logging

### 6. Run

```bash
./requiem.exe
```

Or with hot reload (config changes are watched automatically):

```bash
./requiem.exe
# Edit config/config.yaml — changes are picked up automatically
```

## API Reference

### Chat

```bash
curl -X POST http://127.0.0.1:15153/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello, what do you know?"}'
```

### Metrics (JSON)

```bash
curl http://127.0.0.1:15153/api/metrics
```

### Metrics (Prometheus)

```bash
curl http://127.0.0.1:15153/metrics
```

### Health Check

```bash
curl http://127.0.0.1:15153/health
```

### Conversations

```bash
# List conversations (paginated)
curl "http://127.0.0.1:15153/api/conversations?limit=20&offset=0"

# Create conversation
curl -X POST http://127.0.0.1:15153/api/conversations \
  -H "Content-Type: application/json" \
  -d '{"title": "My conversation"}'

# Get conversation messages
curl "http://127.0.0.1:15153/api/conversations/{id}?limit=50&offset=0"

# Delete conversation
curl -X DELETE http://127.0.0.1:15153/api/conversations/{id}
```

### Memory

```bash
# Search memories
curl "http://127.0.0.1:15153/api/memory?type=episodic&tags=architecture,technical&limit=20"

# Get specific memory
curl http://127.0.0.1:15153/api/memory/{id}

# Get memory provenance
curl http://127.0.0.1:15153/api/memory/{id}/provenance
```

### Mind

```bash
# Full mind state
curl http://127.0.0.1:15153/api/mind

# Individual collections
curl http://127.0.0.1:15153/api/mind/beliefs
curl http://127.0.0.1:15153/api/mind/opinions
curl http://127.0.0.1:15153/api/mind/goals
curl http://127.0.0.1:15153/api/mind/hypotheses
curl http://127.0.0.1:15153/api/mind/contradictions

# Audit history
curl "http://127.0.0.1:15153/api/mind/history?entity_id={id}"

# Manual controls
curl -X POST http://127.0.0.1:15153/api/mind/beliefs/approve \
  -H "Content-Type: application/json" -d '{"belief_id":"id-here"}'

curl -X PUT http://127.0.0.1:15153/api/mind/beliefs/edit \
  -H "Content-Type: application/json" -d '{"belief_id":"id","content":"updated","confidence":0.9}'
```

### Admin Operations

```bash
# Backfill embeddings for memories without embeddings
curl -X POST http://127.0.0.1:15153/api/embeddings/backfill \
  -H "Content-Type: application/json" -d '{"limit": 100}'

# Drift detection report
curl http://127.0.0.1:15153/api/mind/drift

# Create mind state snapshot
curl -X POST http://127.0.0.1:15153/api/mind/snapshot

# Pipeline debug (requires turn_id)
curl http://127.0.0.1:15153/api/cognitive/{turn_id}

# Live events (SSE)
curl http://127.0.0.1:15153/api/events/stream
```

### Authentication (optional)

If `API_KEY` environment variable is set, include it in requests:

```bash
curl -H "X-API-Key: your-key" http://127.0.0.1:15153/api/mind
```

## Rate Limiting

- **Default**: 50 requests/second per IP with burst of 100
- **429 Too Many Requests** when exceeded
- `Retry-After: 1` header included in rate-limited responses

## Database Migrations

Migrations run automatically on startup. Each migration is versioned and tracked in a `_migrations` collection.

```bash
# View current migration version (in mongosh)
db._migrations.find()
```

The 4 initial migrations create indexes for:
1. `memories` — conversation_id, type, created_at, importance, tags
2. `beliefs` — subject, status, confidence, updated_at
3. Mind collections — opinions, preferences, goals, hypotheses, uncertainties, self_observations, contradictions
4. `messages`, `events`, `memory_history` — audit and event tracking

## Testing

```bash
# Unit tests (no external services required)
go test ./tests/ -v -timeout 120s

# Integration tests (requires MongoDB + Ollama)
go test ./tests/integration/... -v -timeout 120s

# Compile check for integration tests
go test ./tests/integration/... -c -o NUL

# Lint
go vet ./...
```

## Project Structure

```
cmd/requiem/                    # Main entry point
internal/
  api/
    handlers.go               # HTTP handlers (chat, mind, memory, embeddings)
    conversations.go          # Conversation CRUD with pagination
    mind_control.go           # Manual belief/opinion controls
    middleware.go             # CORS, rate limiting, API auth, size limits, timeout
    health.go                 # Health check endpoint
  cognitive/
    pipeline/
      pipeline.go             # Pipeline orchestrator
      stage.go                # Stage interface and I/O types
      stage0.go               # Perception
      stage1.go               # Recall
      stage2.go               # Interpretation
      stage3.go               # Reflection
      stage4.go               # Response
      stage5_response_humanize.go   # Humanization
      stage6.go               # Consolidation
    mind/
      drift.go                # Drift detection
      snapshots.go            # Mind state snapshots
      decay.go                # Memory decay
      hypothesis.go           # Hypothesis promotion
      value.go                # Value modification rules
    prompts/
      prompts.go              # Prompt builders with DATA/INSTRUCTIONS delimiters
  config/
    config.go                 # YAML config loading
    hot_reload.go             # fsnotify-based config hot reload
  events/
    sse.go                    # SSE event bus
  llm/
    interfaces.go             # LLMProvider, EmbeddingProvider interfaces
    ollama.go                 # Ollama provider
    llamacpp.go               # LlamaCpp provider
    openai_compatible.go      # OpenAI-compatible provider
    registry.go               # Provider registry + Mock
    embedding_service.go      # Embedding cache + service
    embeddings.go             # Ollama embedding provider + Mock
  metrics/
    metrics.go                # JSON + Prometheus metrics
  storage/
    interfaces.go             # Store interfaces
    mongo.go                  # MongoStore with transactions + retry
    mongo_memory.go           # Memory store implementation
    mongo_mind.go             # Mind store implementation
    mongo_other.go            # Conversation, events, audit, pipeline runs
    migrations/
      migrations.go           # Versioned migration system
  validation/
    validator.go              # OutputSanitizer, MemoryImportValidator, confidence/importance validation
pkg/models/                   # Data types and models
tests/
  pipeline_test.go            # Unit tests
  integration/
    integration_test.go       # Integration tests
```
