# Build and Test Commands

## Build Commands
- `go build ./cmd/requiem` — Build main binary
- `go build ./internal/... ./pkg/...` — Build internal packages
- `go build ./ReqiCode/...` — Build ReQiCode daemon

## Test Commands
- `go test ./tests/... ./internal/... -count=1 -v` — Run all tests with verbose output
- `go test ./tests/benchmark/... -bench=. -count=1` — Run benchmarks
- `go test ./tests/benchmark/... -v` — Run benchmark suite test
- `go test ./internal/cognitive/pipeline/... -run TestVerify -v` — Run verification policy tests
- `go test ./internal/cognitive/pipeline/... -run TestEvaluateClaim -v` — Run claim evaluation tests

## Lint Commands
- `go vet ./internal/... ./pkg/... ./cmd/... ./tests/...` — Lint check
- `go vet ./...` — Full lint

## Verification Policy Tests
- `go test ./internal/cognitive/pipeline/... -run "TestEvaluateClaim|TestCreateClaim|TestBuildResponsePlan|TestAssessEvidence|TestRequiresVerification" -v` — Run all verification policy tests

## Benchmark Framework
- `go test ./tests/benchmark/... -bench=. -count=1` — Run benchmarks
- Results saved to temp dir by default
