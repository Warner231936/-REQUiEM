# REQUiEM — Architecture Document

**Document:** ARCH-001  
**Date:** 2026-10-14  
**Classification:** Internal  
**Version:** v1.0 (post-Phase 8)  

---

## 1. System Overview

REQUiEM is a cognitive architecture composed of two Go modules that communicate across an HTTP boundary:

1. **`github.com/requiem/cognitive`** (root, Go 1.22) — the cognitive kernel: a seven-stage reasoning pipeline with LLM abstraction, belief management, evidence tracking, and verification policy enforcement.
2. **`github.com/kilo/spiral-codemaker`** (subdirectory `ReqiCode/`, Go 1.27.1) — the engineering subsystem: a spiral development loop driven by a potential function and convergence detection.

The cognitive kernel decides *what* to do and *whether* it is confident; ReQiCode executes the *how* of engineering tasks and reports back results that feed into the verification policy.

---

## 2. Component Map

### 2.1 Cognitive Kernel (`github.com/requiem/cognitive`)

```
cmd/requiem/                    # Entry point, CLI, config watching
├── main.go                     # Pipeline assembly, signal handling, hot reload

internal/
├── api/                        # HTTP API server
│   ├── handlers.go             # Request handlers
│   ├── auth.go                 # Authentication middleware
│   ├── middleware.go           # CORS, logging, request ID
│   ├── conversations.go        # Conversation management
│   └── health.go               # Health check endpoint

├── cognitive/
│   ├── integration/
│   │   └── reqicode_client.go   # HTTP client → ReQiCode API
│   ├── pipeline/                # The 7-stage pipeline
│   │   ├── pipeline.go          # Pipeline orchestrator + Context
│   │   ├── stage.go             # Stage interface + helpers
│   │   ├── stage0.go            # Boot & initialization
│   │   ├── stage1.go            # Ingestion & parsing
│   │   ├── stage2.go            # Belief & memory update
│   │   ├── stage3.go            # Planning & tool selection
│   │   ├── stage4.go            # Response & tool execution
│   │   ├── stage5.go            # Evidence processing
│   │   ├── stage6.go            # Consolidation
│   │   ├── verification_policy.go  # 0.8 confidence gate, ReviseClaim
│   │   └── explainability.go    # Pipeline-level explanations
│   ├── mind/                    # Cognitive mind (beliefs, drift, memory)
│   │   ├── beliefs.go           # Belief network operations
│   │   ├── drift.go             # Belief drift detection
│   │   ├── services.go          # Belief query services
│   │   └── snapshot.go          # Memory snapshot/restore
│   └── tool/
│       ├── registry.go          # Tool registry with schema validation
│       ├── builtin_tools.go     # Calculator, web fetch, file ops
│       └── reqicode.go          # ReQiCodeTool (bridge to ReQiCode)

├── events/
│   └── events.go               # EventBus with 20+ typed events

├── explainability/
│   └── explainability.go       # Explanation, EvidenceItem, HypothesisInfo, DecisionNode

├── llm/                        # LLM abstraction layer
│   ├── interfaces.go           # Provider, ProviderRegistry, EmbeddingProvider
│   ├── registry.go             # Provider factory registry with lazy init
│   ├── ollama.go               # Ollama provider (local)
│   ├── openai_compatible.go    # OpenAI-compatible provider
│   ├── llamacpp.go             # llama.cpp provider (local)
│   ├── embedding_service.go    # Embedding provider abstraction
│   └── embeddings.go           # Embedding storage/retrieval

├── storage/                    # Storage abstraction
│   ├── interfaces.go           # Store interfaces (6 stores)
│   ├── mongo.go                # MongoDB implementations
│   ├── mongo_other.go          # Cross-collection aggregation
│   ├── mongo_mind.go           # Belief-specific MongoDB queries
│   ├── mongo_memory.go         # Memory snapshot/restore
│   └── errors.go              # Storage error types

├── config/
│   └── config.go               # Config loading, hot reload, env defaults

├── validation/
│   └── validator.go            # OutputSanitizer, MemoryImportValidator

├── logger/                     # Structured logging (slog)

pkg/
├── models/                     # Shared typed models
│   ├── base.go                 # IDs, timestamps, confidence types
│   ├── evidence.go             # Claim, EvidenceRecord, VerificationPlan, etc.
│   ├── mind.go                 # Belief, BeliefNetwork, Contradiction, Decision
│   ├── memory.go               # MemoryEntry, MemorySnapshot
│   ├── stages.go               # Pipeline stage enums
│   ├── pipeline.go             # Pipeline config types
│   └── tool.go                 # ToolCall, ToolResult types
```

