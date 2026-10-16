package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/requiem/cognitive/internal/cognitive/integration"
	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/pkg/models"
	pipelinetest "github.com/requiem/cognitive/tests"
)

func newTestPolicyWithClient(t *testing.T, gate models.Confidence, client *integration.ReQiCodeClient) (*VerificationPolicy, *pipelinetest.MockStore) {
	t.Helper()
	policy, store := newTestPolicy(t, gate)
	policy.reqicodeClient = client
	return policy, store
}

func newMockReQiCodeServer(t *testing.T, submitHandler, resultHandler http.HandlerFunc) (*integration.ReQiCodeClient, func()) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/engineering", submitHandler)
	mux.HandleFunc("/api/runs/", resultHandler)

	server := httptest.NewServer(mux)
	client := integration.NewReQiCodeClient(server.URL, 2)
	return client, server.Close
}

func TestEvaluateAndVerify_NoVerificationNeeded(t *testing.T) {
	claim := models.Claim{
		BaseRecord:  models.NewBaseRecord("claim-1"),
		Subject:     "weather_forecast",
		Content:     "It will rain tomorrow",
		Status:      models.ClaimProposed,
		Confidence:  0.9,
		EvidenceIDs: []string{},
	}

	policy, store := newTestPolicyWithClient(t, models.Confidence(0.8), nil)
	if err := store.Claim().CreateClaim(context.Background(), claim); err != nil {
		t.Fatalf("CreateClaim failed: %v", err)
	}

	result := policy.EvaluateAndVerify(context.Background(), claim, nil)

	if result.Decision != models.VerificationDecisionAccept {
		t.Errorf("expected decision %s, got %s", models.VerificationDecisionAccept, result.Decision)
	}
	if !result.Verified {
		t.Error("expected verified to be true")
	}
	if result.AssessedValidity != models.ValidityConfirmed {
		t.Errorf("expected validity %s, got %s", models.ValidityConfirmed, result.AssessedValidity)
	}
	if result.Plan.Decision != models.DecisionAcceptProvisional {
		t.Errorf("expected plan decision %s, got %s", models.DecisionAcceptProvisional, result.Plan.Decision)
	}

	evRecs, err := store.Evidence().GetByClaim(context.Background(), claim.ID)
	if err != nil {
		t.Fatalf("GetByClaim failed: %v", err)
	}
	if len(evRecs) != 1 {
		t.Errorf("expected 1 evidence record, got %d", len(evRecs))
	}

	verif, err := store.Verification().GetResult(context.Background(), claim.ID)
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}
	if verif.Verified != result.Verified {
		t.Error("stored verification result mismatch")
	}
}

func TestEvaluateAndVerify_TechnicalClaim_NoClient(t *testing.T) {
	claim := models.Claim{
		BaseRecord:  models.NewBaseRecord("claim-2"),
		Subject:     "api_endpoint",
		Content:     "The API endpoint returns user data",
		Status:      models.ClaimProposed,
		Confidence:  0.3,
		EvidenceIDs: []string{"ev-1"},
	}

	policy, _ := newTestPolicyWithClient(t, models.Confidence(0.8), nil)

	result := policy.EvaluateAndVerify(context.Background(), claim, []string{"must return 200"})

	if result.Decision != models.VerificationDecisionReject {
		t.Errorf("expected decision %s (no client), got %s", models.VerificationDecisionReject, result.Decision)
	}
	if result.Verified {
		t.Error("expected verified to be false")
	}
	if result.Error == "" {
		t.Error("expected error message for missing client")
	}
}

