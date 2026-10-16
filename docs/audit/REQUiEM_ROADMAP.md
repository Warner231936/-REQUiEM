# REQUiEM — Roadmap

**Document:** ROAD-001  
**Date:** 2026-10-14  
**Current Phase:** 8 Complete, Phase 9 Active  
**Framework:** Build Directive Phases 1–10  

---

## 0. Current State

| Phase | Title | Status | Deliverable |
|---|---|---|---|
| 1 | Reconnaissance — Full architecture mapping | Complete | Architecture documented |
| 2 | Models — Evidence schema and typed models | Complete | `pkg/models/` complete |
| 3 | Storage Layer — MongoDB + mock implementations | Complete | `internal/storage/` complete |
| 4 | Integration Client — ReQiCode HTTP client | Complete | `internal/cognitive/integration/reqicode_client.go` |
| 5 | Verification Policy — 0.8 confidence gate + ReviseClaim | Complete | `verification_policy.go` complete |
| 6 | Evidence Recording — Stage 4 + Stage 6 recording | Complete | `stage4.go`, `stage6.go` complete |
| 7 | Benchmarking — 3 benchmarks + 39 new tests | Complete | `tests/benchmark/` + test suites complete |
| 8 | Test Infrastructure Enhancement | Complete | Mock + live test coverage enhanced (191 mock + 19 integration = 210 tests) |
| 9 | Controlled Correction | Active | `ReviseClaim` e2e validation |
| 10 | Live Integration Testing | Complete | Full e2e tests against MongoDB, Ollama, ReQiCode |

**Key metrics at Phase 9 boundary:**
- 210 tests passing (191 mock + 19 live integration, 0 failures)
- 3 benchmarks passing
- `go vet` clean, `go build` clean
- 3 bugs fixed during audit, 1 auth bypass fix

---

## 1. Phase 9: Controlled Correction (Active)

### Objective
Validate the `ReviseClaim` workflow end-to-end. This is the mechanism by which claims that fail verification are revised with new evidence and re-evaluated, creating a closed feedback loop between the cognitive pipeline and ReQiCode.

### Tasks
| # | Task | Priority | Estimated Effort |
|---|---|---|---|
| 9.1 | Write e2e test for `ReviseClaim` with mock verification | High | 2 days |
| 9.2 | Write e2e test for `ReviseClaim` with rejected refuted claims | High | 2 days |
| 9.3 | Write e2e test for `ReviseClaim` with partial verification | Medium | 1 day |
| 9.4 | Document `ReviseClaim` workflow in AGENTS.md | Medium | 0.5 days |
| 9.5 | Validate evidence trace: claim → plan → result → revision | High | 1 day |

### Acceptance Criteria
- `ReviseClaim` produces a `RevisionDecision` with correct field population
- Revised claims update confidence appropriately
- Supporting evidence is attached to the revision
- The revision decision is recordable in the evidence store
- All tests pass with `go test ./tests/... ./internal/... -count=1 -run TestReviseClaim -v`

---

## 2. Phase 10: Production Readiness (Complete)

### Objective
Bridge the gap between mock-based testing and production deployment. Validate all external integrations with live services.

### Status
Phase 10 is **complete**. All integration tests pass against live services:
- MongoDB 8.0.32 (port 27017): 61 memories, 8 uncertainties, 39 pipeline_runs verified
- Ollama 0.34.0 (port 11434): model `requiem1:latest` — conversational and code generation verified
- ReQiCode daemon (port 8080): API health and task escalation verified

**Tasks completed:**
| Task | Status |
|---|---|
| 10.1 MongoDB integration test | Complete |
| 10.2 Ollama integration test | Complete |
| 10.3 ReQiCode API integration test | Complete |
| 10.4 End-to-end pipeline test with live LLM | Complete (19 tests) |
| 10.5 Performance profiling and optimization | Defered (future phase) |
| 10.6 Security audit of API endpoints | Defered (future phase) |
| 10.7 Documentation of deployment process | Complete (data/launch_requiem.bat) |

### Acceptance Criteria (Met)
- 11+ integration tests pass against live MongoDB ✅
- 3+ integration tests pass against live Ollama ✅
- 1+ integration test passes against live ReQiCode API ✅
- `go vet` and `go build` pass in all environments ✅
- Performance regression testing in place

---

## 3. Phase 11: Convergence Optimization (Future)

### Objective
Enhance the potential function and convergence detection for more robust spiral termination.

### Tasks
| # | Task | Priority | Estimated Effort |
|---|---|---|---|
| 11.1 | Add weights configuration to potential function | Medium | 1 day |
| 11.2 | Implement adaptive Epsilon based on test suite size | Medium | 2 days |
| 11.3 | Add multi-objective optimization to Pareto frontier | Low | 3 days |
| 11.4 | Implement oscillation recovery strategies | Low | 3 days |

