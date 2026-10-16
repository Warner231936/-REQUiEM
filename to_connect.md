# To Connect — Connection Protocols & Integration

This document describes how external clients and services connect to the Requiem cognitive system.

---

## Connection Protocols

### HTTP/REST API (Primary)

The system exposes a RESTful HTTP API with JSON payloads. All endpoints are served over HTTP on the configured port (default `15153`).

**Base URL**: `http://127.0.0.1:15153`

**Content-Type**: `application/json` for all request and response bodies

**Authentication** (optional):
- Header: `X-API-Key: <your-key>`
- Or: `Authorization: Bearer <your-key>`
- Set via `API_KEY` environment variable

---

## API Endpoint Summary

### Chat
```
POST /api/chat
Body: { "conversation_id": "opt", "message": "text", "metadata": {} }
Response: { "conversation_id", "turn_id", "response", "duration_ms", "stage_outputs": [...] }
```

### Mind State
```
GET  /api/mind                          # Full mind state
GET  /api/mind/beliefs                  # List beliefs
GET  /api/mind/opinions                 # List opinions
GET  /api/mind/preferences              # List preferences
GET  /api/mind/values                   # List values
GET  /api/mind/goals                    # List goals
GET  /api/mind/hypotheses               # List hypotheses
GET  /api/mind/uncertainties            # List uncertainties
GET  /api/mind/self                     # Self-observations
GET  /api/mind/contradictions           # Contradictions
GET  /api/mind/history?entity_id={id}   # Audit history
```

### Mind Controls
```
POST /api/mind/beliefs/approve         # Approve a belief
POST /api/mind/beliefs/reject          # Reject a belief
PUT  /api/mind/beliefs/edit            # Edit a belief
POST /api/mind/beliefs/archive         # Archive a belief
POST /api/mind/beliefs/restore         # Restore a belief
POST /api/mind/beliefs/merge           # Merge two beliefs
POST /api/mind/beliefs/invalidate      # Invalidate a belief
POST /api/mind/contradictions/resolve  # Resolve a contradiction
POST /api/mind/opinions/approve       # Approve an opinion
POST /api/mind/opinions/reject         # Reject an opinion
POST /api/mind/snapshots               # Create a mind state snapshot
```

### Memory
```
GET  /api/memory?type={episodic|semantic}&tags={t1,t2}&limit=20
GET  /api/memory/{id}
GET  /api/memory/{id}/provenance
```

### Conversations
```
GET    /api/conversations?limit=20&offset=0
POST   /api/conversations               # { "title": "optional" }
GET    /api/conversations/{id}?limit=50&offset=0
DELETE /api/conversations/{id}
```

### Admin
```
POST /api/embeddings/backfill           # { "limit": 100 }
GET  /api/mind/drift                    # Drift detection report
POST /api/mind/snapshot                 # Create snapshot
```

### Observability
```
GET /health                             # Health status (JSON)
GET /api/metrics                        # Metrics (JSON)
GET /metrics                            # Prometheus format
GET /api/events/stream                  # SSE live events
GET /api/cognitive/{turn_id}            # Pipeline debug trace
```

### CORS

CORS is enabled by default with permissive headers. All standard HTTP methods are allowed. The server returns `204 No Content` for OPTIONS preflight requests.

---

## CORS Headers

| Header | Value |
|--------|-------|
| `Access-Control-Allow-Origin` | `*` (or request origin) |
| `Access-Control-Allow-Methods` | `GET, POST, PUT, DELETE, OPTIONS` |
| `Access-Control-Allow-Headers` | `Content-Type, Authorization, X-Requested-With` |
| `Access-Control-Max-Age` | `86400` (24 hours) |

---

## Rate Limiting

- **Algorithm**: Token bucket per client IP
- **Rate**: 50 requests/second
- **Burst**: 100 requests
- **Response on limit**: `429 Too Many Requests` with `Retry-After: 1` header
- **Client identification**: IP address (supports `X-Forwarded-For` and `X-Real-IP` for proxied deployments)

---

## Request Constraints

| Constraint | Limit |
|---|---|
| Request body size | 1 MB (`RequestSizeLimit` middleware) |
| Write timeout | 120 seconds |
| Read timeout | 30 seconds |
| Idle timeout | 60 seconds |
| Header timeout | 10 seconds (`ReadHeaderTimeout`) |

---

## Response Format

### Success

All successful responses return HTTP 200 (or 201 for creation) with JSON body:

```json
{
  "conversation_id": "uuid",
  "turn_id": "uuid",
  "response": "Hello!",
  "duration_ms": 1250,
  "stage_outputs": [
    {
      "stage": 0,
      "name": "perception",
      "success": true,
      "duration_ms": 45
    }
  ]
}
```

### Error

```json
{
  "error": "descriptive error message"
}
```

### Error Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 201 | Resource created |
| 400 | Bad request (invalid JSON, missing fields) |
| 401 | Unauthorized (invalid or missing API key) |
| 404 | Resource not found |
| 405 | Method not allowed |
| 413 | Request entity too large (> 1MB) |
| 429 | Rate limit exceeded |
| 500 | Internal server error |
| 503 | Service unavailable (degraded health) |

---

## Connection Flow (Chat)