func TestEvaluateAndVerify_TechnicalClaim_Verified(t *testing.T) {
	submitted := false

	client, cleanup := newMockReQiCodeServer(t,
		func(w http.ResponseWriter, r *http.Request) {
			submitted = true
			var req models.TaskRequest
			json.NewDecoder(r.Body).Decode(&req)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "accepted",
				"run_id":  "run-verify-123",
				"task_id": req.TaskID,
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{
				"id": "run-verify-123",
				"task_id": "verify-claim-2",
				"status": "COMPLETED",
				"result": {
					"status": "passed",
					"verification_status": "passed",
					"duration_ms": 2500
				}
			}`)
		},
	)
	defer cleanup()

	claim := models.Claim{
		BaseRecord:  models.NewBaseRecord("claim-verified"),
		Subject:     "api_endpoint",
		Content:     "The API endpoint returns user data",
		Status:      models.ClaimProposed,
		Confidence:  0.5,
		EvidenceIDs: []string{"ev-1"},
	}

	policy, store := newTestPolicyWithClient(t, models.Confidence(0.8), client)
	if err := store.Claim().CreateClaim(context.Background(), claim); err != nil {
		t.Fatalf("CreateClaim failed: %v", err)
	}

	result := policy.EvaluateAndVerify(context.Background(), claim, []string{"returns 200 OK"})

	if !submitted {
		t.Error("expected task to be submitted to ReQiCode")
	}
	if result.Decision != models.VerificationDecisionAccept {
		t.Errorf("expected decision %s, got %s", models.VerificationDecisionAccept, result.Decision)
	}
	if !result.Verified {
		t.Error("expected verified to be true")
	}
	if result.AssessedValidity != models.ValidityConfirmed {
		t.Errorf("expected validity %s, got %s", models.ValidityConfirmed, result.AssessedValidity)
	}

	savedClaim, err := store.Claim().GetClaim(context.Background(), claim.ID)
	if err != nil {
		t.Fatalf("GetClaim failed: %v", err)
	}
	if savedClaim.Status != models.ClaimVerified {
		t.Errorf("expected claim status %s, got %s", models.ClaimVerified, savedClaim.Status)
	}

	evRecs, err := store.Evidence().GetByClaim(context.Background(), claim.ID)
	if err != nil {
		t.Fatalf("GetByClaim failed: %v", err)
	}
	if len(evRecs) != 1 {
		t.Errorf("expected 1 evidence record, got %d", len(evRecs))
	}
	if evRecs[0].EvidenceSource != models.EvidenceSourceVerification {
		t.Errorf("expected evidence source %s, got %s", models.EvidenceSourceVerification, evRecs[0].EvidenceSource)
	}
}

func TestEvaluateAndVerify_TechnicalClaim_Refuted(t *testing.T) {
	client, cleanup := newMockReQiCodeServer(t,
		func(w http.ResponseWriter, r *http.Request) {
			var req models.TaskRequest
			json.NewDecoder(r.Body).Decode(&req)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "accepted",
				"run_id":  "run-refute-456",
				"task_id": req.TaskID,
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{
				"id": "run-refute-456",
				"task_id": "verify-refute",
				"status": "COMPLETED",
				"result": {
					"status": "passed",
					"verification_status": "failed",
					"duration_ms": 1800,
					"error_summary": "tests did not pass"
				}
			}`)
		},
	)
	defer cleanup()

	claim := models.Claim{
		BaseRecord:  models.NewBaseRecord("claim-refuted"),
		Subject:     "code_function",
		Content:     "The function handles all edge cases",
		Status:      models.ClaimProposed,
		Confidence:  0.4,
		EvidenceIDs: []string{},
	}

	policy, store := newTestPolicyWithClient(t, models.Confidence(0.8), client)
	if err := store.Claim().CreateClaim(context.Background(), claim); err != nil {
		t.Fatalf("CreateClaim failed: %v", err)
	}

	result := policy.EvaluateAndVerify(context.Background(), claim, nil)

	if result.Decision != models.VerificationDecisionReject {
		t.Errorf("expected decision %s, got %s", models.VerificationDecisionReject, result.Decision)
	}
	if result.Verified {
		t.Error("expected verified to be false")
	}
	if result.AssessedValidity != models.ValidityRefuted {
		t.Errorf("expected validity %s, got %s", models.ValidityRefuted, result.AssessedValidity)
	}

	savedClaim, err := store.Claim().GetClaim(context.Background(), claim.ID)
	if err != nil {
		t.Fatalf("GetClaim failed: %v", err)
	}
	if savedClaim.Status != models.ClaimContradicted {
		t.Errorf("expected claim status %s, got %s", models.ClaimContradicted, savedClaim.Status)
	}
}

