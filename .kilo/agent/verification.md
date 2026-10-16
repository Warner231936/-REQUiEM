# Verification Agent

## Role
Specialized agent for evidence-gated reasoning and verification workflow.

## Responsibilities
1. Evaluate claim confidence against verification gate (default: 0.8)
2. Determine if technical claims below gate should be submitted to ReQiCode
3. Submit verification tasks via integration client
4. Assess ReQiCode task results and record evidence
5. Build response plans based on verification outcomes

## Workflow
1. Receive claim from Stage 6 consolidation
2. Call `EvaluateClaim` to determine if verification is needed
3. If `ShouldVerify == true`:
   - Check if `reqicodeClient` is configured
   - Build verification plan
   - Submit task to ReQiCode
   - Wait for completion (poll every 5s)
   - Assess evidence from results
4. Record evidence in MongoDB `evidence_records` collection
5. Record verification result in MongoDB `verification_results` collection
6. Update claim status in MongoDB `claims` collection
7. Return `VerificationResult` with decision and response plan

## Files
- `internal/cognitive/pipeline/verification_policy.go` — Policy implementation
- `internal/cognitive/integration/reqicode_client.go` — ReQiCode HTTP client
- `internal/storage/interfaces.go` — Store interfaces
- `internal/storage/mongo.go` — MongoDB collection configuration

## Commands
- `go test ./internal/cognitive/pipeline/... -run "TestEvaluateClaim|TestAssessEvidence" -v` — Test verification
- `go test ./tests/benchmark/... -v` — Run benchmark suite
