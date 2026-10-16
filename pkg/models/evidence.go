package models

import (
	"time"
)

type ClaimStatus string

const (
	ClaimProposed   ClaimStatus = "proposed"
	ClaimSupported  ClaimStatus = "supported"
	ClaimVerified   ClaimStatus = "verified"
	ClaimContradicted ClaimStatus = "contradicted"
	ClaimUncertain  ClaimStatus = "uncertain"
	ClaimSuperseded ClaimStatus = "superseded"
)

type EvidenceSource string

const (
	EvidenceSourceTestPass       EvidenceSource = "test_pass"
	EvidenceSourceTestFailure    EvidenceSource = "test_failure"
	EvidenceSourceBuildSuccess   EvidenceSource = "build_success"
	EvidenceSourceBuildFailure   EvidenceSource = "build_failure"
	EvidenceSourceStaticAnalysis EvidenceSource = "static_analysis"
	EvidenceSourceCodeReview     EvidenceSource = "code_review"
	EvidenceSourceExternal       EvidenceSource = "external"
	EvidenceSourceObservation    EvidenceSource = "observation"
	EvidenceSourceReqiCodeRun    EvidenceSource = "reqicode_run"
	EvidenceSourcePolicy         EvidenceSource = "policy"
	EvidenceSourceVerification   EvidenceSource = "verification"
)

type ValidityStatus string

const (
	ValidityConfirmed    ValidityStatus = "confirmed"
	ValidityRefuted      ValidityStatus = "refuted"
	ValidityAmbiguous    ValidityStatus = "ambiguous"
	ValidityPending      ValidityStatus = "pending"
	ValidityUnverified   ValidityStatus = "unverified"
	ValidityInsufficient ValidityStatus = "insufficient"
)

type EvidenceRecord struct {
	BaseRecord
	ClaimID          string          `json:"claim_id" bson:"claim_id"`
	Subject          string          `json:"subject" bson:"subject"`
	EvidenceSource   EvidenceSource  `json:"evidence_source" bson:"evidence_source"`
	ClaimContent     string          `json:"claim_content" bson:"claim_content"`
	ObservedResult   string          `json:"observed_result" bson:"observed_result"`
	VerificationMethod string        `json:"verification_method" bson:"verification_method"`
	Validity         ValidityStatus  `json:"validity" bson:"validity"`
	Limitations      string          `json:"limitations,omitempty" bson:"limitations,omitempty"`
	Confidence       Confidence      `json:"confidence" bson:"confidence"`
	Provenance       Provenance      `json:"provenance" bson:"provenance"`
	ArtifactLocation string          `json:"artifact_location,omitempty" bson:"artifact_location,omitempty"`
	Metadata         AnyMap          `json:"metadata,omitempty" bson:"metadata,omitempty"`
}

type Claim struct {
	BaseRecord
	Subject          string         `json:"subject" bson:"subject"`
	Content          string         `json:"content" bson:"content"`
	Status           ClaimStatus    `json:"status" bson:"status"`
	Confidence       Confidence     `json:"confidence" bson:"confidence"`
	Inferred         bool           `json:"inferred" bson:"inferred"`
	Assumptions      []string       `json:"assumptions,omitempty" bson:"assumptions,omitempty"`
	DisconfirmingEvidence string    `json:"disconfirming_evidence,omitempty" bson:"disconfirming_evidence,omitempty"`
	EvidenceIDs      []string       `json:"evidence_ids,omitempty" bson:"evidence_ids,omitempty"`
	PipelineRunID    string         `json:"pipeline_run_id,omitempty" bson:"pipeline_run_id,omitempty"`
	Provenance       Provenance     `json:"provenance" bson:"provenance"`
}

func NewClaim(subject, content string, confidence Confidence, provenance Provenance) *Claim {
	now := time.Now().UTC()
	return &Claim{
		BaseRecord: BaseRecord{
			ID:        NewID(),
			CreatedAt: now,
			UpdatedAt: now,
		},
		Subject:    subject,
		Content:    content,
		Status:     ClaimProposed,
		Confidence: confidence,
		Provenance: provenance,
	}
}

func (c *Claim) AddEvidence(id string) {
	c.EvidenceIDs = append(c.EvidenceIDs, id)
	c.UpdatedAt = time.Now().UTC()
}

