# Why Your AI Agent Doesn't Know When to Stop

Most "autonomous" coding agents never know when to quit. They run for a fixed number of iterations, or let the LLM declare "I'm done."

This isn't a tooling problem. It's a **systems control** problem.

I built REQUiEM — a cognitive architecture that applies control theory to LLM development spirals. Here's the core idea:

## The Lyapunov Potential Function

Instead of counting iterations, REQUiEM computes a **scalar potential** over the system's internal state. If it stops improving for 3 consecutive iterations, the loop stops.

8 normalized signals:

| Signal | Weight | Measures |
|---|---|---|
| Test pass rate | 0.32 | Are tests passing? (de-duplicated) |
| Compile success | 0.18 | Does it build? |
| Evidence density | 0.13 | Evidence strength (saturates at 20) |
| Resolution rate | 0.08 | Claims settled vs. accumulated |
| Confidence | 0.12 | System's own belief |
| Decision stability | 0.05 | Thrashing penalty |
| Conflict penalty | -0.07 | Active contradictions |
| Objection penalty | -0.05 | Severity-weighted objections |

**Key insight:** The function is *pure*. Same state in → same potential out. Trajectory changes are attributable to state, not noise.

## Convergence Classification

The classifier looks at potential shape:

- **Converged** (3-iter plateau): Stop
- **Productive** (rising): Continue
- **Oscillating** (cycle detected): Force differentiation
- **Regressing** (below best): Roll back

This replaces `maxIterations=50` with something that adapts to the actual task.

## Pareto Frontier: Never Return Your Worst Output

Traditional agents' final iteration can be worse than earlier ones. REQUiEM tracks a **Pareto frontier** across (pass rate, coverage, token cost, complexity), so it can always return its best revision — not its last.

## Architecture

Two Go modules with HTTP boundary:
- `github.com/requiem/cognitive` — 7-stage pipeline (ingestion → belief → planning → execution → evidence → consolidation)
- `github.com/kilo/spiral-codemaker` — Engineering spiral (requirements → architecture → code → test → debug → iterate)

The cognitive module reasons about confidence. The engineering module verifies it. Low-confidence claims auto-escalate.

## Results

210 tests passing (191 mock + 19 live integration against real MongoDB, Ollama, ReQiCode). 39 pipeline runs in production MongoDB, all 7 stages succeeding.

**Docs:** [Architecture](docs/audit/REQUiEM_ARCHITECTURE.md) | [Evidence register](docs/audit/REQUiEM_EVIDENCE_REGISTER.md) | [Technical note](docs/audit/REQUiEM_POTENTIAL_FUNCTION_NOTE.md)

The world doesn't need another "agent that calls tools." It needs systems that can reason about their own correctness and stop when they're done.

#AI #SoftwareEngineering #LLM #AgentLoop #ControlTheory #SystemsDesign #LyapunovFunction #ParetoOptimality
