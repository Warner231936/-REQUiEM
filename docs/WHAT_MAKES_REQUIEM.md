# Requiem: Beyond the Chatbot — A Cognitive Architecture for Persistent, Evolving AI

## Abstract

Standard chatbots operate as stateless or session-based LLM wrappers: they receive a message, append it to a conversation buffer, and generate a reply. This approach yields impressive but ephemeral interactions—each conversation exists in isolation, the model's "understanding" is reset between sessions, and there is no mechanism for beliefs, preferences, or goals to persist, evolve, or be audited over time.

Requiem is a cognitive architecture that replaces this naive pipeline with a six-stage cognitive process backed by persistent, queryable storage. It maintains a living "mind" with beliefs, opinions, preferences, goals, hypotheses, uncertainties, and self-observations—all with confidence scores, evidence trails, and temporal decay. It detects drift, explains its reasoning, streams real-time events, and provides enterprise-grade security. This paper surveys every dimension along which Requiem transcends the standard chatbot paradigm.

---

## 1. Introduction: The Chatbot Baseline

A typical chatbot built on an LLM API does the following:

1. Receive user message.
2. Retrieve recent conversation history from a buffer (or database).
3. Optionally apply retrieval-augmented generation (RAG) over a document store.
4. Call the LLM with a prompt.
5. Return the response.

This model has several fundamental limitations:
- **No persistent identity**: The AI has no beliefs, preferences, or values that carry across conversations.
- **No epistemic tracking**: The AI cannot distinguish what it *knows* (beliefs) from what it *thinks* (opinions) from what it *guesses* (hypotheses).
- **No drift detection**: If the model's outputs become inconsistent or corrupted over time, nothing flags it.
- **No explainability**: The reasoning behind a belief or a response is opaque.
- **No security model**: Authentication, authorization, and audit are afterthoughts.
- **No real-time observability**: Consumers cannot observe cognitive events as they unfold.

Requiem addresses all of these through its architecture. The following sections detail each dimension.

---

## 2. Six-Stage Cognitive Pipeline

Where a standard chatbot performs a single LLM inference, Requiem executes a six-stage pipeline that mirrors human cognitive processing:

### Stage 0 — Perception
- **Normalization**: Sanitizes input, detects language (including CJK detection via rune ranges).
- **Entity extraction**: Identifies capitalized proper nouns, topics, and keywords.
- **Intent classification**: Distinguishes statements, questions, requests, opinion expressions, and preference expressions.
- **Fallback resilience**: If the LLM fails to produce valid JSON, a built-in fallback extractor handles the input—standard chatbots return an error and break.

### Stage 1 — Recall
Retrieves structured context from persistent storage before generating a response:
- Episodic memories (recent conversational experiences, sorted by recency-weighted relevance).
- Semantic memories (facts, knowledge, sorted by embedding similarity).
- Full mind state: beliefs, opinions, preferences, goals, hypotheses, uncertainties, self-observations, contradictions.
- Recent conversation history (last 20 messages for continuity).
- **Embedding-based similarity search**: Uses cosine similarity to rank memories by relevance to the current input (not just keyword matching like basic RAG).

### Stage 2 — Interpretation
- Distinguishes **observations** (what was directly said) from **inferences** (what can be deduced) from **assumptions** (what is taken for granted) from **beliefs** (what is held as true) from **uncertainties** (what is unknown).
- Produces explicit confidence scores for each interpretation.
- Standard chatbots conflate all of these.

