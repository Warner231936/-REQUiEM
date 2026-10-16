package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/requiem/cognitive/internal/cognitive/integration"
	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

type VerificationPolicy struct {
	store            storage.Store
	reqicodeClient   *integration.ReQiCodeClient
	logger           *slog.Logger
	defaultTimeout   time.Duration
	verificationGate models.Confidence
}

func NewVerificationPolicy(store storage.Store, client *integration.ReQiCodeClient, logger *slog.Logger, gate models.Confidence, timeout time.Duration) *VerificationPolicy {
	if logger == nil {
		logger = slog.Default()
	}
	return &VerificationPolicy{
		store:            store,
		reqicodeClient:   client,
		logger:           logger,
		verificationGate: gate,
		defaultTimeout:   timeout,
	}
}

type ClaimEvaluation struct {
	ClaimID      string
	Subject      string
	Confidence   models.Confidence
	ShouldVerify bool
	Reason       string
}

func (p *VerificationPolicy) EvaluateClaim(claim models.Claim) ClaimEvaluation {
	eval := ClaimEvaluation{
		ClaimID:    claim.ID,
		Subject:    claim.Subject,
		Confidence: claim.Confidence,
	}

	if claim.Confidence >= p.verificationGate {
		eval.ShouldVerify = false
		eval.Reason = "confidence meets verification gate threshold"
		return eval
	}

	if !requiresVerification(claim.Subject, claim.EvidenceIDs) {
		eval.ShouldVerify = false
		eval.Reason = "claim is non-technical, no verification required"
		return eval
	}

	evidenceCount := len(claim.EvidenceIDs)
	if evidenceCount > 0 {
		eval.ShouldVerify = true
		eval.Reason = fmt.Sprintf("confidence %f below gate %f with %d evidence items", claim.Confidence, p.verificationGate, evidenceCount)
		return eval
	}

	eval.ShouldVerify = true
	eval.Reason = "no evidence and confidence below gate"
	return eval
}

func (p *VerificationPolicy) CreateClaim(ctx context.Context, subject, statement string, confidence models.Confidence, evidence []EvidenceItem) (models.Claim, error) {
	claim := models.Claim{
		BaseRecord:   models.NewBaseRecord(models.NewID()),
		Subject:    subject,
		Content:    statement,
		Status:     models.ClaimProposed,
		Confidence: confidence,
		EvidenceIDs: evidenceToIDs(evidence),
		Provenance: models.Provenance{
			SourceType: string(models.SourceModelInferred),
		},
	}

	if err := p.store.Claim().CreateClaim(ctx, claim); err != nil {
		return models.Claim{}, fmt.Errorf("create claim: %w", err)
	}

	p.logger.Info("created claim",
		"claim_id", claim.ID,
		"subject", claim.Subject,
		"confidence", claim.Confidence,
	)

	return claim, nil
}