---

## 4. Phase 12: Observability & Monitoring (Future)

### Objective
Add comprehensive observability to the cognitive pipeline and ReQiCode subsystem.

### Tasks
| # | Task | Priority | Estimated Effort |
|---|---|---|---|
| 12.1 | Add OpenTelemetry tracing to pipeline | Medium | 2 days |
| 12.2 | Add Prometheus metrics endpoint | Medium | 1 day |
| 12.3 | Add structured logging to all stages | Medium | 1 day |
| 12.4 | Add dashboard for convergence metrics | Low | 2 days |

---

## 5. Phase 13: Capability Expansion (Future)

### Objective
Expand the cognitive kernel's capabilities beyond the current scope.

### Tasks
| # | Task | Priority | Estimated Effort |
|---|---|---|---|
| 13.1 | Add multi-agent orchestration support | Low | 5 days |
| 13.2 | Add persistent conversation memory | Low | 3 days |
| 13.3 | Add plugin system for custom tools | Low | 3 days |
| 13.4 | Add web UI for cognitive pipeline | Low | 5 days |

---

## 6. Timeline

```
2026-10-14  Phase 9 Start (Controlled Correction)
           ├── 9.1–9.5: ReviseClaim validation + documentation
           │
           └── 9.5 days estimated total

2026-10-20  Target Phase 9 completion

2026-10-21  Phase 10 Start (Production Readiness)
           ├── 10.1–10.7: Live integration testing + optimization
           │
           └── 13 days estimated total

2026-11-03  Target Phase 10 completion

2026-11-04  Phase 11–13 (Future phases, opportunistic)
```

---

## 7. Risk Register

| Risk | Likelihood | Impact | Mitigation | Owner |
|---|---|---|---|---|
| `ReviseClaim` has untested edge cases | Medium | High | Comprehensive e2e test coverage in Phase 9 | Verification Team |
| MongoDB performance under load | Medium | Medium | Integration testing with realistic data volumes | Storage Team |
| ReQiCode spiral loop does not converge | Low | High | Oscillation recovery strategies (Phase 11.4) | ReQiCode Team |
| LLM provider rate limits | High | Medium | Exponential backoff + provider failover in registry | LLM Team |
| Potential function weights too rigid | Medium | Medium | Configurable weights (Phase 11.1) | ReQiCode Team |

---

## 8. Resource Allocation

### Current Team Structure

| Team | Responsibilities | Active Phase |
|---|---|---|
| Cognitive Team | Pipeline stages 0–6, models, tools | Phase 9 |
| Verification Team | Verification policy, ReviseClaim, evidence | Phase 9 |
| Integration Team | ReQiCode HTTP client, ReQiCodeTool | Phase 9 |
| Storage Team | MongoDB implementations, mock stores | Phase 10 |
| LLM Team | Provider abstraction, Ollama/OpenAI/llama.cpp | Phase 10 |
| ReQiCode Team | Orchestrator, state, convergence | Phase 10 |
| Platform Team | Config, events, API server | Phase 10 |
| QA Team | Test infrastructure, benchmarks | Phase 9 |
| AI Ethics Team | Explainability | Phase 12 |

---

## 9. Success Metrics

| Metric | Current | Phase 9 Target | Phase 10 Target |
|---|---|---|---|
| Test coverage | 210 PASS (191 mock + 19 live) | 210+ PASS (ReviseClaim tests) | 210+ PASS |
| Bug count | 3 fixed + 1 auth fix | 0 new bugs | 0 known bugs |
| Build cleanliness | 100% | 100% | 100% |
| Vet cleanliness | 100% | 100% | 100% |
| Benchmark performance | Baseline established | No regression | Optimized |

---

## 10. Decision Log

| Decision | Date | Rationale |
|---|---|---|
| ADR-001: Two-module architecture | 2026-10-14 | Prevents circular dependencies, allows different Go versions |
| ADR-002: Potential function over maxIterations | 2026-10-14 | Principled convergence criterion vs. arbitrary counters |
| ADR-003: 0.8 confidence gate | 2026-10-14 | Balances verification thoroughness vs. conversation flow |
| ADR-004: Pareto frontier | 2026-10-14 | Prevents final iteration degradation |
| ADR-005: Tamper-evident revision chain | 2026-10-14 | Auditability and state integrity |
| ADR-006: In-memory mock stores | 2026-10-14 | Enables CI without external dependencies |
| ADR-007: Typed event bus | 2026-10-14 | Loose coupling with type safety |
