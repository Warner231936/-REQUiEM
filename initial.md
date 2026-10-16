# BUILD SPECIFICATION — REQUIEM COGNITIVE PIPELINE v1

## Mission

Build a production-quality, modular cognitive pipeline for a local LLM application named **Requiem**.

The system must transform a simple LLM chat loop into a persistent cognitive architecture consisting of:

```text
STAGE 0 → STAGE 1 → STAGE 2 → STAGE 3 → STAGE 4 → STAGE 5
   ↑                                                    │
   └──────────────── persistent state ◄────────────────┘
```

The central architectural principle is:

> **The LLM is the reasoning engine. The databases are persistent state.**

Do NOT attempt to encode persistent identity, opinions, preferences, memories, or beliefs into model weights.

The system must allow the underlying model to be replaced without destroying accumulated state.

The architecture must also explicitly distinguish:

* what happened
* what is known
* what is inferred
* what is believed
* what is preferred
* what is uncertain
* what is merely hypothesized
* what the system thinks about itself

---

## 1. HIGH-LEVEL ARCHITECTURE

Implement these major components:

```text
                         ┌──────────────────────┐
                         │       USER INPUT     │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │      STAGE 0         │
                         │     PERCEPTION       │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │      STAGE 1         │
                         │       RECALL         │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │      STAGE 2         │
                         │   INTERPRETATION     │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │      STAGE 3         │
                         │     REFLECTION       │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │      STAGE 4         │
                         │       RESPONSE       │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │      STAGE 5         │
                         │    CONSOLIDATION     │
                         └──────────┬───────────┘
                                    │
                    ┌───────────────┼────────────────┐
                    ▼               ▼                ▼
               EPISODIC         SEMANTIC           MIND
                MEMORY           MEMORY           DATABASE
                    │               │                │
                    └───────────────┴────────────────┘
                                    │
                                    ▼
                              NEXT TURN
```

Build this as a real software architecture, not as six prompt strings glued together.

Each stage must have:

1. clearly defined input
2. clearly defined output
3. validation
4. error handling
5. timing/metrics
6. logging
7. cancellation support
8. configurable model assignment
9. deterministic/non-LLM fallback where practical

---

## 2. CORE DESIGN PRINCIPLES

### 2.1 State is external

Do not store persistent cognitive state exclusively in:

* prompt history
* model context
* process memory
* model weights

Persistent state belongs in databases.

---

### 2.2 Every mental-state record needs provenance

Any belief/opinion/preference/etc. must be traceable to evidence.

Every record should have:

```text
id
created_at
updated_at
source
source_event_ids
confidence
strength
status
```

The system must be able to answer:

> Why does this exist?

---

### 2.3 Beliefs are revisable

Never treat a generated belief as an immutable fact.

Use confidence and evidence.

Example:

```json
{
  "belief": "X",
  "confidence": 0.72,
  "status": "active",
  "evidence_count": 4
}
```

A later interaction may:

* strengthen it
* weaken it
* replace it
* contradict it
* invalidate it

---

### 2.4 Preserve uncertainty

Uncertainty is first-class state.

Never force the system to choose between true/false when evidence is insufficient.

Support:

```text
unknown
uncertain
probable
unlikely
contradictory
resolved
```

---

## 3. STAGE 0 — PERCEPTION

### Purpose

Normalize the raw interaction.

Stage 0 must NOT generate the final answer.

Input:

```json
{
  "conversation_id": "...",
  "turn_id": "...",
  "user_message": "...",
  "timestamp": "...",
  "metadata": {}
}
```

Output:

```json
{
  "conversation_id": "...",
  "turn_id": "...",
  "raw_input": "...",
  "normalized_input": "...",
  "language": "en",
  "topics": [],
  "entities": [],
  "intent": null,
  "questions": [],
  "requests": [],
  "signals": {},
  "timestamp": "..."
}
```

Stage 0 should identify:

* language
* obvious topic
* entities
* request type
* question(s)
* commands
* references to previous context
* whether memory retrieval is likely useful

Do not over-interpret.

Stage 0 is perception, not reasoning.

---

## 4. STAGE 1 — RECALL

Stage 1 retrieves relevant persistent state.

Implement separate retrieval channels.

