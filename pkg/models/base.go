package models

import (
	"time"

	"github.com/google/uuid"
)

type StringArray []string
type StringMap map[string]string
type AnyMap map[string]interface{}

type Provenance struct {
	SourceEventIDs   []string `json:"source_event_ids,omitempty" bson:"source_event_ids,omitempty"`
	SourceMessageIDs []string `json:"source_message_ids,omitempty" bson:"source_message_ids,omitempty"`
	ProposalID       string   `json:"proposal_id,omitempty" bson:"proposal_id,omitempty"`
	PipelineRunID    string   `json:"pipeline_run_id,omitempty" bson:"pipeline_run_id,omitempty"`
	SourceType       string   `json:"source_type,omitempty" bson:"source_type,omitempty"`
}

type SourceTrust string

const (
	SourceSystemVerified   SourceTrust = "system_verified"
	SourceExternalVerified SourceTrust = "external_verified"
	SourceUserStated       SourceTrust = "user_stated"
	SourceModelInferred    SourceTrust = "model_inferred"
	SourceModelSpeculated  SourceTrust = "model_speculated"
	SourceUnknown          SourceTrust = "unknown"
)

type Confidence float64

func (c Confidence) Validate() error {
	if c < 0.0 || c > 1.0 {
		return NewValidationError("confidence", "must be between 0.0 and 1.0")
	}
	return nil
}

type Importance float64

func (i Importance) Validate() error {
	if i < 0.0 || i > 1.0 {
		return NewValidationError("importance", "must be between 0.0 and 1.0")
	}
	return nil
}

type BaseRecord struct {
	ID        string    `json:"id" bson:"_id"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

type RecordStatus string

const (
	StatusActive     RecordStatus = "active"
	StatusArchived   RecordStatus = "archived"
	StatusRejected   RecordStatus = "rejected"
	StatusSuperseded RecordStatus = "superseded"
	StatusPaused     RecordStatus = "paused"
)

type Evidence struct {
	Type      string    `json:"type" bson:"type"`
	ID        string    `json:"id,omitempty" bson:"id,omitempty"`
	Content   string    `json:"content,omitempty" bson:"content,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty" bson:"timestamp,omitempty"`
}

func NewBaseRecord(id string) BaseRecord {
	now := time.Now().UTC()
	return BaseRecord{
		ID:        id,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func NewID() string {
	return uuid.New().String()
}

func (b *BaseRecord) Touch() {
	b.UpdatedAt = time.Now().UTC()
}
