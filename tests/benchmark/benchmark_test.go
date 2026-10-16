package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/requiem/cognitive/internal/cognitive/pipeline"
	"github.com/requiem/cognitive/pkg/models"
	pipelinetest "github.com/requiem/cognitive/tests"
)

type BenchmarkSuite struct {
	store      *pipelinetest.MockStore
	policy     *pipeline.VerificationPolicy
	logger     *slog.Logger
	resultsDir string
}

type BenchmarkResult struct {
	Name       string             `json:"name"`
	Iterations int                `json:"iterations"`
	Duration   time.Duration      `json:"duration"`
	PerOp      time.Duration      `json:"per_op_ns"`
	ClaimID    string             `json:"claim_id,omitempty"`
	Outcome    string             `json:"outcome"`
	Metadata   map[string]any     `json:"metadata,omitempty"`
}

func NewBenchmarkSuite(t *testing.T) *BenchmarkSuite {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	store := pipelinetest.NewMockStore()

	resultsDir := filepath.Join(t.TempDir(), "benchmark_results")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Fatalf("failed to create results dir: %v", err)
	}

	return &BenchmarkSuite{
		store:      store,
		policy:     pipeline.NewVerificationPolicy(store, nil, logger, models.Confidence(0.8), 30*time.Second),
		logger:     logger,
		resultsDir: resultsDir,
	}
}

func (bs *BenchmarkSuite) RunClaimCreation(b *testing.B) *BenchmarkResult {
	ctx := context.Background()
	result := &BenchmarkResult{
		Name:       "Claim_Creation",
		Iterations: b.N,
	}
	start := time.Now()

	for i := 0; i < b.N; i++ {
		claim, err := bs.policy.CreateClaim(ctx, fmt.Sprintf("subject-%d", i), fmt.Sprintf("Test claim %d for verification", i), models.Confidence(0.5), nil)
		if err != nil {
			b.Fatal(err)
		}
		result.ClaimID = claim.ID
	}

	result.Duration = time.Since(start)
	result.PerOp = time.Duration(int64(result.Duration) / int64(b.N))
	result.Outcome = "success"
	return result
}

func (bs *BenchmarkSuite) RunPolicyEvaluation(b *testing.B) *BenchmarkResult {
	ctx := context.Background()
	claim, _ := bs.policy.CreateClaim(ctx, "code_architecture", "The system uses a microservices architecture", models.Confidence(0.3), nil)

	result := &BenchmarkResult{
		Name:       "Policy_Evaluation",
		Iterations: b.N,
	}
	start := time.Now()

	for i := 0; i < b.N; i++ {
		eval := bs.policy.EvaluateClaim(claim)
		_ = eval
	}

	result.Duration = time.Since(start)
	result.PerOp = time.Duration(int64(result.Duration) / int64(b.N))
	result.ClaimID = claim.ID
	result.Outcome = "evaluated"
	return result
}

func (bs *BenchmarkSuite) RunClaimCreationParallel(b *testing.B) *BenchmarkResult {
	ctx := context.Background()
	result := &BenchmarkResult{
		Name:       "Claim_Creation_Parallel",
		Iterations: b.N,
	}

	var counter int64
	var wg sync.WaitGroup
	var errs []error
	var errMu sync.Mutex
	workers := runtime.NumCPU()
	workPerWorker := b.N / workers
	if workPerWorker < 1 {
		workPerWorker = 1
	}

	start := time.Now()

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < workPerWorker; i++ {
				idx := atomic.AddInt64(&counter, 1)
				_, err := bs.policy.CreateClaim(ctx, fmt.Sprintf("parallel-subject-%d", idx), fmt.Sprintf("Parallel claim %d", idx), models.Confidence(0.5), nil)
				if err != nil {
					errMu.Lock()
					errs = append(errs, err)
					errMu.Unlock()
					return
				}
			}
		}()
	}

	wg.Wait()

	for _, e := range errs {
		b.Fatal(e)
	}

	result.Duration = time.Since(start)
	result.PerOp = time.Duration(int64(result.Duration) / int64(b.N))
	result.Outcome = "success"
	return result
}

func (bs *BenchmarkSuite) SaveResults(results []*BenchmarkResult) error {
	allResults, err := os.ReadFile(filepath.Join(bs.resultsDir, "benchmark_results.json"))
	if err != nil || len(allResults) == 0 {
		allResults = []byte("[]")
	}

	var existing []BenchmarkResult
	if err := json.Unmarshal(allResults, &existing); err != nil {
		existing = []BenchmarkResult{}
	}
	for _, r := range results {
		existing = append(existing, *r)
	}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}

	return os.WriteFile(filepath.Join(bs.resultsDir, "benchmark_results.json"), data, 0644)
}

func RunSuite(b *testing.B) {
	bs := NewBenchmarkSuite(&testing.T{})

	var results []*BenchmarkResult

	res1 := bs.RunClaimCreation(b)
	results = append(results, res1)

	res2 := bs.RunPolicyEvaluation(b)
	results = append(results, res2)

	res3 := bs.RunClaimCreationParallel(b)
	results = append(results, res3)

	if err := bs.SaveResults(results); err != nil {
		b.Fatalf("failed to save results: %v", err)
	}

	b.Logf("Benchmark suite completed. Results saved to %s", bs.resultsDir)
	for _, r := range results {
		b.Logf("  %s: %d iterations, %v total, %v/op, outcome=%s", r.Name, r.Iterations, r.Duration, r.PerOp, r.Outcome)
	}
}

func BenchmarkClaimCreation(b *testing.B) {
	bs := NewBenchmarkSuite(&testing.T{})
	res := bs.RunClaimCreation(b)
	b.Logf("Claim creation: %s, %d ops, %v/op", res.Name, res.Iterations, res.PerOp)
}

func BenchmarkPolicyEvaluation(b *testing.B) {
	bs := NewBenchmarkSuite(&testing.T{})
	res := bs.RunPolicyEvaluation(b)
	b.Logf("Policy evaluation: %s, %d ops, %v/op", res.Name, res.Iterations, res.PerOp)
}

func BenchmarkClaimCreationParallel(b *testing.B) {
	bs := NewBenchmarkSuite(&testing.T{})
	res := bs.RunClaimCreationParallel(b)
	b.Logf("Claim creation parallel: %s, %d ops, %v/op", res.Name, res.Iterations, res.PerOp)
}

func TestBenchmarkSuite(t *testing.T) {
	runtime.GC()

	ctx := context.Background()
	bs := NewBenchmarkSuite(t)

	b := &testing.B{}
	b.N = 100
	_ = ctx

	res1 := bs.RunClaimCreation(b)
	t.Logf("  %s: %d iterations, %v/op, outcome=%s", res1.Name, res1.Iterations, res1.PerOp, res1.Outcome)

	b = &testing.B{}
	b.N = 100
	res2 := bs.RunPolicyEvaluation(b)
	t.Logf("  %s: %d iterations, %v/op, outcome=%s", res2.Name, res2.Iterations, res2.PerOp, res2.Outcome)

	b = &testing.B{}
	b.N = 100
	res3 := bs.RunClaimCreationParallel(b)
	t.Logf("  %s: %d iterations, %v/op, outcome=%s", res3.Name, res3.Iterations, res3.PerOp, res3.Outcome)

	if err := bs.SaveResults([]*BenchmarkResult{res1, res2, res3}); err != nil {
		t.Fatalf("failed to save results: %v", err)
	}

	t.Logf("Benchmark suite completed. Results saved to %s", bs.resultsDir)
}