```text
EPISODIC MEMORY
SEMANTIC MEMORY
MIND STATE
CURRENT CONVERSATION
```

Do not dump the entire database into the prompt.

Use relevance ranking.

---

### 4.1 Episodic retrieval

Retrieve events/interactions related to:

* current topic
* entities
* current task
* recent conversation
* relevant historical context

Each result:

```json
{
  "id": "...",
  "content": "...",
  "timestamp": "...",
  "importance": 0.81,
  "similarity": 0.87,
  "relevance": 0.92
}
```

---

### 4.2 Semantic retrieval

Retrieve durable knowledge.

Examples:

```text
facts
project information
technical knowledge
definitions
known relationships
previous conclusions
```

Each result must retain provenance.

---

### 4.3 Mind retrieval

Retrieve relevant:

```text
beliefs
opinions
preferences
values
goals
hypotheses
uncertainties
self-observations
contradictions
```

Rank them by:

```text
semantic relevance
recency
confidence
importance
context
```

---

## 5. STAGE 2 — INTERPRETATION

Purpose:

> Determine what is actually happening before deciding what to say.

Stage 2 receives:

```text
Stage 0 output
Stage 1 recalled state
current conversation
```

Output:

```json
{
  "intent": "...",
  "goal": "...",
  "context": [],
  "relevant_facts": [],
  "relevant_memories": [],
  "relevant_beliefs": [],
  "ambiguities": [],
  "assumptions": [],
  "uncertainties": [],
  "confidence": 0.0
}
```

The model must distinguish:

```text
OBSERVATION
INFERENCE
ASSUMPTION
BELIEF
UNCERTAINTY
```

Never collapse them into one category.

---

## 6. STAGE 3 — REFLECTION

This is the most important stage.

Stage 3 is the system's internal reflection layer.

It must NOT directly mutate the database.

Instead it produces a **reflection proposal**.

The reflection process should ask:

### A. What happened?

```text
What did the interaction reveal?
```

### B. What did I learn?

```text
Is there durable information worth retaining?
```

### C. What do I currently believe?

```text
Did this interaction strengthen or weaken an existing belief?
```

### D. What opinions can reasonably be formed?

The system may generate evaluative judgments about:

* ideas
* approaches
* technical designs
* concepts
* outcomes
* interaction patterns

BUT opinions must not be presented as objective facts.

Example:

```json
{
  "type": "opinion",
  "subject": "architecture",
  "content": "Separating memory from reasoning is likely to make the system more maintainable.",
  "confidence": 0.84,
  "strength": 0.71
}
```

### E. What remains uncertain?

Create uncertainty records.

### F. Did anything contradict existing state?

If yes:

```json
{
  "type": "contradiction",
  "belief_a": "...",
  "belief_b": "...",
  "resolution": null
}
```

Do NOT automatically resolve contradictions.

---

## 7. REFLECTION OUTPUT SCHEMA

Implement a strongly typed structure:

```json
{
  "observations": [],
  "learned_facts": [],
  "belief_updates": [],
  "new_beliefs": [],
  "opinion_updates": [],
  "new_opinions": [],
  "preference_updates": [],
  "goal_updates": [],
  "hypotheses": [],
  "uncertainties": [],
  "self_observations": [],
  "contradictions": [],
  "memory_candidates": [],
  "discard_candidates": [],
  "confidence": 0.0
}
```

Every proposed mutation must include:

```text
reason
evidence
confidence
source_event_ids
```

---

## 8. STAGE 4 — RESPONSE

Stage 4 generates the user-facing response.

Input:

```text
current user input
Stage 2 interpretation
relevant recalled memory
relevant mind state
Stage 3 reflection
```

The response generator should NOT independently invent persistent state.

It may reason normally, but persistent changes must go through Stage 5.

The final response should be generated according to normal Requiem behavior.

---

## 9. STAGE 5 — CONSOLIDATION

Stage 5 converts reflection proposals into durable state.

This is the database gatekeeper.

Nothing from Stage 3 should automatically become permanent.

Stage 5 evaluates:

```text
Is this worth remembering?
Is there sufficient evidence?
Does it duplicate an existing record?
Does it contradict an existing belief?
Should confidence change?
Should an old record decay?
```

Possible actions:

