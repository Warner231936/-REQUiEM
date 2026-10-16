package storage

import (
	"context"
	"time"

	"github.com/requiem/cognitive/pkg/models"
)

type MemoryStore interface {
	Create(ctx context.Context, memory models.Memory) error
	Get(ctx context.Context, id string) (models.Memory, error)
	Search(ctx context.Context, query models.MemoryQuery) ([]models.MemorySearchResult, error)
	Update(ctx context.Context, memory models.Memory) error
	Delete(ctx context.Context, id string) error
	GetByConversation(ctx context.Context, conversationID string, limit int) ([]models.Memory, error)
}

type MindStore interface {
	GetBeliefs(ctx context.Context, query models.MindQuery) ([]models.Belief, error)
	CreateBelief(ctx context.Context, belief models.Belief) error
	UpdateBelief(ctx context.Context, belief models.Belief) error
	GetOpinions(ctx context.Context, query models.MindQuery) ([]models.Opinion, error)
	CreateOpinion(ctx context.Context, opinion models.Opinion) error
	UpdateOpinion(ctx context.Context, opinion models.Opinion) error
	GetPreferences(ctx context.Context, query models.MindQuery) ([]models.Preference, error)
	CreatePreference(ctx context.Context, preference models.Preference) error
	UpdatePreference(ctx context.Context, preference models.Preference) error
	GetValues(ctx context.Context, query models.MindQuery) ([]models.Value, error)
	CreateValue(ctx context.Context, value models.Value) error
	GetGoals(ctx context.Context, query models.MindQuery) ([]models.Goal, error)
	CreateGoal(ctx context.Context, goal models.Goal) error
	UpdateGoal(ctx context.Context, goal models.Goal) error
	GetHypotheses(ctx context.Context, query models.MindQuery) ([]models.Hypothesis, error)
	CreateHypothesis(ctx context.Context, hypothesis models.Hypothesis) error
	UpdateHypothesis(ctx context.Context, hypothesis models.Hypothesis) error
	GetUncertainties(ctx context.Context, query models.MindQuery) ([]models.Uncertainty, error)
	CreateUncertainty(ctx context.Context, uncertainty models.Uncertainty) error
	UpdateUncertainty(ctx context.Context, uncertainty models.Uncertainty) error
	GetSelfObservations(ctx context.Context, query models.MindQuery) ([]models.SelfObservation, error)
	CreateSelfObservation(ctx context.Context, observation models.SelfObservation) error
	UpdateSelfObservation(ctx context.Context, observation models.SelfObservation) error
	GetContradictions(ctx context.Context, query models.MindQuery) ([]models.Contradiction, error)
	CreateContradiction(ctx context.Context, contradiction models.Contradiction) error
	UpdateContradiction(ctx context.Context, contradiction models.Contradiction) error
}

type ConversationStore interface {
	CreateConversation(ctx context.Context, conversation models.ConversationMessage) error
	AddMessage(ctx context.Context, conversationID string, message models.ConversationMessage) error
	GetMessages(ctx context.Context, conversationID string, limit int) ([]models.ConversationMessage, error)
	GetMessagesPaginated(ctx context.Context, conversationID string, limit, offset int) ([]models.ConversationMessage, error)
	ListConversations(ctx context.Context, limit, offset int) ([]ConversationSummary, error)
	DeleteAll(ctx context.Context, conversationID string) (int64, error)
}

type ConversationSummary struct {
	ConversationID string    `json:"conversation_id" bson:"conversation_id"`
	Title          string    `json:"title" bson:"title"`
	MessageCount   int64     `json:"message_count" bson:"message_count"`
	LastActivity   time.Time `json:"last_activity" bson:"last_activity"`
}

type EventStore interface {
	RecordEvent(ctx context.Context, event models.PipelineEvent) error
	GetEvents(ctx context.Context, pipelineRunID string) ([]models.PipelineEvent, error)
}