### 2.2 ReQiCode Engineering Subsystem (`github.com/kilo/spiral-codemaker`)

```
ReqiCode/
├── api/                        # HTTP API server
│   ├── server.go               # Router, middleware, server setup
│   ├── engineering.go          # /api/engineering endpoint
│   └── runmanager.go           # /api/runmanager endpoint

├── core/
│   ├── converge/               # Convergence detection (Phase 0)
│   │   ├── potential.go        # Potential function (Lyapunov-like scalar)
│   │   └── convergence.go      # Trajectory tracking, phase classification
│   ├── state/                  # State management
│   │   ├── types.go            # SemiState type definitions
│   │   ├── semi_state.go       # Mutable state operations
│   │   └── chain.go            # Tamper-evident revision chain (SHA-256)
│   ├── system/
│   │   └── orchestrator.go     # Spiral loop with 10 roles
│   ├── verify/                 # Output verification
│   └── engine/                 # Code generation engine

├── domain/                     # Business logic types
├── infrastructure/             # External adapters
├── interfaces/                 # API interface definitions
├── pkg/                        # Internal packages
└── cmd/requicode/              # ReQiCode entry point
```

---

## 3. Data Flow

### 3.1 Cognitive Pipeline Data Flow

```
┌─────────────────────────────────────────────────────────┐
│                    PipelineContext                      │
│  (threaded through all stages)                         │
│                                                         │
│  Fields:                                                 │
│  - Config (config.Config)                                │
│  - LLM Provider (llm.Provider)                           │
│  - Tool Registry (*tool.ToolRegistry)                    │
│  - Store references (storage interfaces)                 │
│  - Verification Policy (*VerificationPolicy)             │
│  - EventBus (*events.EventBus)                           │
│  - Input, Output, Claims, Evidence, Errors               │
└─────────────────────────────────────────────────────────┘
```

**Stage transitions** are sequential (Stage 0 → 1 → ... → 6). Each stage:
1. Reads from `PipelineContext`
2. Performs its processing
3. Writes results back to `PipelineContext`
4. Publishes events to `EventBus`
5. Returns updated context + any errors to the pipeline orchestrator

### 3.2 Verification Escalation Flow

```
Stage 4 / Stage 6
    │
    ├── Claim detected (confidence < 1.0)
    │
    ├── VerificationPolicy.EvaluateClaim()
    │   │
    │   ├── Confidence >= 0.8? → Accept automatically
    │   │   (Evidence count >= 3? → Accept without escalation)
    │   │
    │   └── Confidence < 0.8? → Requires verification
    │       │
    │       ├── Create VerificationPlan (verification_plans collection)
    │       │
    │       ├── SubmitTask() → ReQiCode HTTP API
    │       │   │
    │       │   ├── ReQiCode orchestrator receives task
    │       │   ├── Runs spiral loop (potential function)
    │       │   ├── Produces verification result
    │       │   └── Returns result via HTTP
    │       │
    │       ├── PollTask() (exponential backoff)
    │       │
    │       └── Record VerificationResult (verification_results collection)
    │           │
    │           └── ReviseClaim() if needed (controlled correction)
```

### 3.3 Storage Data Flow