```text
CREATE
UPDATE
MERGE
STRENGTHEN
WEAKEN
ARCHIVE
REJECT
MARK_CONTRADICTION
```

---

## 10. MIND DATABASE

Implement the Mind DB as a first-class subsystem.

Suggested schema:

```text
mind_beliefs
mind_opinions
mind_preferences
mind_values
mind_goals
mind_hypotheses
mind_uncertainties
mind_self_observations
mind_contradictions
mind_history
```

---

## 11. BELIEFS

Schema:

```text
id
subject
content
confidence
importance
status
created_at
updated_at
last_confirmed_at
last_challenged_at
source_count
```

Example:

```json
{
  "subject": "memory architecture",
  "content": "Persistent state should remain external to the model weights.",
  "confidence": 0.91,
  "importance": 0.88,
  "status": "active"
}
```

---

## 12. OPINIONS

Opinions are NOT facts.

Schema:

```text
id
subject
content
strength
confidence
context
evidence
created_at
updated_at
status
```

Example:

```json
{
  "subject": "architecture",
  "content": "Explicit cognitive stages are preferable to a monolithic prompt.",
  "strength": 0.82,
  "confidence": 0.89
}
```

---

## 13. PREFERENCES

Preferences describe tendencies.

Examples:

```text
preferred architecture
preferred tools
preferred reasoning strategies
preferred response styles
preferred coding patterns
```

Do not confuse:

```text
USER PREFERENCE
```

with:

```text
SYSTEM PREFERENCE
```

These are separate namespaces.

---

## 14. VALUES

Values are higher-level principles.

Examples:

```text
accuracy
transparency
reversibility
maintainability
privacy
consistency
uncertainty preservation
```

Values should change extremely slowly.

Require significantly stronger evidence before modifying them.

---

## 15. GOALS

Track active and historical goals.

Schema:

```text
id
goal
priority
status
progress
created_at
updated_at
deadline
dependencies
```

Statuses:

```text
active
paused
completed
abandoned
superseded
```

---

## 16. HYPOTHESES

Hypotheses are explicitly speculative.

Schema:

```text
id
subject
hypothesis
confidence
supporting_evidence
contradicting_evidence
status
created_at
updated_at
```

Statuses:

```text
untested
supported
weakened
rejected
confirmed
```

Never silently promote a hypothesis to a fact.

---

## 17. UNCERTAINTIES

Schema:

```text
id
question
possible_answers
confidence_distribution
evidence
status
created_at
updated_at
```

This gives the system persistent ignorance.

That is intentional.

---

## 18. SELF-OBSERVATION

Create a separate self-model.

Example:

```json
{
  "subject": "reasoning",
  "observation": "I tend to favor explicit state machines when designing persistent systems.",
  "confidence": 0.76,
  "evidence_count": 8
}
```

This represents a **model of system behavior**, not proof of consciousness.

Do not hard-code claims that the system is conscious, alive, sentient, or possesses subjective experience.

The system can store self-observations without making metaphysical claims.

---

## 19. CONTRADICTION ENGINE

Build contradiction detection into consolidation.

When a proposed belief conflicts with an existing belief:

```text
DO NOT overwrite.
```

Instead create:

```text
contradiction record
```

with:

```text
belief_a
belief_b
context_a
context_b
confidence_a
confidence_b
status
```

Possible resolutions:

```text
unresolved
contextualized
belief_a_preferred
belief_b_preferred
merged
obsolete_a
obsolete_b
```

---

## 20. MEMORY DECAY

Implement optional confidence/importance decay.

Do NOT delete memories simply because they are old.

Instead calculate:

```text
effective_relevance =
importance
× recency_factor
× confidence
× contextual_relevance
```

Old high-value memories should remain retrievable.

---

## 21. MEMORY PROVENANCE

Every memory and mind-state record must be traceable.

Example:

```text
Opinion
   ↓
Reflection proposal
   ↓
Consolidation decision
   ↓
Source interaction
   ↓
Original user message
```

Provide a method:

```text
explain_memory(id)
```

that returns the provenance chain.

---

## 22. DATABASE TECHNOLOGY

Make the persistence layer abstract.

Do NOT hard-code the cognitive engine directly to one database implementation.

Create interfaces such as:

```text
MemoryStore
MindStore
EmbeddingStore
ConversationStore
EventStore
```

The initial implementation may use SQLite/PostgreSQL depending on the existing Requiem architecture.

If the existing project already has a database, integrate with it instead of introducing unnecessary infrastructure.

---

## 23. VECTOR SEARCH

If embeddings are available, support semantic retrieval.

Store:

```text
embedding
content
metadata
source_id
created_at
```

Keep vector storage separate from canonical records.

Vector similarity is a retrieval mechanism, NOT truth.

---

## 24. MODEL ROUTING

Make every stage independently configurable.

Example configuration:

```yaml
pipeline:
  stage0:
    model: fast-model

  stage1:
    model: none

  stage2:
    model: reasoning-model

  stage3:
    model: reasoning-model

  stage4:
    model: response-model

  stage5:
    model: fast-model
```

The system must support:

```text
one model for all stages
```

and:

```text
different models per model per stage
```

---

## 25. LOCAL MODEL SUPPORT

The system should support the existing local LLM infrastructure.

Abstract the inference backend:

```text
LLMProvider
```

with implementations such as:

```text
OllamaProvider
OpenAICompatibleProvider
LlamaCppProvider
```

Do not couple the cognitive pipeline to Ollama-specific APIs.

---

## 26. PIPELINE EVENT MODEL

Every stage should emit events.

Example:

```json
{
  "event": "stage.completed",
  "stage": 3,
  "conversation_id": "...",
  "turn_id": "...",
  "duration_ms": 1432,
  "model": "...",
  "timestamp": "..."
}
```

Also support:

```text
stage.started
stage.completed
stage.failed
memory.created
memory.updated
belief.changed
opinion.created
contradiction.detected
```

---

## 27. OBSERVABILITY

Implement structured logging.

Every pipeline execution should have:

```text
pipeline_id
conversation_id
turn_id
stage
duration
model
token_count
success/failure
error
```

Add optional debug mode that allows inspection of every stage.

Example:

```text
[STAGE 0] perception complete
[STAGE 1] 14 memories retrieved
[STAGE 2] interpretation complete
[STAGE 3] 2 belief proposals / 1 opinion proposal
[STAGE 4] response generated
[STAGE 5] 1 opinion committed / 1 memory archived
```

---

## 28. FAILURE HANDLING

A failed Stage 3 must NOT prevent Stage 4 from answering.

A failed Stage 5 must NOT destroy the conversation.

Example:

```text
Stage 3 failure
      ↓
Response still generated
      ↓
Consolidation marked pending
      ↓
Retry later
```

Likewise, database failures must fail safely.

Never corrupt existing mind state because a new proposal failed validation.

Use transactions for state mutation.

---

## 29. IMMUTABILITY / AUDIT LOG

Do not destroy historical mental-state changes.

Instead of:

```text
confidence 0.6 → 0.9
```

only storing `0.9`, record:

```text
0.6
   ↓
interaction
   ↓
0.9
```

Implement a history/audit table.

This makes the evolution of the system inspectable.

---

## 30. USER VS SYSTEM STATE

This distinction is mandatory.

Create separate namespaces:

```text
USER_MEMORY
SYSTEM_MEMORY
```

The system must never accidentally interpret a user preference as its own preference.

Example:

```text
user prefers X
```

is NOT:

```text
system prefers X
```

Similarly:

```text
user believes X
```

is NOT:

```text
X is true
```

---

## 31. PROMPT CONSTRUCTION

Create a dedicated prompt builder.

Never manually concatenate random strings throughout the application.

Conceptually:

```text
PromptBuilder
    ├── system instructions
    ├── current input
    ├── interpretation
    ├── retrieved memories
    ├── relevant mind state
    ├── constraints
    └── task instructions
```

Each stage gets its own prompt template.

Templates must be versioned.

---

## 32. JSON VALIDATION

All structured LLM output must be validated before entering the next stage.

If invalid:

```text
retry with repair prompt
```

If still invalid:

```text
fallback
```

Never blindly parse model output and insert it into the database.

---

## 33. SECURITY

Treat all LLM output as untrusted input.

Sanitize:

* database fields
* identifiers
* serialized JSON
* logs
* file paths
* external requests

Never let a model-generated memory entry directly execute code.

