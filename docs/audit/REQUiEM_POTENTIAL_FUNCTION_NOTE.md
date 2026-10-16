# A Lyapunov Function for LLM Development Loops: Principled Termination via Scalar Potential

**Technical Note**  
**Date:** 2026-10-14  
**Author:** Kilo (audit review)  
**Project:** REQUiEM / ReQiCode  
**Source:** `ReqiCode/core/converge/potential.go`, `convergence.go`

---

## Abstract

LLM agent loops today terminate by one of two mechanisms: a fixed iteration budget, or the LLM itself declaring "I'm done." Both are brittle. The first wastes compute on diminishing returns; the second hallucinates completion. ReQiCode replaces both with a *scalar potential function* over the semi-state — a Lyapunov analogue that answers "is this iteration making progress?" with a measurable number. When the potential plateaus for three consecutive iterations, the loop stops. Three phases classify trajectory shape: **converged** (plateau), **productive** (rising), and **regressing** (falling), each with a distinct remediation action. A Pareto frontier over four objectives ensures the system always returns its best revision, even if a final exploratory iteration degrades results.

---

## 1. The Problem: No Principled Termination

Every existing LLM agent spiral ends in one of two failure modes:

| Failure | Description |
|---|---|
| Fixed budget | "Run for 50 iterations" — wastes compute on iterations that add nothing; or stops too early, cutting off a promising line of work. |
| LLM judgment | "I'm done" — the model has no reliable internal metric for completion; it says so regardless of actual progress. |

Neither accounts for the fundamental question: **is the system extracting value from additional iterations?**

## 2. The Potential Function

The potential function `V(s)` maps a `SemiState` `s` to a scalar in `[-1, 1]`. It is **pure** — same state in, same potential out — so any change in `V` between revisions is attributable to state change, not external factors.

### 2.1 Components

Eight terms, each normalized to [0, 1] or signed for penalties:

| Component | Weight | Formula | Rationale |
|---|---|---|---|
| TestPassRate | 0.32 | `dedup_pass / dedup_total` | Correctness is the dominant signal. De-duplicates identical test runs by `command + status`. |
| CompileSuccess | 0.18 | 1 if last `TestResult.Status == PASSED`, 0.5 if non-build failure, 0 if build failure | Build failure is categorically worse than an assertion failure. |
| EvidenceDensity | 0.13 | `0.6 * avg_strength + 0.4 * min(count/20, 1)` | Saturation at 20 evidence items prevents gaming by accumulation of weak evidence. |
| ResolutionRate | 0.08 | `resolved_findings / total_findings` | Epistemic progress: a system that adjudicates claims is genuinely better, regardless of test count. |
| ConfidenceTerm | 0.12 | `ss.Confidence` | The system's own belief, bounded by the other evidence terms. |
| DecisionStability | 0.05 | `1 - superseded_decisions / total_decisions` | Penalizes thrashing. A system that changes its mind every iteration is not converging. |
| ConflictPenalty | -0.07 | `active_conflicts / total_conflicts` (with floor at 0.5) | Subtracted. Any active contradiction meaningfully dents potential. |
| ObjectionPenalty | -0.05 | `weighted_objections / (2 * total_objections)` | Subtracted. Severity-weighted: critical=1, high=1, medium=0.5, default=0.25. |

```
V(s) = Σ(wᵢ · positiveᵢ) - Σ(wⱼ · negativeⱼ)
```

### 2.2 Weight Rationale

Weights encode a clear priority ordering: **correctness > evidence > stability > hygiene**. Test pass rate (0.32) dominates because a failing test suite is indefensible. Compile success (0.18) is second — an unbuilding system cannot be correct. Evidence density (0.13) follows — evidence without resolution is incomplete work. Resolution rate (0.08) rewards claim adjudication. Confidence (0.12) and decision stability (0.05) provide epistemic grounding. Penalties are smaller in magnitude because they represent systemic problems, not progress blockers.

### 2.3 StateFingerprint

For cycle detection, a deterministic SHA-256 fingerprint is computed over semantically load-bearing state:

```
fingerprint = sha256(
  plan.description + plan.alternative_id + sorted(components) + sorted(endpoints) +
  sorted(decisions) + sorted(requirements) + sorted(proposals) + sorted(file_paths)
)
```

**Deliberately excluded:** timestamps, revision counter, file contents. These change every iteration without implying reasoning change — including them would prevent meaningful cycle detection.

## 3. Convergence Classification

