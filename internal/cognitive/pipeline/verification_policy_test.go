package pipeline

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/requiem/cognitive/internal/cognitive/integration"
	"github.com/requiem/cognitive/pkg/models"
	pipelinetest "github.com/requiem/cognitive/tests"
)

func newTestPolicy(t *testing.T, gate models.Confidence) (*VerificationPolicy, *pipelinetest.MockStore) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	store := pipelinetest.NewMockStore()
	policy := NewVerificationPolicy(store, nil, logger, gate, 30*time.Second)
	return policy, store
}

func TestEvaluateClaim_LowConfidenceNonTechnical(t *testing.T) {
	policy, _ := newTestPolicy(t, models.Confidence(0.9))
	ctx := context.Background()

	claim, err := policy.CreateClaim(ctx, "weather", "It will rain tomorrow", models.Confidence(0.3), nil)
	if err != nil {
		t.Fatalf("CreateClaim failed: %v", err)
	}

	eval := policy.EvaluateClaim(claim)
	if eval.ShouldVerify {
		t.Errorf("non-technical claim should not require verification")
	}
}

func TestEvaluateClaim_LowConfidenceTechnical(t *testing.T) {
	policy, _ := newTestPolicy(t, models.Confidence(0.9))
	ctx := context.Background()

	claim, err := policy.CreateClaim(ctx, "database_schema", "The database uses PostgreSQL with JSONB columns", models.Confidence(0.3), nil)
	if err != nil {
		t.Fatalf("CreateClaim failed: %v", err)
	}

	eval := policy.EvaluateClaim(claim)
	if !eval.ShouldVerify {
		t.Errorf("technical claim below gate should require verification")
	}
}

func TestEvaluateClaim_HighConfidence(t *testing.T) {
	policy, _ := newTestPolicy(t, models.Confidence(0.8))
	ctx := context.Background()

	claim, err := policy.CreateClaim(ctx, "api_endpoint", "The API endpoint /api/v1/users returns user data", models.Confidence(0.9), nil)
	if err != nil {
		t.Fatalf("CreateClaim failed: %v", err)
	}

	eval := policy.EvaluateClaim(claim)
	if eval.ShouldVerify {
		t.Errorf("high confidence claim should not require verification")
	}
}

func TestCreateClaim_Success(t *testing.T) {
	policy, store := newTestPolicy(t, models.Confidence(0.8))
	ctx := context.Background()

	evidence := []EvidenceItem{
		{Description: "test evidence 1", Source: models.EvidenceSourceObservation},
		{Description: "test evidence 2", Source: models.EvidenceSourceTestPass},
	}

	claim, err := policy.CreateClaim(ctx, "code_function", "The function handles nil inputs", models.Confidence(0.5), evidence)
	if err != nil {
		t.Fatalf("CreateClaim failed: %v", err)
	}

	if claim.ID == "" {
		t.Error("expected claim ID to be set")
	}
	if claim.Status != models.ClaimProposed {
		t.Errorf("expected status %s, got %s", models.ClaimProposed, claim.Status)
	}
	if claim.Subject != "code_function" {
		t.Errorf("expected subject 'code_function', got '%s'", claim.Subject)
	}
	if len(claim.EvidenceIDs) != 2 {
		t.Errorf("expected 2 evidence IDs, got %d", len(claim.EvidenceIDs))
	}

	saved, err := store.Claim().GetClaim(ctx, claim.ID)
	if err != nil {
		t.Fatalf("GetClaim failed: %v", err)
	}
	if saved.ID != claim.ID {
		t.Errorf("saved claim ID mismatch: %s != %s", saved.ID, claim.ID)
	}
}

func TestBuildResponsePlan_Verified(t *testing.T) {
	policy, _ := newTestPolicy(t, models.Confidence(0.8))
	ctx := context.Background()

	claim, _ := policy.CreateClaim(ctx, "api", "API returns correct data", models.Confidence(0.5), nil)

	verification := models.VerificationResult{
		ClaimID:          claim.ID,
		Verified:         true,
		Confidence:       0.95,
		AssessedValidity: models.ValidityConfirmed,
	}

	plan := policy.BuildResponsePlan(claim, verification)
	if plan.Output != "Claim confirmed: API returns correct data" {
		t.Errorf("unexpected output: %s", plan.Output)
	}
	if plan.Confidence != models.Confidence(0.95) {
		t.Errorf("unexpected confidence: %f", plan.Confidence)
	}
}

func TestBuildResponsePlan_NotVerified(t *testing.T) {
	policy, _ := newTestPolicy(t, models.Confidence(0.8))
	ctx := context.Background()

	claim, _ := policy.CreateClaim(ctx, "api", "API returns correct data", models.Confidence(0.5), nil)

	verification := models.VerificationResult{
		ClaimID:          claim.ID,
		Verified:         false,
		Error:            "verification failed",
		AssessedValidity: models.ValidityRefuted,
	}

	plan := policy.BuildResponsePlan(claim, verification)
	if plan.Output != "Claim could not be fully verified: API returns correct data" {
		t.Errorf("unexpected output: %s", plan.Output)
	}
	if len(plan.Uncertainty) != 1 {
		t.Errorf("expected 1 uncertainty, got %d", len(plan.Uncertainty))
	}
}