type AuditStore interface {
	RecordHistory(ctx context.Context, record AuditRecord) error
	GetHistory(ctx context.Context, entityType, entityID string) ([]AuditRecord, error)
	GetProvenance(ctx context.Context, entityID string) ([]ProvenanceStep, error)
}

type EvidenceStore interface {
	CreateRecord(ctx context.Context, record models.EvidenceRecord) error
	GetByClaim(ctx context.Context, claimID string) ([]models.EvidenceRecord, error)
	GetByID(ctx context.Context, id string) (models.EvidenceRecord, error)
	UpdateValidity(ctx context.Context, id string, validity models.ValidityStatus) error
	Delete(ctx context.Context, id string) error
}

type ClaimStore interface {
	CreateClaim(ctx context.Context, claim models.Claim) error
	GetClaim(ctx context.Context, id string) (models.Claim, error)
	UpdateClaim(ctx context.Context, claim models.Claim) error
	GetBySubject(ctx context.Context, subject string, limit int) ([]models.Claim, error)
	UpdateStatus(ctx context.Context, id string, status models.ClaimStatus, confidence models.Confidence) error
}

type VerificationStore interface {
	RecordPlan(ctx context.Context, plan models.VerificationPlan) error
	RecordResult(ctx context.Context, result models.VerificationResult) error
	GetPlan(ctx context.Context, claimID string) (models.VerificationPlan, error)
	GetResult(ctx context.Context, claimID string) (models.VerificationResult, error)
}

type AuditRecord struct {
	EntityType     string        `json:"entity_type" bson:"entity_type"`
	EntityID       string        `json:"entity_id" bson:"entity_id"`
	Action         string        `json:"action" bson:"action"`
	PreviousState  models.AnyMap `json:"previous_state,omitempty" bson:"previous_state,omitempty"`
	NewState       models.AnyMap `json:"new_state,omitempty" bson:"new_state,omitempty"`
	Reason         string        `json:"reason" bson:"reason"`
	SourceEventIDs []string      `json:"source_event_ids,omitempty" bson:"source_event_ids,omitempty"`
	Timestamp      string        `json:"timestamp" bson:"timestamp"`
}

type ProvenanceStep struct {
	Step      string `json:"step" bson:"step"`
	EntityID  string `json:"entity_id" bson:"entity_id"`
	Action    string `json:"action" bson:"action"`
	Timestamp string `json:"timestamp" bson:"timestamp"`
	Reason    string `json:"reason" bson:"reason"`
}

type PipelineRunStore interface {
	RecordRun(ctx context.Context, run models.PipelineRun) error
	GetRun(ctx context.Context, id string) (models.PipelineRun, error)
}

type TransactionalStore interface {
	StartTransaction(ctx context.Context) (context.Context, error)
	CommitTransaction(ctx context.Context) error
	AbortTransaction(ctx context.Context) error
	WithRetry(ctx context.Context, fn func(ctx context.Context) error) error
}

type AuthStore interface {
	CreateUser(ctx context.Context, user models.User) error
	GetUser(ctx context.Context, userID string) (models.User, error)
	GetUserByEmail(ctx context.Context, email string) (models.User, error)
	ListUsers(ctx context.Context) ([]models.User, error)
	UpdateUser(ctx context.Context, user models.User) error
	DeleteUser(ctx context.Context, userID string) error

	CreateVerificationToken(ctx context.Context, token models.VerificationToken) error
	GetVerificationToken(ctx context.Context, token string) (models.VerificationToken, error)
	DeleteVerificationToken(ctx context.Context, token string) error

	CreatePasswordResetToken(ctx context.Context, token models.PasswordResetToken) error
	GetPasswordResetToken(ctx context.Context, token string) (models.PasswordResetToken, error)
	DeletePasswordResetToken(ctx context.Context, token string) error
}

type Store interface {
	Memory() MemoryStore
	Mind() MindStore
	Conversation() ConversationStore
	Event() EventStore
	Audit() AuditStore
	PipelineRun() PipelineRunStore
	Auth() AuthStore
	Evidence() EvidenceStore
	Claim() ClaimStore
	Verification() VerificationStore
	TransactionalStore
}