```
┌────────────────────────────────────────────────────┐
│  Pipeline / Stage                                │
│         │                                           │
│         ▼                                           │
│  EvidenceStore  ───►  evidence_records (MongoDB)   │
│  ClaimStore     ───►  claims (MongoDB)             │
│  VerificationStore ─►  verification_plans,        │
│                        verification_results         │
│  MemoryStore    ───►  memory_entries (MongoDB)     │
│  MindStore      ───►  mind_beliefs (MongoDB)       │
│  ConversationStore ─►  conversations (MongoDB)      │
│         │                                           │
└─────────┼──────────────────────────────────────────┘
          │ (EventBus)
          ▼
   ┌─────────────┐
   │ Events      │
   │ (in-process)│
   └─────────────┘
```

**MongoDB collection names:**
| Collection | Purpose |
|---|---|
| `memory_entries` | Short-term memory key-value entries |
| `mind_beliefs` | Belief network nodes and links |
| `conversations` | Conversation history |
| `evidence_records` | Evidence supporting or opposing claims |
| `claims` | Detected claims with confidence scores |
| `verification_plans` | Plans for verifying low-confidence claims |
| `verification_results` | Results from ReQiCode verification |
| `memory_snapshots` | Point-in-time memory snapshots |
| `memory_indices` | Memory retrieval indices |
| `drift_log` | Belief drift log entries |
| `drift_entries` | Drift detection records |

---

## 4. Key Types

### 4.1 Evidence Domain Model (`pkg/models/evidence.go`)

```go
type Claim struct {
    ID          string
    Text        string
    Confidence  Confidence          // typed float64 0.0–1.0
    Evidence    []EvidenceItemID
    Source      EvidenceSource      // enum: LLM, User, Tool, ReQiCode
    Status      ClaimStatus         // Pending, Accepted, Rejected, Verified, Refuted
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type EvidenceRecord struct {
    ID         string
    Source     EvidenceSource      // LLM, User, Tool, ReQiCode
    Strength   float64             // 0.0–1.0
    ClaimID    string
    Metadata   map[string]any
    CreatedAt  time.Time
}

type VerificationPlan struct {
    ID         string
    ClaimIDs   []string
    Strategy   VerificationStrategy
    Priority   int
    Deadline   time.Time
    Status     VerificationPlanStatus
}

type VerificationResult struct {
    PlanID      string
    Status      VerificationStatus   // Verified, Refuted, Partial, Failed
    Confidence  Confidence
    Evidence    []EvidenceRecord
    Notes       string
    CompletedAt time.Time
}

type RevisionDecision struct {
    OriginalClaim    string
    RevisedClaim     string
    NewConfidence    Confidence
    Rationale        string
    Timestamp        time.Time
    SupportingEvidence []EvidenceRecord
}
```

### 4.2 Pipeline Context (`internal/cognitive/pipeline/pipeline.go`)

The pipeline threads a `Context` struct through all stages. Key fields:
- `Config` — loaded configuration
- `LLM` — primary LLM provider
- `Tools` — registered tool registry
- `Store` — storage interfaces (evidence, claim, verification, memory, mind, conversation)
- `Policy` — verification policy
- `EventBus` — event bus for lifecycle events
- `PipelineInput` / `PipelineOutput` — user input and system response

### 4.3 SemiState (`core/state/types.go` + `semi_state.go`)

Mutable working state for a single spiral iteration:
- `Revision` — integer revision counter
- `Requirements` — parsed requirements
- `Findings` — detected issues and observations
- `Proposals` — file modification proposals
- `Decisions` — architectural/design decisions
- `ArchitecturePlan` — plan with components and endpoints
- `TestResults` — results from test execution
- `EvidenceLog` — accumulated evidence
- `Contradictions` — detected contradictions
- `Objections` — raised objections
- `GeneratedFiles` — files produced in this iteration
- `Confidence` — aggregate confidence score

### 4.4 Potential (`core/converge/potential.go`)

```go
type Potential struct {
    Revision         int
    Value            float64    // weighted sum
    TestPassRate     float64    // 0.32
    CompileSuccess   float64    // 0.18
    EvidenceDensity  float64    // 0.13
    ResolutionRate   float64    // 0.08
    ConfidenceTerm   float64    // 0.12
    DecisionStability float64   // 0.05
    ConflictPenalty  float64    // -0.07
    ObjectionPenalty float64    // -0.05
    HasTestResult    bool
    HasFiles         bool
}
```

