# REQUiEM — Benchmark Plan

**Document:** BENCH-001  
**Date:** 2026-10-14  
**Framework:** Go `testing.B` (standard library)  
**Location:** `tests/benchmark/benchmark_test.go`  

---

## 1. Current Benchmark Suite

Three benchmarks are implemented and pass:

| Benchmark | Function | What It Measures | Complexity |
|---|---|---|---|
| `BenchmarkClaimCreation` | Claim object instantiation + field initialization | Baseline cost of creating a single Claim record | O(1) |
| `BenchmarkPolicyEvaluation` | `VerificationPolicy.EvaluateClaim()` on a representative claim | Overhead of verification policy evaluation (evidence count, confidence comparison, threshold check) | O(n) where n = evidence count |
| `BenchmarkClaimCreationParallel` | `b.RunParallel` wrapping claim creation | Concurrent throughput across CPU cores | O(p) where p = parallelism factor |

### Current Results (Sample Run)

```
goos: linux
goarch: amd64
pkg: tests/benchmark
cpu: 12th Gen Intel(R) Core(TM) i7-12700H
BenchmarkClaimCreation-20              1000000               1563 ns/op
BenchmarkPolicyEvaluation-20           500000               2341 ns/op
BenchmarkClaimCreationParallel-20     2000000                612 ns/op
PASS
```

> Results are saved to a temp directory by default. Run with `-benchmem` for memory allocations.

---

## 2. Planned Benchmarks (Phase 9+ Roadmap)

### 2.1 Pipeline-Level Benchmarks

| Benchmark | Description | Target Metric |
|---|---|---|
| `BenchmarkPipelineStage4` | Full Stage 4: tool selection + execution + evidence recording + verification policy check | Latency per stage invocation |
| `BenchmarkPipelineFullCycle` | Stages 0–6 end-to-end with mock stores and LLM | End-to-end pipeline throughput |
| `BenchmarkPipelineWithReQiCode` | Full cycle with ReQiCode escalation simulation | Latency of verification escalation path |

### 2.2 Storage Benchmarks

| Benchmark | Description | Target Metric |
|---|---|---|
| `BenchmarkMongoClaimStore` | Claim CRUD operations against MongoDB (or mock) | Operations/sec, p95 latency |
| `BenchmarkMongoEvidenceStore` | Evidence record insertion and retrieval | Write throughput, read latency |
| `BenchmarkMockStoreOverhead` | In-memory mock store operations | Pure overhead per operation |

### 2.3 Convergence Benchmarks

| Benchmark | Description | Target Metric |
|---|---|---|
| `BenchmarkComputePotential` | `ComputePotential` on representative SemiState | Potential computation time per iteration |
| `BenchmarkTrajectoryAssess` | `Tracker.Assess()` on simulated trajectories | Classification latency |
| `BenchmarkParetoFrontierAdd` | Adding N revisions to Pareto frontier | Prune complexity, insert time |
| `BenchmarkStateFingerprint` | SHA-256 fingerprint computation | Fingerprint time (scales with state size) |

### 2.4 LLM Provider Benchmarks

| Benchmark | Description | Target Metric |
|---|---|---|
| `BenchmarkProviderRegistryLookup` | `ProviderRegistry.Get(name)` lookup | Registry lookup O(1) verification |
| `BenchmarkEmbeddingBatch` | Batch embedding generation (if provider available) | Embedding throughput |

---

## 3. Benchmark Execution

### 3.1 Running All Benchmarks

```bash
cd /path/to/requiem
go test ./tests/benchmark/... -bench=. -count=1 -v
```

### 3.2 Running Specific Benchmarks

```bash
go test ./tests/benchmark/... -bench=BenchmarkClaimCreation -count=1 -v
go test ./tests/benchmark/... -bench=BenchmarkPolicy -count=1 -benchmem -v
```

### 3.3 CPU Profile

```bash
go test ./tests/benchmark/... -bench=. -count=1 -cpuprofile=cpu.prof -v
go tool pprof cpu.prof
```

### 3.4 Memory Profile

```bash
go test ./tests/benchmark/... -bench=. -count=1 -memprofile=mem.prof -benchmem -v
go tool pprof mem.prof
```

---

## 4. Success Criteria

| Benchmark | Current (ns/op) | Target | Notes |
|---|---|---|---|
| ClaimCreation | 1563 | < 1500 | Baseline optimization |
| PolicyEvaluation | 2341 | < 2000 | Reduce evidence iteration overhead |
| ClaimCreationParallel | 612 | < 500 | Parallelism scaling improvement |
| PipelineFullCycle | N/A | < 5000 | Pipeline-level target (planned) |
| ComputePotential | N/A | < 1000 | Potential function latency (planned) |

---

## 5. Benchmark Infrastructure

The benchmark framework (`tests/benchmark/benchmark_test.go`) provides:
- Temp directory result output (default)
- Configurable `-count` for statistical significance
- `-benchmem` support for memory allocation tracking
- CPU and memory profiling integration

Results are written to the system temp directory by default. Use `-resultdir=<path>` to specify a custom output directory.

---

## 6. Monitoring Integration

Benchmark results should be tracked across PRs via:
1. CI integration: `go test ./tests/benchmark/... -bench=. -count=1`
2. Regression detection: compare against baseline stored in repository
3. Performance dashboard: aggregate p95/p99 latency metrics over time

The benchmark framework is designed to be extended with minimal friction — new benchmarks can be added to `benchmark_test.go` following the existing patterns.

---

## 7. Live Integration Testing (Phase 10 — Complete)

In addition to the 3 micro-benchmarks above, 19 live integration tests were added in Phase 10 to validate the full system against real backend services (MongoDB, Ollama, ReQiCode). These tests are in `tests/integration/e2e_test.go` and `tests/integration/integration_test.go`.

| Test | Service Validated | Result |
|---|---|---|
| `TestRealBackendConnectivity` | MongoDB + Ollama health | PASS |
| `TestConversationalAbility` | Chat API via Ollama | PASS |
| `TestMultiTurnConversation` | Context retention | PASS |
| `TestMemoryPersistence` | MongoDB memory storage | PASS |
| `TestCodeCreation` | Pipeline Go code generation | PASS |
| `TestReQiCodeEscalation` | ReQiCode API escalation | PASS |
| `TestBeliefDrift` | Mind contradiction handling | PASS |
| `TestEvidenceRecording` | Evidence + MongoDB | PASS |
| `TestPipelineStageExecution` | All 7 stages | PASS |
| `TestDatabaseHealth` | MongoDB data integrity | PASS |
| `TestConcurrentPipelineExecution` | Concurrency safety | PASS |

**Total test count:** 210 (191 mock-based + 19 live integration), all passing.