func (p *VerificationPolicy) EvaluateAndVerify(ctx context.Context, claim models.Claim, acceptanceCriteria []string) models.VerificationResult {
	eval := p.EvaluateClaim(claim)
	now := time.Now().UTC()
	result := models.VerificationResult{
		ClaimID:       claim.ID,
		ClaimSubject:  claim.Subject,
		CheckedAt:      now,
		AssessedValidity: models.ValidityPending,
	}

	if !eval.ShouldVerify {
		plan := models.VerificationPlan{
			ClaimID:              claim.ID,
			Objective:            fmt.Sprintf("Verify claim about %s", claim.Subject),
			ClaimedVerifications: []string{eval.Reason},
			Methods:              []models.VerificationMethod{models.MethodAbstain},
			Decision:             models.DecisionAcceptProvisional,
		}
		result.Plan = plan
		result.Decision = models.VerificationDecisionAccept
		result.Verified = true
		result.Confidence = 0.9
		result.AssessedValidity = models.ValidityConfirmed

		evRecord := models.EvidenceRecord{
			BaseRecord:         models.NewBaseRecord(models.NewID()),
			ClaimID:            claim.ID,
			Subject:            claim.Subject,
			EvidenceSource:     models.EvidenceSourcePolicy,
			ClaimContent:       eval.Reason,
			ObservedResult:     eval.Reason,
			VerificationMethod: string(models.MethodAbstain),
			Validity:           models.ValidityConfirmed,
			Confidence:         0.9,
			Provenance: models.Provenance{SourceType: "policy"},
		}
		if err := p.store.Evidence().CreateRecord(ctx, evRecord); err != nil {
			p.logger.Warn("failed to record evidence", "error", err)
		}
		_ = p.store.Verification().RecordResult(ctx, result)
		return result
	}

	plan := p.reqicodeClient.BuildVerificationPlan(claim.ID, fmt.Sprintf("Verify claim: %s", claim.Subject), acceptanceCriteria)
	plan.AcceptanceCriteria = acceptanceCriteria
	_ = p.store.Verification().RecordPlan(ctx, plan)
	result.Plan = plan

	if p.reqicodeClient == nil {
		result.Decision = models.VerificationDecisionReject
		result.Error = "no ReQiCode client configured for verification"
		return result
	}

	taskReq := models.TaskRequest{
		TaskID:            fmt.Sprintf("verify-%s", claim.ID),
		Objective:         plan.Objective,
		Rationale:         fmt.Sprintf("Verifying claim: %s (confidence=%f)", claim.Content, claim.Confidence),
		AcceptanceCriteria: acceptanceCriteria,
	}

	runID, err := p.reqicodeClient.SubmitTask(ctx, taskReq)
	if err != nil {
		result.Decision = models.VerificationDecisionReject
		result.Error = fmt.Sprintf("failed to submit verification task: %v", err)
		_ = p.store.Verification().RecordResult(ctx, result)
		return result
	}

	result.RunID = runID
	result.Decision = models.VerificationDecisionVerify

	taskResult, err := p.reqicodeClient.WaitForCompletion(ctx, runID, 5*time.Second)
	if err != nil {
		result.Decision = models.VerificationDecisionReject
		result.Error = fmt.Sprintf("verification task error: %v", err)
		_ = p.store.Verification().RecordResult(ctx, result)
		return result
	}

	result.TaskResult = &taskResult
	result.AssessedValidity = p.reqicodeClient.AssessEvidence(taskResult)
	result.Verified = result.AssessedValidity == models.ValidityConfirmed

	if result.Verified {
		result.Decision = models.VerificationDecisionAccept
		result.Confidence = 0.95
		_ = p.store.Claim().UpdateStatus(ctx, claim.ID, models.ClaimVerified, claim.Confidence)
	} else {
		result.Decision = models.VerificationDecisionReject
		result.Confidence = 0.3
		_ = p.store.Claim().UpdateStatus(ctx, claim.ID, models.ClaimContradicted, models.Confidence(0))
	}

	evRecord := models.EvidenceRecord{
		BaseRecord:         models.NewBaseRecord(models.NewID()),
		ClaimID:            claim.ID,
		EvidenceSource:     models.EvidenceSourceVerification,
		ClaimContent:       fmt.Sprintf("ReQiCode verification run %s", runID),
		ObservedResult:     taskResult.VerificationStatus,
		VerificationMethod: "reqicode_run",
		Validity:           result.AssessedValidity,
		Confidence:         result.Confidence,
		Provenance: models.Provenance{SourceType: "verification"},
		ArtifactLocation:   taskResult.RunID,
	}
	_ = p.store.Evidence().CreateRecord(ctx, evRecord)
	_ = p.store.Verification().RecordResult(ctx, result)

	return result
}

func (p *VerificationPolicy) ReviseClaim(ctx context.Context, originalClaim models.Claim, revisedStatement string, evidence []EvidenceItem) (models.RevisionDecision, error) {
	revisedClaim, err := p.CreateClaim(ctx, originalClaim.Subject, revisedStatement, models.Confidence(0.6), evidence)
	if err != nil {
		return models.RevisionDecision{}, fmt.Errorf("create revised claim: %w", err)
	}

	decision := models.RevisionDecision{
		OriginalClaimID: originalClaim.ID,
		RevisedClaimID:  revisedClaim.ID,
		Reason:          fmt.Sprintf("original claim refuted, revised with new evidence"),
		Evidence:        evidenceToIDs(evidence),
		Accepted:        true,
	}

	if err := p.store.Claim().UpdateStatus(ctx, originalClaim.ID, models.ClaimContradicted, models.Confidence(0)); err != nil {
		p.logger.Warn("failed to mark original claim as contradicted", "error", err)
		decision.Accepted = false
	}

	p.logger.Info("claim revised",
		"original_claim_id", originalClaim.ID,
		"revised_claim_id", revisedClaim.ID,
	)

	return decision, nil
}

func (p *VerificationPolicy) BuildResponsePlan(claim models.Claim, verification models.VerificationResult) models.ResponsePlan {
	plan := models.ResponsePlan{
		Claim: claim,
	}

	if verification.Verified {
		plan.Output = fmt.Sprintf("Claim confirmed: %s", claim.Content)
		plan.Confidence = models.Confidence(0.95)
	} else {
		plan.Output = fmt.Sprintf("Claim could not be fully verified: %s", claim.Content)
		plan.Confidence = models.Confidence(0.3)
		if verification.Error != "" {
			plan.Uncertainty = append(plan.Uncertainty, verification.Error)
		}
	}

	return plan
}

type EvidenceItem struct {
	Description string
	Source      models.EvidenceSource
	Status      models.ValidityStatus
	URL         string
}

func evidenceToIDs(items []EvidenceItem) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.Description)
	}
	return result
}

func requiresVerification(subject string, evidence []string) bool {
	technicalSubjects := []string{
		"code", "api", "database", "architecture", "model", "implementation",
		"function", "class", "module", "test", "verify", "build",
	}
	lowerSubject := strings.ToLower(subject)
	for _, ts := range technicalSubjects {
		if strings.Contains(lowerSubject, ts) {
			return true
		}
	}
	return false
}