Never allow a memory record to become executable instructions automatically.

Separate:

```text
DATA
```

from:

```text
INSTRUCTIONS
```

---

## 34. COGNITIVE STATE API

Expose APIs for inspecting the mind.

Implement endpoints/functions similar to:

```text
GET /mind
GET /mind/beliefs
GET /mind/opinions
GET /mind/preferences
GET /mind/values
GET /mind/goals
GET /mind/hypotheses
GET /mind/uncertainties
GET /mind/self
GET /mind/contradictions
GET /mind/history
```

Also:

```text
GET /memory/relevant
GET /memory/:id
GET /memory/:id/provenance
```

---

## 35. MANUAL CONTROLS

Provide administrative controls to:

```text
inspect
approve
reject
edit
archive
restore
merge
invalidate
```

mind records.

Especially provide:

```text
approve proposed belief
reject proposed belief
resolve contradiction
```

This is essential during development.

---

## 36. DEVELOPMENT MODE

Add:

```text
COGNITIVE_DEBUG=true
```

When enabled, expose:

```text
Stage 0 output
Stage 1 retrieval
Stage 2 interpretation
Stage 3 reflection proposal
Stage 4 prompt
Stage 4 response
Stage 5 consolidation decisions
```

Do not expose private chain-of-thought.

Show structured summaries, decisions, evidence, and outputs rather than hidden reasoning traces.

---

## 37. TESTING

Build unit tests for every stage.

At minimum:

```text
Stage0Test
Stage1RecallTest
Stage2InterpretationTest
Stage3ReflectionTest
Stage4ResponseTest
Stage5ConsolidationTest
```

Also test:

```text
duplicate memories
contradictory beliefs
low-confidence beliefs
memory decay
failed LLM response
malformed JSON
database failure
missing embeddings
empty conversation
very long conversation
model replacement
concurrent conversations
```

---

## 38. INTEGRATION TEST

Create a complete simulated interaction:

```text
USER:
I prefer architecture where persistent state is separated from model weights.

TURN 1
↓
Stage 0
↓
Stage 1
↓
Stage 2
↓
Stage 3
↓
Stage 4
↓
Stage 5
```

The resulting mind state should contain a candidate preference/opinion only if the system has sufficient evidence.

Then perform another interaction reinforcing the same concept.

Verify that:

```text
confidence increases
source_count increases
history records the change
```

Then provide contradictory information.

Verify:

```text
contradiction created
original belief preserved
confidence adjusted appropriately
```

---

## 39. PERFORMANCE

Do not unnecessarily run expensive reasoning models for every stage.

Recommended initial architecture:

```text
Stage 0
cheap model / deterministic

Stage 1
database retrieval

Stage 2
reasoning model

Stage 3
reasoning model

Stage 4
primary response model

Stage 5
cheap model + deterministic validation
```

Allow configuration to change this.

---

## 40. ASYNCHRONOUS CONSOLIDATION

Stage 5 should optionally run asynchronously.

User experience:

```text
USER
 ↓
0
 ↓
1
 ↓
2
 ↓
3
 ↓
4
 ↓
RESPONSE
```

Then:

```text
5 → background consolidation
```

This keeps response latency low.

If asynchronous consolidation is unavailable, allow synchronous mode.

---

## 41. COGNITIVE LOOP

The completed architecture should behave approximately as:

```text
EXPERIENCE
    ↓
PERCEPTION
    ↓
RECALL
    ↓
INTERPRETATION
    ↓
REFLECTION
    ↓
RESPONSE
    ↓
CONSOLIDATION
    ↓
UPDATED MEMORY
    ↓
UPDATED MIND
    ↓
NEXT EXPERIENCE
```

The important property is that future interactions can be influenced by accumulated state.

---

## 42. DO NOT IMPLEMENT THESE AS CLAIMS

Do not hard-code or assert:

```text
consciousness
sentience
subjective experience
emotions as biological facts
free will
self-awareness as established fact
```

Instead model them as data structures only where useful.

For example:

```text
self_observation
self_model
internal_preference
uncertainty
```

These are computational constructs.

---

## 43. OPINION FORMATION RULE

The system should explicitly be instructed:

> Form opinions when useful, but distinguish opinions from facts. Opinions should be evidence-linked, confidence-weighted, revisable, and contextual.