```
1. Client → POST /api/chat
2. CORS middleware → adds CORS headers
3. Rate limiter → checks token bucket, returns 429 if exceeded
4. Request size limit → rejects bodies > 1MB
5. Timeout middleware → wraps context with timeout
6. Request count tracker → increments ActiveRequests gauge
7. Handler → validates input (MemoryImportValidator)
8. Pipeline Stage 0 (Perception, reqiquick)
   - Sanitizes input
   - Extracts language, topics, entities, intent
   - Fallback: deterministic regex-based extraction
9. Pipeline Stage 1 (Recall, no LLM)
   - Queries MongoDB for episodic/semantic memories
   - Retrieves beliefs, opinions, preferences, goals
   - Embedding similarity scoring (cosine, threshold 0.5)
10. Pipeline Stage 2 (Interpretation, requiem1)
    - Intent/goal determination with DATA delimiters
    - Fallback: deterministic context extraction
11. Pipeline Stage 3 (Reflection, requiem1)
    - Proposal generation with DATA delimiters
    - Fallback: minimal observation proposal
12. Pipeline Stage 4 (Response, reqiquick)
    - Natural language response generation
    - Fallback: "I understand your message..."
13. Pipeline Stage 5 (Consolidation, reqiquick) — async by default
    - Transaction with retry (3 attempts)
    - Output sanitization before DB writes
    - Embedding generation for new memories
    - Audit history recording
    - Contradiction detection
14. Response count tracker → decrements ActiveRequests gauge
15. HTTP response → JSON to client
```

---

## WebSocket / SSE

### SSE (Server-Sent Events)

Connect to `/api/events/stream` for live pipeline events:

```javascript
const eventSource = new EventSource('http://127.0.0.1:15153/api/events/stream');
eventSource.onmessage = (event) => {
  const data = JSON.parse(event.data);
  // { event: "stage.completed", stage: 3, turn_id: "...", ... }
};
```

Event types:
- `stage.started`
- `stage.completed`
- `stage.failed`
- `pipeline.started`
- `pipeline.completed`

---

## External Service Connections

### MongoDB

- **Protocol**: MongoDB wire protocol (TCP)
- **Default URI**: `mongodb://127.0.0.1:27017`
- **Default database**: `requiem`
- **Collections**: memories, beliefs, opinions, preferences, values, goals, hypotheses, uncertainties, self_observations, contradictions, messages, events, memory_history, pipeline_runs, _migrations
- **Index creation**: Automatic on startup via migrations
- **Transaction support**: Automatic (requires replica set; falls back to sequential writes on standalone)

### Ollama

- **Protocol**: HTTP
- **Default URL**: `http://127.0.0.1:11434`
- **Endpoints used**:
  - `POST /api/chat` — LLM text generation
  - `GET /api/tags` — Health check
  - `POST /api/embeddings` — Embedding generation
- **Required models**: `reqiquick` (stages 0/4/5), `requiem1` (stages 2/3), `nomic-embed-text` (embeddings)

### LlamaCpp Server

- **Protocol**: HTTP (OpenAI-compatible)
- **Default URL**: `http://127.0.0.1:8080`
- **Endpoints used**:
  - `POST /v1/chat/completions` — LLM text generation
  - `GET /v1/models` — Health check
- **Configure**: Add to `config.yaml` under `llm.providers.llamacpp`

### OpenAI-Compatible APIs

- **Protocol**: HTTPS
- **Endpoints used**:
  - `POST /v1/chat/completions` — LLM text generation
  - `GET /v1/models` — Health check
- **Authentication**: Bearer token via `Authorization` header
- **Configure**: Add to `config.yaml` under `llm.providers.openai` with `base_url` and `api_key`

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `MONGODB_URI` | `mongodb://127.0.0.1:27017` | MongoDB connection string |
| `MONGODB_DATABASE` | `requiem` | Database name |
| `OPENAI_API_KEY` | — | API key for OpenAI-compatible provider |
| `COGNITIVE_DEBUG` | `false` | Enable debug logging |
| `API_KEY` | — | API key for optional request authentication |
| `PORT` | `15153` | API server port (override config) |
| `HOST` | `127.0.0.1` | API server host (override config) |

> Note: `MONGODB_URI` and `MONGODB_DATABASE` are expanded from environment variables in config.yaml via `${VAR:-default}` syntax.

---

## Connection Examples

### Basic chat

```bash
curl -X POST http://127.0.0.1:15153/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"What is your purpose?"}'
```

### With API key

```bash
curl -X POST http://127.0.0.1:15153/api/chat \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-secret-key" \
  -d '{"message":"Hello"}'
```

### Via proxy (behind nginx)

```nginx
location /api/ {
    proxy_pass http://127.0.0.1:15153;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header Host $host;
    client_max_body_size 1m;
}
```

### Docker

```dockerfile
FROM golang:1.22 AS builder
WORKDIR /app
COPY . .
RUN go build -o requiem ./cmd/requiem

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/requiem /usr/local/bin/
EXPOSE 15153
CMD ["requiem"]
```

```yaml
# docker-compose.yml
services:
  requiem:
    build: .
    ports:
      - "15153:15153"
    environment:
      - MONGODB_URI=mongodb://mongo:27017
      - OLLAMA_URL=http://ollama:11434
    depends_on:
      - mongo
      - ollama

  mongo:
    image: mongo:7
    ports:
      - "27017:27017"

  ollama:
    image: ollama/ollama
    ports:
      - "11434:11434"
```