---

## 5. Architectural Decisions

### 5.1 Two-Module Separation (ADR-001)
**Status:** Accepted  
**Context:** The cognitive pipeline and engineering subsystem have different dependency requirements (Go 1.22 vs 1.27.1) and different scaling concerns. Tightly coupling them would create circular dependency risks and deployment complexity.  
**Decision:** Separate Go modules with HTTP as the inter-module interface. The cognitive kernel calls ReQiCode via a typed HTTP client (`reqicode_client.go`), never importing its types directly.

### 5.2 Potential Function Over maxIterations (ADR-002)
**Status:** Accepted  
**Context:** Fixed iteration counts are brittle — they either terminate prematurely or waste cycles. The system needs a principled termination criterion.  
**Decision:** Implement a Lyapunov-like potential function over SemiState. The spiral loop classifies trajectory shape (productive, plateau, oscillating, regressing) and terminates on plateau. This is inspired by control theory and dynamical systems.

### 5.3 Confidence-Gated Verification (ADR-003)
**Status:** Accepted  
**Context:** The system produces claims of varying confidence. Manually reviewing all claims is infeasible.  
**Decision:** 0.8 confidence threshold with evidence count minimum (3). Claims below the gate are automatically submitted to ReQiCode for engineering verification. This keeps the conversation flowing while ensuring technical accuracy.

### 5.4 Pareto Frontier for Best Revision (ADR-004)
**Status:** Accepted  
**Context:** A spiral loop may produce a poor result in its final exploratory iteration, overwriting a good result from an earlier revision.  
**Decision:** Track non-dominated revisions on a Pareto frontier across (pass rate, coverage, token cost, complexity). The system can always return the best revision found, regardless of the current iteration's quality.

### 5.5 Tamper-Evident Revision Chain (ADR-005)
**Status:** Accepted  
**Context:** State history must be auditable. Without tamper detection, malicious or buggy state transitions could go unnoticed.  
**Decision:** Each revision stores the SHA-256 hash of the previous revision's state fingerprint. Verifying the chain detects any state manipulation.

### 5.6 In-Memory Mock Stores for Testing (ADR-006)
**Status:** Accepted  
**Context:** Tests must run in CI without MongoDB or Ollama dependencies.  
**Decision:** Define storage interfaces in `internal/storage/interfaces.go`. Implementations include both MongoDB (production) and in-memory mocks (testing). All 191 mock-based tests pass using only mocks; 19 additional live integration tests pass against real MongoDB.

### 5.7 Typed Event Bus (ADR-007)
**Status:** Accepted  
**Context:** Pipeline stages need loose coupling. String-based events are error-prone.  
**Decision:** Implement a type-safe `EventBus` in `internal/events/events.go` with 20+ structured event types. Events are broadcast synchronously to registered handlers.

---

## 6. Deployment View

```
┌─────────────────────────────────────────────────────────┐
│  Client (CLI / API consumer)                            │
└────────────┬────────────────────────────────────────────┘
             │ HTTP
             ▼
┌─────────────────────────────────────────────────────────┐
│  cmd/requiem (cognitive kernel)                         │
│  ┌───────────────────────────────────────────────────┐  │
│  │ Pipeline (7 stages)                               │  │
│  │ ├─ LLM Providers (Ollama/OpenAI/llama.cpp)        │  │
│  │ ├─ Tool Registry                                    │  │
│  │ ├─ Verification Policy (0.8 gate)                  │  │
│  │ └─ Storage (MongoDB or mock)                        │  │
│  └──────────────────┬─────────────────────────────────┘  │
│                     │ HTTP (to ReQiCode)                  │
│                     ▼                                      │
│  ┌───────────────────────────────────────────────────┐  │
│  │ ReQiCode Integration Client                      │  │
│  │ (SubmitTask, GetTaskResult, PollTask, CancelTask)  │  │
│  └──────────────────┬─────────────────────────────────┘  │
└─────────────────────┼────────────────────────────────────┘
                      │ HTTP
                      ▼
┌─────────────────────────────────────────────────────────┐
│  ReQiCode Subsystem                                    │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │ API Server   │  │ Orchestrator │  │ Converger     │   │
│  │ (Go 1.27.1)  │  │ (10 roles)   │  │ (Potential)   │   │
│  └──────────────┘  └──────────────┘  └──────────────┘   │
└─────────────────────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│  ReQiCode Execution Layer                              │
│  ├─ Code Generation                                    │
│  ├─ Test Execution                                     │
│  ├─ Debugging                                          │
│  └─ Output Curation                                    │
└─────────────────────────────────────────────────────────┘
```