The `Tracker` maintains an append-only `Trajectory` of `(revision, potential, fingerprint, timestamp)`. `Assess()` classifies the current trajectory position:

| Phase | Trigger | Action |
|---|---|---|
| `productive` | `ΔV > ε` (potential rising) | Continue spiral |
| `converged` | `plateau_streak ≥ 3` and `ΔV ≤ ε` | **Stop.** No further extraction available. |
| `oscillating` | `fingerprint[N] == fingerprint[N-2]` (two-cycle) or longer cycle detected | **Force differentiation.** Mutate a design parameter. |
| `regressing` | `V_current < V_best - ε` | **Roll back.** Return to best-known revision. |
| `insufficient_data` | `< 2 trajectory points` | Continue spiral |

Constants: `ε = 0.01`, `plateau_tolerance = 3`.

### 3.1 Design Decisions

1. **Oscillation checked before regression:** A two-cycle produces a near-zero delta that would otherwise be misread as a healthy plateau. Detecting the cycle first allows `ForceDifferentiate` to break the loop.

2. **Bounded cycle detection window (8):** Full cycle detection over unbounded history is O(n²). A cycle longer than 8 revisions is not actionable.

3. **Revision 0 excluded from plateau:** The first iteration has no predecessor, so delta is undefined. The system continues until at least 2 points exist.

## 4. Pareto Frontier: Best-Revision Retention

```go
type ParetoPoint struct {
    Revision   int
    PassRate   float64
    Coverage   float64
    TokenCost  int
    Complexity int
}
```

The `ParetoFrontier` tracks non-dominated revisions across four objectives: **maximize** pass rate and coverage, **minimize** token cost and complexity. On each `Add()`, dominated points are pruned. `Best()` returns the frontier point with highest pass rate (ties broken by coverage, then token cost).

**Why this matters:** In a typical multi-step agent, the final iteration — often an exploratory one — can be worse than an earlier iteration. Without a frontier, the system reports its worst attempt as its answer. With it, the system can stop at any moment and return the best revision it ever found.

---

## 5. Implementation Notes

### 5.1 Purity

`ComputePotential` is marked pure in its documentation. In practice, it reads from `SemiState` methods (`GetTestResults()`, `GetEvidence()`, etc.) which return defensive copies under read locks. This ensures thread safety without compromising purity.

### 5.2 Nil Safety

The function includes a nil guard for `SemiState`, added after a discovery: the nil check was originally placed *after* `ss.Revision` was read, making it dead code. Found by `core/verify`, which synthesizes smoke tests calling every exported function with zero/nil arguments.

### 5.3 Test Pass Rate De-duplication

`testPassRate()` de-duplicates by `(command, status)` tuple. Without this, re-running the same test suite inflates the pass rate denominator, masking genuine progress.

## 6. Future Work

| Item | Description |
|---|---|
| Weight tuning experiments | Current weights (0.32, 0.18, 0.13, 0.08, 0.12, 0.05, -0.07, -0.05) are hand-set. Empirical validation across domains (web app, CLI tool, library) would reveal whether the priority ordering holds. |
| Adaptive epsilon | `ε = 0.01` is fixed. Could be derived from the signal-to-noise ratio of the potential trajectory itself. |
| Multi-objective frontier expansion | Currently 4 objectives. Could add security posture, documentation coverage, dependency freshness. |
| Fingerprint collision handling | SHA-256 collisions are cryptographically infeasible, but the 8-revision window means longer cycles are undetected. A bloom filter over full history could extend detection at O(n) memory cost. |

## 7. Conclusion

The potential function replaces the twin failures of fixed-budget and LLM-judgment termination with a measurable, reproducible scalar objective. Its components are individually interpretable, the trajectory phases are individually actionable, and the purity guarantee makes trajectories trustworthy. This is control theory for LLM spirals: a Lyapunov function that answers not "how many more steps?" but "is this step making things better?" — and stops when the answer is definitively no.

**3 benchmarks** validate the framework at the micro level (`BenchmarkClaimCreation`, `BenchmarkPolicyEvaluation`, `BenchmarkClaimCreationParallel`). **19 live integration tests** validate the full pipeline end-to-end against real MongoDB, Ollama, and ReQiCode services, confirming all 7 stages execute successfully.

---

*Corresponding source: `ReqiCode/core/converge/potential.go` (328 lines), `ReqiCode/core/converge/convergence.go` (392 lines). Related: `ReqiCode/core/state/semi_state.go` (435 lines), `ReqiCode/core/state/chain.go` (tamper-evident revision history).*