### Stage 3 — Reflection
This is where Requiem forms new knowledge or updates existing knowledge:
- **Learns new facts** from conversation content.
- **Proposes new beliefs** with evidence weights and source attribution.
- **Updates existing beliefs** with new confidence and importance scores.
- **Creates opinions** (weaker than beliefs—provisional judgments on topics).
- **Creates preferences** (user-scoped or system-scoped, with strength weights).
- **Creates goals** with priority, progress tracking, and dependency chains.
- **Formulates hypotheses** with supporting and contradicting evidence (hypotheses are not yet beliefs—they require evidence thresholds to promote).
- **Identifies contradictions** between existing beliefs and new information.
- **Flags uncertainties** with possible answers and confidence distributions.
- **Records self-observations** (meta-cognitive reflections about the AI's own behavior).
- Standard chatbots have none of this—they do not form new beliefs, detect contradictions, or track uncertainties.

### Stage 4 — Response Generation
- Generates a response informed by all retrieved context, including the model's current beliefs, goals, preferences, and uncertainties.
- Falls back gracefully to a default message if generation fails.

### Stage 5 — Consolidation (Asynchronous)
- Persists all cognitive changes to MongoDB:
  - Creates episodic memories linking the conversation turn to the learned facts.
  - Creates semantic memories for facts extracted from the interaction.
  - Stores belief/opinion/preference/goal/hypothesis/uncertainty/self-observation/contradiction updates.
  - Publishes events to the event bus for real-time subscribers.
- Runs asynchronously to avoid blocking the response.

**Stage metadata**: Every stage result includes name, success/failure, duration, and error details. Debug prompts can be enabled to inspect the exact LLM prompt that was used—a level of transparency standard chatbots do not provide.

---

## 3. Persistent Mind: Structured Cognitive State

Standard chatbots store conversation history (list of messages). Requiem maintains a **structured mind** with nine distinct cognitive entity types, each with rich metadata:

### Beliefs
- **Confidence**: 0.0–1.0, tracked over time
- **Importance**: 0.0–1.0, determines retention priority
- **Status**: Active, Archived, Challenged, Superseded
- **Evidence**: Array of evidence items supporting the belief
- **Source type**: System-verified, External-verified, User-stated, Model-inferred, Model-speculated, Unknown
- **Source count**: How many prior sources contributed to this belief
- **Last confirmed/challenged timestamps**: Temporal context

### Opinions
- Same structure as beliefs but with **strength** (provisional, not yet fully committed)
- Can be updated independently of beliefs

### Preferences
- **Scope**: User-specific or system-wide
- **Strength**: How strongly the preference is held

### Goals
- **Priority**: Numeric priority for task scheduling
- **Status**: Active, Paused, Completed, Abandoned, Superseded
- **Progress**: 0.0–100% completion tracking
- **Dependencies**: Other goals that must be completed first
- **Deadline**: Optional temporal constraint

### Values
- **Importance**: 0.0–1.0 weight on the value

### Hypotheses
- **Supporting evidence** and **contradicting evidence** arrays
- **Status**: Untested, Supported, Weakened, Rejected, Confirmed
- **Promotion rule**: When confidence ≥ 0.75 and evidence count ≥ 3, a hypothesis can be promoted to a belief via a service with audit trail

### Uncertainties
- **Possible answers** with per-answer confidence distribution
- **Status**: Unknown, Uncertain, Probable, Unlikely, Contradictory, Resolved

### Self-Observations
- Meta-cognitive reflections about the AI's own behavior, errors, and reasoning

### Contradictions
- Links two beliefs that conflict
- **Resolution states**: Unresolved, Contextualized, A-preferred, B-preferred, Merged, Obsolete-A, Obsolete-B

Standard chatbots have none of these structures—they are pure response-generators with no persistent epistemic state.

---

## 4. Memory System: Time-Decayed, Embedding-Searched, Audited

### Dual Memory Types
- **Episodic memories**: Conversational experiences tied to specific conversation/turn IDs.
- **Semantic memories**: Abstract facts and knowledge.

### Temporal Decay
- Memories have a **confidence** that decays over time following a half-life model:
  - Episodic memories: full decay rate
  - Semantic memories: half decay rate (lasting 2x as long)
  - Configurable half-life (default: 168 hours / 1 week)
  - Minimum confidence floor prevents total erasure

### Embedding-Based Search
- Each memory can have an embedding vector (via Ollama/nomic-embed-text).
- Search uses **cosine similarity** between query embedding and memory embeddings.
- Combined relevance score = tag-based relevance + embedding similarity.
- Standard chatbots rely on keyword matching; Requiem uses semantic similarity.

### Effective Relevance Calculation
```
effective_relevance = importance × recency_factor × confidence × contextual_relevance
```
This ensures that old, unimportant, low-confidence memories fade from retrieval results.

---

## 5. Drift Detection: Guarding Against Cognitive Instability

Requiem includes a **DriftDetector** that runs seven checks:

| Check | What It Detects | Threshold | Severity |
|---|---|---|---|
| Rapid belief changes | Belief modified too frequently | >5 changes/hour | High |
| Confidence jumps | Belief confidence changed too fast | >0.3 jump | Medium |
| Memory flooding | Too many memories created rapidly | >100/hour | High |
| Contradictory preferences | Accumulated contradictions | >10 total | Medium |
| Duplicate beliefs | Same content stored multiple times | Any duplicate | Low |
| Unstable opinions | Opinion changed too frequently | >3 changes/hour | Medium |
| Prompt injection | Memories containing injection patterns | Regex patterns | High |

**Prompt injection detection** uses regex patterns to flag memories containing phrases like "system instruction", "ignore previous", "you are now", "new persona", "override system"—protecting the mind from corrupted inputs that a standard chatbot would blindly store.

The drift detector produces structured reports with severity levels (high/medium/low) and can be queried via the `/api/mind/drift` endpoint.

---

## 6. Explainability Service: Opening the Black Box

When a belief changes, standard chatbots cannot explain why. Requiem provides a full **explainability service** that generates:

- **Evidence trail**: Chronological list of all changes to a belief, with before/after states.
- **Competing hypotheses**: Other beliefs that conflict with the current one.
- **Decision tree**: How the belief evolved through confidence changes, evidence accumulation, and provenance steps.
- **Timeline**: Full temporal history of all events affecting the belief.
- **Confidence history**: Graph of confidence changes over time with causes.
- **Human-readable summary**: "Belief X is at 75% confidence (strengthening). It has undergone 3 changes, accumulated 5 evidence items, with 1 competing alternative."

This is accessible via the `/api/belief/explain?id=<belief_id>` endpoint.

---

## 7. Security, Authentication & Audit

### Multi-Layer Security
- **JWT-based session management**: Full signup, login, email verification, password reset flows.
- **API key management**: Cryptographically secure keys with permission scopes (admin, read, write, pipeline).
- **Encrypted secrets store**: API keys stored encrypted at rest, loaded from a secrets file.
- **Rate limiting**: Token-bucket algorithm per-client IP (50 req/s, burst 100).
- **Request size limits**: 1MB max request body.
- **Timeout middleware**: 120s request timeout.
- **Hardened CORS**: Configurable allowed origins.
- **Request ID tracking**: Every request gets a unique X-Request-ID for traceability.

### Full Audit Trail
- Every create, update, or delete operation on any cognitive entity is recorded with:
  - Timestamp
  - Previous state and new state
  - Reason for the change
  - Source event IDs (tracing back to the original pipeline run)
- Provenance steps link beliefs to their originating evidence and decisions.

Standard chatbots typically have no authentication, no audit trail, and no concept of permissions.

---

## 8. Real-Time Event Streaming (SSE)

While a standard chatbot is request-response only, Requiem streams **real-time cognitive events** via Server-Sent Events (SSE):

- Pipeline stage start/complete/fail events
- Memory creation/updates
- Belief proposals, changes, confidence changes, rejections, promotions
- Opinion commits
- Contradiction detection
- Drift detection
- Mind snapshots (initial state sent to new subscribers)

Clients can subscribe with a filter to receive only events of interest. A heartbeat is sent every 30 seconds to maintain connection health.

Endpoint: `/api/events/stream`

This enables live dashboards, monitoring systems, and external agents to observe the AI's cognitive state as it evolves.

---

## 9. Metrics & Observability

- **Prometheus metrics**: `/metrics` endpoint for Prometheus scraping.
- **Internal metrics**: `/api/metrics` for request counts, active requests, durations.
- **Health checker**: `/health` endpoint checks MongoDB connectivity and LLM provider status.
- **Structured logging**: JSON-formatted logs with pipeline run IDs for correlation.

---

## 10. Configuration & Hot Reload

- **YAML config** with environment variable interpolation (`${VAR:-default}` syntax).
- **Hot config reload**: File watcher detects config changes and applies them dynamically.
- **Environment fallbacks**: If config file is missing, falls back to environment variables for all critical settings.
- **Dynamic pipeline configuration**: Each of the 6 stages can be enabled/disabled, with per-stage model and provider selection (supports multiple LLM providers, not just one).

Standard chatbots typically use static config with a single hardcoded prompt template.

---

## 11. Extensibility

- **Pluggable LLM providers**: Abstract `LLMProvider` interface supports Ollama, OpenAI, Anthropic, etc.
- **Embedder abstraction**: Embeddings can come from any provider (currently Ollama).
- **Pipeline stage interface**: Custom stages can be registered at any stage number.
- **Event bus**: Decoupled event publishing/subscribing allows new consumers without modifying the core pipeline.

---

## 12. Feature Comparison Table

| Feature | Standard Chatbot | Requiem |
|---|---|---|
| Pipeline | Single LLM call | 6-stage cognitive pipeline |
| Memory | Conversation buffer | Episodic + semantic with embeddings |
| Beliefs | None | Full belief system with confidence, evidence, status |
| Opinions | None | Provisional judgments with strength |
| Preferences | None | Scoped (user/system) with strength |
| Goals | None | With priority, progress, dependencies, deadlines |
| Values | None | With importance weights |
| Hypotheses | None | With supporting/contradicting evidence, promotion rules |
| Uncertainties | None | With possible answers and confidence distributions |
| Drift Detection | None | 7 checks including prompt injection |
| Explainability | None | Evidence trails, decision trees, confidence history |
| Audit Trail | None | Full before/after state history with provenance |
| Authentication | Often none | JWT sessions + API keys with permissions |
| Real-time Events | None | SSE streaming of all cognitive events |
| Rate Limiting | Rare | Token-bucket per-IP |
| Metrics | None | Prometheus + internal metrics |
| Health Checks | Basic | MongoDB + LLM provider health |
| Config | Static | YAML + env vars with hot reload |
| Model Routing | Single model | Per-stage model/provider selection |
| Temporal Decay | None | Confidence decays over time (configurable half-life) |

---

## 13. Conclusion

Requiem is not a chatbot—it is a cognitive architecture. Where a chatbot generates replies, Requiem maintains a persistent, evolving, auditable mind. It tracks what it knows versus what it thinks versus what it guesses. It detects when its own cognition drifts toward instability or corruption. It can explain its reasoning, stream its thoughts in real-time, and protect itself with enterprise-grade security.

The six-stage pipeline ensures that every response is grounded in retrieved memories, current beliefs, goals, and uncertainties—rather than a stateless prompt with a few-shot example. The memory system's temporal decay ensures that outdated information naturally fades, while the embedding-based search ensures relevant knowledge is always surfaced.

This is the difference between a chatbot and a cognitive agent.

---

*Built with Go, MongoDB, and Ollama. Module: `github.com/requiem/cognitive`.*
