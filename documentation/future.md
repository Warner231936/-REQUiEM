# Future Development Roadmap

## Short-term Enhancements (1-3 months)

### Vector Database Integration
- MongoDB Atlas Vector Search — Add native vector search index for memories, replacing in-memory cosine similarity for production-scale deployments
- Alternative vector stores — Support for Qdrant, Pinecone, or Weaviate as external vector stores
- Embedding similarity threshold configuration — Make the 0.5 default threshold configurable per-stage via config.yaml

### Enhanced Security
- JWT authentication — Replace optional API key auth with full JWT token validation and refresh
- Rate limiting by API key — Different rate limits per API key tier
- Audit log export — Stream audit records to external SIEM via syslog or HTTP webhook
- PII redaction — Automatic redaction of personally identifiable information from stored memories

### Observability
- Grafana dashboard template — Pre-built dashboard importing the Prometheus metrics
- Distributed tracing — OpenTelemetry integration for pipeline stage spans
- Alerting rules — Prometheus alertmanager rules for drift detection, pipeline errors, LLM failures

### Memory Management
- Memory aging policies — Configurable retention policies per memory type (e.g., episodic memories expire after 30 days, semantic after 365 days)
- Memory summarization — LLM-powered summarization of old conversations to compress context
- Vector index backfill tool — CLI utility for bulk embedding generation and backfill (partial implementation: `/api/embeddings/backfill` endpoint exists)

## Medium-term Roadmap (3-6 months)

### Multi-Agent Architecture
- Agent spawning — Ability to spawn sub-agents for specific tasks (e.g., web research, code execution)
- Agent registry — Track and manage multiple cognitive agents with shared or isolated memory
- Inter-agent communication — Standardized protocol for agents to share beliefs and resolve contradictions

### Advanced Cognitive Features
- Temporal reasoning — Add time-based belief validity windows (beliefs expire after a TTL)
- Causal inference — Extend reflection stage to propose causal models and test hypotheses
- Meta-cognition — System tracks its own confidence calibration and adjusts thresholds dynamically
- Multi-modal input — Support image, audio, and file attachments in addition to text

### LLM Orchestration
- Model routing — Dynamic model selection based on task complexity, cost, and latency
- Model ensembles — Run multiple models per stage and aggregate results
- LLM-as-a-judge — Use a separate LLM call to evaluate/critique the output of another LLM call
- Streaming responses — Stream Stage 4 (Response) output back to the client via SSE

### Data Management
- Schema migration tooling — CLI for managing migration up/down with rollback safety
- Backup/restore — Automated MongoDB backup and restore for the cognitive state
- Data export/import — Export beliefs, memories, and mind state as portable JSON/YAML
- Multi-tenant isolation — Separate databases or namespaces for different users/organizations

## Long-term Vision (6-12 months)

### Distributed Cognitive System
- Horizontal scaling — Multiple Requiem instances behind a load balancer with shared MongoDB
- Fibrated cognition — Multiple instances sharing beliefs and memories across a federation
- Edge deployment — Lightweight version for on-device inference (llama.cpp backend only)

### Advanced Reasoning
- Symbolic reasoning engine — Integrate a rule engine for deterministic logic alongside LLM reasoning
- Plan generation and execution — Full planning stack from goal decomposition to action execution
- Theory of mind — Model other agents' beliefs and intentions for multi-agent interaction

### User Experience
- Web UI — React/Vue frontend for mind state visualization and manual belief management
- Mobile app — iOS/Android client with offline capability
- Voice interface — Speech-to-text and text-to-speech integration

### Integration Ecosystem
- Plugin system — Extend the pipeline with custom plugins (e.g., web search, database queries)
- API marketplace — Pre-built connectors for Slack, Discord, email, calendar, etc.
- LLM provider marketplace — Easy add new LLM backends via config

## Research Directions

### Cognitive Science Alignment
- ACT-R compatibility — Map pipeline stages to ACT-R cognitive modules
- SOAR integration — Explore hybrid symbolic/subsymbolic reasoning
- Phenomenology of AI — Study the relationship between explicit belief tracking and emergence of self-models

### Evaluation Framework
- Cognitive benchmark — Standardized tests for belief consistency, memory retention, and reasoning quality
- Adversarial testing — Automated red-teaming for prompt injection and manipulation resistance
- Longitudinal studies — Track cognitive drift and belief evolution over extended interactions

---

## Envisioned Enhancements Summary

| Enhancement | Complexity | Impact | Current Status |
|---|---|---|---|
| Prometheus metrics | Low | High | Done |
| Grafana dashboard template | Low | Medium | Not started |
| Config hot reload | Low | High | Done |
| Database migrations | Low | High | Done |
| LlamaCpp/OpenAI providers | Medium | High | Done |
| MemoryImportValidator | Medium | High | Done |
| Rate limiting + API auth | Medium | High | Done |
| DATA delimiters | Medium | High | Done |
| Transaction retry | Medium | High | Done |
| Vector DB integration | Medium | High | Partial (in-memory cosine similarity) |
| JWT auth | Medium | Medium | Not started |
| OpenAPI spec | Low | Medium | Not started |
| Multi-modal input | Medium | Medium | Not started |
| LLM model routing | Medium | Medium | Not started |
| Web UI | High | High | Not started |
| Multi-agent support | High | High | Not started |
| Symbolic reasoning | High | High | Not started |
| Mobile app | High | Medium | Not started |
| Voice interface | Medium | Medium | Not started |
| OpenTelemetry tracing | Medium | High | Not started |
| Plugin system | High | High | Not started |