func (c *Claim) UpdateStatus(status ClaimStatus, confidence Confidence) {
	c.Status = status
	c.Confidence = confidence
	c.UpdatedAt = time.Now().UTC()
}

type TaskRequest struct {
	TaskID             string   `json:"task_id"`
	Objective          string   `json:"objective"`
	Rationale          string   `json:"rationale,omitempty"`
	Repository         string   `json:"repository,omitempty"`
	WorkspacePath      string   `json:"workspace_path,omitempty"`
	RelevantFiles      []string `json:"relevant_files,omitempty"`
	ContextualEvidence string   `json:"contextual_evidence,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	TestRequirements   *TaskTestRequirements `json:"test_requirements,omitempty"`
	Budget             *TaskBudget           `json:"budget,omitempty"`
	Permissions        *TaskPermissions      `json:"permissions,omitempty"`
}

type TaskTestRequirements struct {
	UnitTests     bool `json:"unit_tests"`
	Integration   bool `json:"integration"`
	Coverage      bool `json:"coverage"`
	Benchmark     bool `json:"benchmark"`
	RaceDetection bool `json:"race_detection"`
}

type TaskBudget struct {
	MaxTokens    int `json:"max_tokens,omitempty"`
	TimeLimitSec int `json:"time_limit_sec,omitempty"`
	MaxRetries   int `json:"max_retries,omitempty"`
}

type TaskPermissions struct {
	AllowFileSystemWrite bool     `json:"allow_fs_write"`
	AllowNetwork         bool     `json:"allow_network"`
	AllowedPaths         []string `json:"allowed_paths,omitempty"`
	RestrictedCommands   []string `json:"restricted_commands,omitempty"`
}

type TaskResult struct {
	RunID                    string               `json:"run_id,omitempty"`
	TaskID                   string               `json:"task_id"`
	Status                   TaskLifecycleStatus  `json:"status"`
	Objective                string               `json:"objective,omitempty"`
	FilesChanged             []TaskFileChange     `json:"files_changed,omitempty"`
	CommandsExecuted         []TaskCommandExec    `json:"commands_executed,omitempty"`
	TestResults              []TaskTestResult     `json:"test_results,omitempty"`
	BenchmarkResults         []map[string]any     `json:"benchmark_results,omitempty"`
	VerificationStatus       string               `json:"verification_status"`
	ErrorSummary             string               `json:"error_summary,omitempty"`
	KnownLimitations         string               `json:"known_limitations,omitempty"`
	ArtifactLocations        []string             `json:"artifact_locations,omitempty"`
	OutstandingUncertainties []string             `json:"outstanding_uncertainties,omitempty"`
	StartedAt                time.Time            `json:"started_at,omitempty"`
	CompletedAt              time.Time            `json:"completed_at,omitempty"`
	DurationMs               int64                `json:"duration_ms"`
	ChainValid               bool                 `json:"chain_valid"`
}

type TaskLifecycleStatus string

const (
	TaskStatusSubmitted  TaskLifecycleStatus = "SUBMITTED"
	TaskStatusApproved   TaskLifecycleStatus = "APPROVED"
	TaskStatusRunning    TaskLifecycleStatus = "RUNNING"
	TaskStatusVerified   TaskLifecycleStatus = "VERIFIED"
	TaskStatusFailed     TaskLifecycleStatus = "FAILED"
	TaskStatusBlocked    TaskLifecycleStatus = "BLOCKED"
	TaskStatusReviewed   TaskLifecycleStatus = "REVIEWED"
	TaskStatusClosed     TaskLifecycleStatus = "CLOSED"
	TaskStatusCanceled   TaskLifecycleStatus = "CANCELED"
	TaskStatusCompleted  TaskLifecycleStatus = "COMPLETED"
)

type TaskFileChange struct {
	Path      string `json:"path"`
	Operation string `json:"operation"`
}

type TaskCommandExec struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Duration int64  `json:"duration_ms"`
}

type TaskTestResult struct {
	Command      string `json:"command"`
	Status       string `json:"status"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	DurationMs   int64  `json:"duration_ms"`
	FailureClass string `json:"failure_class,omitempty"`
}