func TestEvaluateAndVerify_SubmitTaskFailure(t *testing.T) {
	client, cleanup := newMockReQiCodeServer(t,
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, `{"error": "ReqiCode daemon unavailable"}`)
		},
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cleanup()

	claim := models.Claim{
		BaseRecord:  models.NewBaseRecord("claim-submit-fail"),
		Subject:     "database_schema",
		Content:     "The database schema supports horizontal sharding",
		Status:      models.ClaimProposed,
		Confidence:  0.5,
		EvidenceIDs: []string{"ev-1"},
	}

	policy, _ := newTestPolicyWithClient(t, models.Confidence(0.8), client)

	result := policy.EvaluateAndVerify(context.Background(), claim, nil)

	if result.Decision != models.VerificationDecisionReject {
		t.Errorf("expected decision %s, got %s", models.VerificationDecisionReject, result.Decision)
	}
	if result.Verified {
		t.Error("expected verified to be false")
	}
	if result.Error == "" {
		t.Error("expected error message for submit failure")
	}
}

func TestEvaluateAndVerify_TaskFailed(t *testing.T) {
	client, cleanup := newMockReQiCodeServer(t,
		func(w http.ResponseWriter, r *http.Request) {
			var req models.TaskRequest
			json.NewDecoder(r.Body).Decode(&req)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "accepted",
				"run_id":  "run-failed-789",
				"task_id": req.TaskID,
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{
				"id": "run-failed-789",
				"task_id": "verify-failed",
				"status": "FAILED",
				"result": {
					"status": "failed",
					"verification_status": "failed",
					"duration_ms": 3200,
					"error_summary": "Build failed: undefined symbol"
				}
			}`)
		},
	)
	defer cleanup()

	claim := models.Claim{
		BaseRecord:  models.NewBaseRecord("claim-task-failed"),
		Subject:     "system_architecture",
		Content:     "The architecture follows a microservices pattern",
		Status:      models.ClaimProposed,
		Confidence:  0.6,
		EvidenceIDs: []string{},
	}

	policy, _ := newTestPolicyWithClient(t, models.Confidence(0.8), client)

	result := policy.EvaluateAndVerify(context.Background(), claim, nil)

	if result.Decision != models.VerificationDecisionReject {
		t.Errorf("expected decision %s, got %s", models.VerificationDecisionReject, result.Decision)
	}
	if result.Verified {
		t.Error("expected verified to be false")
	}
	if result.AssessedValidity != models.ValidityRefuted {
		t.Errorf("expected validity %s, got %s", models.ValidityRefuted, result.AssessedValidity)
	}
}

func TestEvaluateAndVerify_HighConfidenceTechnical(t *testing.T) {
	client, cleanup := newMockReQiCodeServer(t,
		func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not submit verification task for high-confidence claim")
		},
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cleanup()

	claim := models.Claim{
		BaseRecord:  models.NewBaseRecord("claim-high"),
		Subject:     "api_endpoint",
		Content:     "The API endpoint returns user data",
		Status:      models.ClaimProposed,
		Confidence:  0.95,
		EvidenceIDs: []string{"ev-1", "ev-2"},
	}

	policy, store := newTestPolicyWithClient(t, models.Confidence(0.8), client)
	if err := store.Claim().CreateClaim(context.Background(), claim); err != nil {
		t.Fatalf("CreateClaim failed: %v", err)
	}

	result := policy.EvaluateAndVerify(context.Background(), claim, nil)

	if result.Decision != models.VerificationDecisionAccept {
		t.Errorf("expected decision %s, got %s", models.VerificationDecisionAccept, result.Decision)
	}
	if !result.Verified {
		t.Error("expected verified to be true for high-confidence claim")
	}
}

func TestEvaluateAndVerify_PlanRecorded(t *testing.T) {
	client, cleanup := newMockReQiCodeServer(t,
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "accepted",
				"run_id":  "run-plan-test",
				"task_id": "verify-claim-plan",
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{
				"id": "run-plan-test",
				"task_id": "verify-claim-plan",
				"status": "COMPLETED",
				"result": {
					"status": "passed",
					"verification_status": "passed",
					"duration_ms": 5000
				}
			}`)
		},
	)
	defer cleanup()

	claim := models.Claim{
		BaseRecord:  models.NewBaseRecord("claim-plan"),
		Subject:     "code_function",
		Content:     "The function correctly handles nil inputs",
		Status:      models.ClaimProposed,
		Confidence:  0.5,
		EvidenceIDs: []string{"ev-1"},
	}

	policy, store := newTestPolicyWithClient(t, models.Confidence(0.8), client)
	if err := store.Claim().CreateClaim(context.Background(), claim); err != nil {
		t.Fatalf("CreateClaim failed: %v", err)
	}

	criteria := []string{"must compile", "all tests pass", "no panics on nil input"}
	result := policy.EvaluateAndVerify(context.Background(), claim, criteria)

	if len(result.Plan.AcceptanceCriteria) != 3 {
		t.Errorf("expected 3 acceptance criteria in plan, got %d", len(result.Plan.AcceptanceCriteria))
	}
	if result.Plan.ClaimID != claim.ID {
		t.Errorf("expected plan claim ID %s, got %s", claim.ID, result.Plan.ClaimID)
	}

	storedPlan, err := store.Verification().GetPlan(context.Background(), claim.ID)
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if storedPlan.ClaimID != claim.ID {
		t.Errorf("stored plan claim ID mismatch: %s != %s", storedPlan.ClaimID, claim.ID)
	}
}