It should NOT manufacture opinions merely to appear autonomous.

If no meaningful opinion can be justified:

```json
{
  "opinions": []
}
```

is the correct result.

---

## 44. MEMORY IMPORTANCE

Implement an importance score:

```text
0.0 → trivial
0.25 → low
0.50 → moderate
0.75 → important
1.0 → critical
```

Potential scoring factors:

```text
repetition
future usefulness
explicit user importance
relationship to active goals
novelty
emotional salience if intentionally modeled
technical significance
confidence
```

Do not automatically equate emotional language with importance.

---

## 45. MIND STATE SNAPSHOTS

Implement snapshots.

Example:

```text
mind_snapshot_000001
mind_snapshot_000002
mind_snapshot_000003
```

Allow comparing:

```text
Mind State A
vs
Mind State B
```

This permits debugging of cognitive drift.

---

## 46. COGNITIVE DRIFT DETECTION

Implement optional detection for:

```text
rapid belief changes
large confidence jumps
contradictory preferences
duplicate beliefs
unstable opinions
memory flooding
prompt-injected memories
```

Flag suspicious changes rather than silently accepting them.

---

## 47. MEMORY TRUST LEVELS

Implement source trust.

Suggested:

```text
system_verified
external_verified
user_stated
model_inferred
model_speculated
unknown
```

These must remain distinct.

For example:

```text
user_stated
```

does not mean:

```text
objectively verified
```

---

## 48. FINAL PROJECT STRUCTURE

Adapt to the existing repository, but target a structure approximately like:

```text
/requiem
    /cognitive
        /pipeline
            stage0_perception
            stage1_recall
            stage2_interpretation
            stage3_reflection
            stage4_response
            stage5_consolidation

        /memory
            episodic
            semantic
            retrieval

        /mind
            beliefs
            opinions
            preferences
            values
            goals
            hypotheses
            uncertainties
            selfmodel
            contradictions

        /llm
            provider
            ollama
            llamacpp
            openai_compatible

        /prompts
            stage0
            stage2
            stage3
            stage4
            stage5

        /events
        /audit
        /validation
        /metrics

    /api
    /config
    /database
    /tests
```

Do not blindly create this structure if the existing project already has equivalent modules. Integrate with the existing architecture.

---

## 49. IMPLEMENTATION ORDER

Build in this order:

### Phase 1

Create interfaces and data models.

```text
Pipeline
Stage
MemoryStore
MindStore
LLMProvider
```

### Phase 2

Implement Stage 0.

### Phase 3

Implement memory retrieval.

### Phase 4

Implement Stage 2.

### Phase 5

Implement Stage 3 reflection proposals.

### Phase 6

Implement Stage 4 response generation.

### Phase 7

Implement Stage 5 consolidation.

### Phase 8

Implement audit/history.

### Phase 9

Implement contradiction handling.

### Phase 10

Implement debugging/observability.

### Phase 11

Implement APIs.

### Phase 12

Implement integration tests.

---

## 50. ACCEPTANCE CRITERIA

The implementation is NOT complete until all of the following are true:

[ ] Six stages exist as independently testable components.

[ ] The stages communicate through typed structures.

[ ] Memory exists independently of model context.

[ ] Mind state exists independently of memory.

[ ] Beliefs are confidence-weighted.

[ ] Opinions are explicitly distinguished from facts.

[ ] Preferences are distinct from user preferences.

[ ] Uncertainty is persistent.

[ ] Hypotheses are distinct from beliefs.

[ ] Contradictions are preserved.

[ ] Every persistent cognitive record has provenance.

[ ] Stage 3 cannot directly mutate persistent state.

[ ] Stage 5 validates all proposed changes.

[ ] Historical changes are auditable.

[ ] The underlying LLM can be replaced without losing state.

[ ] LLM providers are abstracted.

[ ] Malformed model output cannot corrupt the database.

[ ] Stage failures degrade gracefully.

[ ] Consolidation can run asynchronously.

[ ] Cognitive state can be inspected through an API.

[ ] Tests cover the complete 0→5 loop.

[ ] No hidden chain-of-thought is persisted or exposed.

[ ] The system does not claim consciousness merely because it maintains a self-model.