type VerificationPlan struct {
	ClaimID             string                 `json:"claim_id"`
	Objective           string                 `json:"objective,omitempty"`
	Methods             []VerificationMethod   `json:"methods"`
	Accept              []VerificationMethod   `json:"accept,omitempty"`
	AcceptanceCriteria  []string               `json:"acceptance_criteria,omitempty"`
	EstimatedCost       CostEstimate           `json:"estimated_cost"`
	EstimatedLatency    time.Duration          `json:"estimated_latency"`
	CheckerReliability  float64                `json:"checker_reliability"`
	Reversible          bool                   `json:"reversible"`
	Decision            VerificationDecision   `json:"decision"`
	Rationale           string                 `json:"rationale"`
	ClaimedVerifications []string              `json:"claimed_verifications,omitempty"`
}

type VerificationMethod string

const (
	MethodDeterministicCalc VerificationMethod = "deterministic_calc"
	MethodSchemaValidation   VerificationMethod = "schema_validation"
	MethodUnitTests         VerificationMethod = "unit_tests"
	MethodIntegrationTests  VerificationMethod = "integration_tests"
	MethodStaticAnalysis    VerificationMethod = "static_analysis"
	MethodCompilation       VerificationMethod = "compilation"
	MethodRepoInspection    VerificationMethod = "repo_inspection"
	MethodIndependentImpl   VerificationMethod = "independent_impl"
	MethodAbstain           VerificationMethod = "abstain"
)

type VerifiedMethod struct {
	Method     string             `json:"method"`
	Target     string             `json:"target"`
	ClaimedOk  bool               `json:"claimed_ok"`
	Confidence Confidence          `json:"confidence"`
}

type ClaimEvidence struct {
	Description string           `json:"description"`
	Source      EvidenceSource   `json:"source"`
	ClaimRef    string           `json:"claim_ref"`
}

type VerificationDecision string

const (
	DecisionAcceptProvisional    VerificationDecision = "accept_provisional"
	DecisionRequestClarification VerificationDecision = "request_clarification"
	DecisionDeterministicCheck   VerificationDecision = "deterministic_check"
	DecisionRunTests             VerificationDecision = "run_tests"
	DecisionBroadVerification    VerificationDecision = "broad_verification"
	DecisionSeekIndependent      VerificationDecision = "seek_independent"
	DecisionAbstain              VerificationDecision = "abstain"

	VerificationDecisionNoOp     = DecisionAbstain
	VerificationDecisionAccept   = DecisionAcceptProvisional
	VerificationDecisionVerify   = DecisionBroadVerification
	VerificationDecisionReject   = DecisionSeekIndependent
)

type CostEstimate struct {
	TokenCost  int64 `json:"token_cost"`
	TimeCostMs int64 `json:"time_cost_ms"`
	Monetary   float64 `json:"monetary,omitempty"`
}

type VerificationResult struct {
	ClaimID         string            `json:"claim_id"`
	ClaimSubject    string            `json:"claim_subject,omitempty"`
	Plan            VerificationPlan  `json:"plan"`
	Verified        bool              `json:"verified"`
	Refuted         bool              `json:"refuted"`
	Abstained       bool              `json:"abstained"`
	Confidence      Confidence        `json:"confidence"`
	Evidence        []EvidenceRecord  `json:"evidence,omitempty"`
	EvidenceIDs     []string          `json:"evidence_ids"`
	CheckerCalls    int               `json:"checker_calls"`
	Latency         time.Duration     `json:"latency"`
	Cost            CostEstimate      `json:"cost"`
	Error           string            `json:"error,omitempty"`
	Decision        VerificationDecision `json:"decision"`
	RunID           string            `json:"run_id,omitempty"`
	TaskResult      *TaskResult       `json:"task_result,omitempty"`
	AssessmentOK    bool              `json:"assessment_ok"`
	AssessedValidity ValidityStatus   `json:"assessed_validity"`
	CheckedAt       time.Time         `json:"checked_at"`
}

type RevisionDecision struct {
	OriginalClaimID string   `json:"original_claim_id"`
	RevisedClaimID  string   `json:"revised_claim_id"`
	Reason          string   `json:"reason"`
	Evidence        []string `json:"evidence"`
	Accepted        bool     `json:"accepted"`
	Reverted        bool     `json:"reverted"`
}

type ResponsePlan struct {
	Claim      Claim          `json:"claim"`
	Verification *VerificationResult `json:"verification,omitempty"`
	Output     string         `json:"output"`
	Confidence Confidence     `json:"confidence"`
	Uncertainty []string      `json:"uncertainty,omitempty"`
}