func TestAssessEvidence_Completed(t *testing.T) {
	client := integration.NewReQiCodeClient("http://localhost:9999", 5)

	result := models.TaskResult{
		Status:               models.TaskStatusCompleted,
		VerificationStatus:   "passed",
	}

	validity := client.AssessEvidence(result)
	if validity != models.ValidityConfirmed {
		t.Errorf("expected %s, got %s", models.ValidityConfirmed, validity)
	}
}

func TestAssessEvidence_Failed(t *testing.T) {
	client := integration.NewReQiCodeClient("http://localhost:9999", 5)

	result := models.TaskResult{
		Status: models.TaskStatusFailed,
	}

	validity := client.AssessEvidence(result)
	if validity != models.ValidityRefuted {
		t.Errorf("expected %s, got %s", models.ValidityRefuted, validity)
	}
}

func TestAssessEvidence_NoData(t *testing.T) {
	client := integration.NewReQiCodeClient("http://localhost:9999", 5)

	result := models.TaskResult{}

	validity := client.AssessEvidence(result)
	if validity != models.ValidityInsufficient {
		t.Errorf("expected %s, got %s", models.ValidityInsufficient, validity)
	}
}

func TestAssessEvidence_CompletedButUnverified(t *testing.T) {
	client := integration.NewReQiCodeClient("http://localhost:9999", 5)

	result := models.TaskResult{
		Status: models.TaskStatusCompleted,
	}

	validity := client.AssessEvidence(result)
	if validity != models.ValidityUnverified {
		t.Errorf("expected %s, got %s", models.ValidityUnverified, validity)
	}
}

func TestBuildVerificationPlan(t *testing.T) {
	client := integration.NewReQiCodeClient("http://localhost:9999", 5)

	plan := client.BuildVerificationPlan("claim-123", "Verify code implementation", []string{"must compile", "tests pass"})
	if plan.ClaimID != "claim-123" {
		t.Errorf("expected claim_id 'claim-123', got '%s'", plan.ClaimID)
	}
	if plan.Objective != "Verify code implementation" {
		t.Errorf("unexpected objective: %s", plan.Objective)
	}
	if len(plan.ClaimedVerifications) != 2 {
		t.Errorf("expected 2 claimed verifications, got %d", len(plan.ClaimedVerifications))
	}
	if !plan.Reversible {
		t.Error("expected plan to be reversible")
	}
	if plan.CheckerReliability != 0.85 {
		t.Errorf("expected checker reliability 0.85, got %f", plan.CheckerReliability)
	}
}

func TestRequiresVerification_TechnicalSubjects(t *testing.T) {
	technicalSubjects := []string{
		"code_api", "database_schema", "system_architecture",
		"model_implementation", "function_handler",
	}
	for _, subject := range technicalSubjects {
		if !requiresVerification(subject, nil) {
			t.Errorf("expected %s to require verification", subject)
		}
	}
}

func TestRequiresVerification_NonTechnicalSubjects(t *testing.T) {
	nonTechnicalSubjects := []string{
		"weather_forecast", "movie_review", "book_recommendation",
	}
	for _, subject := range nonTechnicalSubjects {
		if requiresVerification(subject, nil) {
			t.Errorf("expected %s to NOT require verification", subject)
		}
	}
}

func TestReviseClaim(t *testing.T) {
	policy, _ := newTestPolicy(t, models.Confidence(0.8))
	ctx := context.Background()

	original, err := policy.CreateClaim(ctx, "api_endpoint", "The API endpoint returns user data", models.Confidence(0.3), nil)
	if err != nil {
		t.Fatalf("CreateClaim failed: %v", err)
	}

	revised, err := policy.ReviseClaim(ctx, original, "The API endpoint returns user data with pagination", nil)
	if err != nil {
		t.Fatalf("ReviseClaim failed: %v", err)
	}

	if revised.OriginalClaimID != original.ID {
		t.Errorf("expected original claim ID %s, got %s", original.ID, revised.OriginalClaimID)
	}
	if revised.RevisedClaimID == "" {
		t.Error("expected revised claim ID to be set")
	}
	if !revised.Accepted {
		t.Error("expected revision to be accepted")
	}

	updated, err := policy.store.Claim().GetClaim(ctx, original.ID)
	if err != nil {
		t.Fatalf("GetClaim failed: %v", err)
	}
	if updated.Status != models.ClaimContradicted {
		t.Errorf("expected original claim to be contradicted, got %s", updated.Status)
	}
}