func TestRecordToolEvidence_GetResult_Success(t *testing.T) {
	store := pipelinetest.NewMockStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client, cleanup := newMockReQiCodeServer(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cleanup()

	policy := NewVerificationPolicy(store, client, logger, models.Confidence(0.8), 30*time.Second)
	stage := NewStage4Response(llm.NewProviderRegistry(logger), logger, WithVerificationPolicy(policy))

	stage.recordToolEvidence(context.Background(), "reqicode", map[string]any{
		"action": "get_result",
		"run_id": "run-ev-001",
	}, "Verification passed", "", "turn-001")

	recs, err := store.Evidence().GetByClaim(context.Background(), "")
	if err != nil {
		t.Fatalf("GetByClaim failed: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("expected evidence record to be recorded for successful get_result")
	}
	ev := recs[len(recs)-1]
	if ev.EvidenceSource != models.EvidenceSourceReqiCodeRun {
		t.Errorf("expected evidence source %s, got %s", models.EvidenceSourceReqiCodeRun, ev.EvidenceSource)
	}
	if ev.Validity != models.ValidityUnverified {
		t.Errorf("expected validity %s, got %s", models.ValidityUnverified, ev.Validity)
	}
	if ev.ArtifactLocation != "run-ev-001" {
		t.Errorf("expected artifact location 'run-ev-001', got %s", ev.ArtifactLocation)
	}
}

func TestRecordToolEvidence_GetResult_Failure(t *testing.T) {
	store := pipelinetest.NewMockStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client, cleanup := newMockReQiCodeServer(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cleanup()

	policy := NewVerificationPolicy(store, client, logger, models.Confidence(0.8), 30*time.Second)
	stage := NewStage4Response(llm.NewProviderRegistry(logger), logger, WithVerificationPolicy(policy))

	stage.recordToolEvidence(context.Background(), "reqicode", map[string]any{
		"action": "get_result",
		"run_id": "run-fail-001",
	}, "", "verification failed: tests did not pass", "turn-002")

	recs, err := store.Evidence().GetByClaim(context.Background(), "")
	if err != nil {
		t.Fatalf("GetByClaim failed: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("expected evidence record to be recorded for failed get_result")
	}
	ev := recs[len(recs)-1]
	if ev.Validity != models.ValidityRefuted {
		t.Errorf("expected validity %s for failed result, got %s", models.ValidityRefuted, ev.Validity)
	}
	if ev.Confidence != 0.3 {
		t.Errorf("expected confidence 0.3 for failed result, got %f", ev.Confidence)
	}
	if ev.Limitations != "verification failed: tests did not pass" {
		t.Errorf("expected limitations to contain error message")
	}
}

func TestRecordToolEvidence_Submit(t *testing.T) {
	store := pipelinetest.NewMockStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client, cleanup := newMockReQiCodeServer(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cleanup()

	policy := NewVerificationPolicy(store, client, logger, models.Confidence(0.8), 30*time.Second)
	stage := NewStage4Response(llm.NewProviderRegistry(logger), logger, WithVerificationPolicy(policy))

	stage.recordToolEvidence(context.Background(), "reqicode", map[string]any{
		"action":   "submit",
		"task_id":  "task-ev-001",
		"objective": "Fix the bug",
	}, "Engineering task submitted successfully. Run ID: run-abc", "", "turn-003")

	recs, err := store.Evidence().GetByClaim(context.Background(), "")
	if err != nil {
		t.Fatalf("GetByClaim failed: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("expected evidence record to be recorded for submit")
	}
	ev := recs[len(recs)-1]
	if ev.EvidenceSource != models.EvidenceSourceReqiCodeRun {
		t.Errorf("expected evidence source %s, got %s", models.EvidenceSourceReqiCodeRun, ev.EvidenceSource)
	}
	if ev.Validity != models.ValidityPending {
		t.Errorf("expected validity %s for submit, got %s", models.ValidityPending, ev.Validity)
	}
}

func TestRecordToolEvidence_Cancel(t *testing.T) {
	store := pipelinetest.NewMockStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client, cleanup := newMockReQiCodeServer(t,
		func(w http.ResponseWriter, r *http.Request) {},
		func(w http.ResponseWriter, r *http.Request) {},
	)
	defer cleanup()

	policy := NewVerificationPolicy(store, client, logger, models.Confidence(0.8), 30*time.Second)
	stage := NewStage4Response(llm.NewProviderRegistry(logger), logger, WithVerificationPolicy(policy))

	stage.recordToolEvidence(context.Background(), "reqicode", map[string]any{
		"action": "cancel",
		"run_id": "run-cancel-001",
	}, "Run run-cancel-001 cancellation requested.", "", "turn-004")

	recs, err := store.Evidence().GetByClaim(context.Background(), "")
	if err != nil {
		t.Fatalf("GetByClaim failed: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("expected evidence record to be recorded for cancel")
	}
	ev := recs[len(recs)-1]
	if ev.Validity != models.ValidityAmbiguous {
		t.Errorf("expected validity %s for cancel, got %s", models.ValidityAmbiguous, ev.Validity)
	}
}

func TestRecordToolEvidence_NonReqiCodeTool(t *testing.T) {
	store := pipelinetest.NewMockStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	policy := NewVerificationPolicy(store, nil, logger, models.Confidence(0.8), 30*time.Second)
	stage := NewStage4Response(llm.NewProviderRegistry(logger), logger, WithVerificationPolicy(policy))

	stage.recordToolEvidence(context.Background(), "search_memories", map[string]any{
		"action": "search",
	}, "Found 3 results", "", "turn-005")

	recs, err := store.Evidence().GetByClaim(context.Background(), "")
	if err != nil {
		t.Fatalf("GetByClaim failed: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected 0 evidence records for non-reqicode tool, got %d", len(recs))
	}
}

func TestRecordToolEvidence_NoVerificationPolicy(t *testing.T) {
	store := pipelinetest.NewMockStore()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	stage := NewStage4Response(llm.NewProviderRegistry(logger), logger)

	stage.recordToolEvidence(context.Background(), "reqicode", map[string]any{
		"action": "get_result",
		"run_id": "run-none",
	}, "result", "", "turn-006")

	recs, err := store.Evidence().GetByClaim(context.Background(), "")
	if err != nil {
		t.Fatalf("GetByClaim failed: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected 0 evidence records when no policy, got %d", len(recs))
	}
}