---

## 7. Cross-Reference Index

| Concept | File(s) |
|---|---|
| Pipeline orchestrator | `internal/cognitive/pipeline/pipeline.go` |
| Pipeline context | `internal/cognitive/pipeline/pipeline.go` |
| Stage interface | `internal/cognitive/pipeline/stage.go` |
| Stage 0 (Boot) | `internal/cognitive/pipeline/stage0.go` |
| Stage 1 (Ingest) | `internal/cognitive/pipeline/stage1.go` |
| Stage 2 (Believe) | `internal/cognitive/pipeline/stage2.go` |
| Stage 3 (Plan) | `internal/cognitive/pipeline/stage3.go` |
| Stage 4 (Execute) | `internal/cognitive/pipeline/stage4.go` |
| Stage 5 (Evidence) | `internal/cognitive/pipeline/stage5.go` |
| Stage 6 (Consolidate) | `internal/cognitive/pipeline/stage6.go` |
| Verification Policy | `internal/cognitive/pipeline/verification_policy.go` |
| ReQiCode HTTP Client | `internal/cognitive/integration/reqicode_client.go` |
| ReQiCodeTool | `internal/cognitive/tool/reqicode.go` |
| Tool Registry | `internal/cognitive/tool/registry.go` |
| Built-in Tools | `internal/cognitive/tool/builtin_tools.go` |
| Belief Network | `internal/cognitive/mind/beliefs.go` |
| Memory Operations | `internal/cognitive/mind/snapshot.go` |
| MongoDB Store | `internal/storage/mongo.go` |
| Store Interfaces | `internal/storage/interfaces.go` |
| Mock Store | `tests/mock_store.go` |
| LLM Registry | `internal/llm/registry.go` |
| Ollama Provider | `internal/llm/ollama.go` |
| OpenAI Provider | `internal/llm/openai_compatible.go` |
| llama.cpp Provider | `internal/llm/llamacpp.go` |
| EventBus | `internal/events/events.go` |
| Explainability | `internal/explainability/explainability.go` |
| Output Validation | `internal/validation/validator.go` |
| Config | `internal/config/config.go` |
| API Server | `internal/api/handlers.go` |
| Entry Point | `cmd/requiem/main.go` |
| SemiState | `ReqiCode/core/state/semi_state.go` |
| State Types | `ReqiCode/core/state/types.go` |
| Revision Chain | `ReqiCode/core/state/chain.go` |
| Potential | `ReqiCode/core/converge/potential.go` |
| Convergence | `ReqiCode/core/converge/convergence.go` |
| Orchestrator | `ReqiCode/core/system/orchestrator.go` |
| ReQiCode API | `ReqiCode/api/engineering.go`, `ReqiCode/api/server.go` |
| Claim Model | `pkg/models/evidence.go` |
| Evidence Model | `pkg/models/evidence.go` |
| Belief Model | `pkg/models/mind.go` |
| Memory Model | `pkg/models/memory.go` |
| Tool Model | `pkg/models/tool.go` |
| Pipeline Models | `pkg/models/pipeline.go` |
| Stage Models | `pkg/models/stages.go` |
| Base Models | `pkg/models/base.go` |
