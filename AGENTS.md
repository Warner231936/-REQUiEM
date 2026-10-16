# AGENTS.md

## Build Commands
- `go build ./...` — Build all packages
- `go build ./cmd/requiem` — Build main binary
- `go build ./internal/... ./pkg/...` — Build internal packages only
- `go test ./tests/... ./internal/... -count=1` — Run all tests
- `go test ./tests/benchmark/... -count=1 -v` — Run benchmark suite
- `go vet ./internal/... ./pkg/... ./cmd/... ./tests/...` — Lint check
- `go test ./tests/benchmark/... -bench=. -count=1` — Run benchmarks
- `go test ./internal/cognitive/pipeline/... -run TestEvaluateClaim -v` — Run verification tests

## Code Style
- Package names: lowercase, no separators
- Error handling: wrap errors with `%w`
- Struct field ordering: public fields first, then private
- Context parameter: first parameter in function signatures
- Logging: use `slog` with structured key-value pairs

## Testing
- Tests use `package pipeline` (internal) or `pipeline_test` (external) naming
- Mock stores defined in `tests/mock_store.go`
- Mock stores in `tests/` must implement `storage.Store`
- API mock stores in `internal/api/mock_test_helpers.go`

## Architecture Notes
- Two separate Go modules: `github.com/requiem/cognitive` (root) and `github.com/kilo/spiral-codemaker` (ReqiCode/)
- Cannot import ReQiCode types directly; use HTTP/API boundary via `internal/cognitive/integration/`
- MongoDB collections: see `internal/storage/mongo.go` for collection names
- Evidence/Claim/Verification stores: `EvidenceStore`, `ClaimStore`, `VerificationStore` interfaces in `internal/storage/interfaces.go`

## Verification Policy
- `VerificationPolicy` is in `internal/cognitive/pipeline/verification_policy.go`
- Gate threshold defaults to `0.8` confidence
- Technical claims below gate are submitted to ReQiCode for verification
- Evidence is recorded in MongoDB `evidence_records` collection
- Verification plans stored in `verification_plans` collection
- Verification results stored in `verification_results` collection
- Claims stored in `claims` collection

## Pipeline Integration
- Stage 4 (Response) uses `WithVerificationPolicy` option for tool evidence recording
- Stage 6 (Consolidation) uses `SetVerificationPolicy` for claim recording from learned facts
- ReQiCodeTool in `internal/cognitive/tool/reqicode.go` uses integration client internally

## Benchmark Framework
- Tests in `tests/benchmark/benchmark_test.go`
- Results saved to temp dir by default
- Run with: `go test ./tests/benchmark/... -v`

