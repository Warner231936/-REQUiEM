package models

import "time"

type BeliefStatus string

const (
	BeliefStatusActive     BeliefStatus = "active"
	BeliefStatusArchived   BeliefStatus = "archived"
	BeliefStatusChallenged BeliefStatus = "challenged"
	BeliefStatusSuperseded BeliefStatus = "superseded"
)

type Belief struct {
	BaseRecord
	Subject          string       `json:"subject" bson:"subject"`
	Content          string       `json:"content" bson:"content"`
	Confidence       Confidence   `json:"confidence" bson:"confidence"`
	Importance       Importance   `json:"importance" bson:"importance"`
	Status           BeliefStatus `json:"status" bson:"status"`
	Evidence         []Evidence   `json:"evidence,omitempty" bson:"evidence,omitempty"`
	SourceCount      int          `json:"source_count" bson:"source_count"`
	LastConfirmedAt  *Timestamp   `json:"last_confirmed_at,omitempty" bson:"last_confirmed_at,omitempty"`
	LastChallengedAt *Timestamp   `json:"last_challenged_at,omitempty" bson:"last_challenged_at,omitempty"`
	SourceType       SourceTrust  `json:"source_type" bson:"source_type"`
}

type Opinion struct {
	BaseRecord
	Subject    string       `json:"subject" bson:"subject"`
	Content    string       `json:"content" bson:"content"`
	Strength   Confidence   `json:"strength" bson:"strength"`
	Confidence Confidence   `json:"confidence" bson:"confidence"`
	Context    []string     `json:"context,omitempty" bson:"context,omitempty"`
	Evidence   []Evidence   `json:"evidence,omitempty" bson:"evidence,omitempty"`
	Status     RecordStatus `json:"status" bson:"status"`
	SourceType SourceTrust  `json:"source_type" bson:"source_type"`
}

type PreferenceScope string

const (
	PreferenceScopeUser   PreferenceScope = "user"
	PreferenceScopeSystem PreferenceScope = "system"
)

type Preference struct {
	BaseRecord
	Subject    string          `json:"subject" bson:"subject"`
	Content    string          `json:"content" bson:"content"`
	Strength   Confidence      `json:"strength" bson:"strength"`
	Scope      PreferenceScope `json:"scope" bson:"scope"`
	Status     RecordStatus    `json:"status" bson:"status"`
	SourceType SourceTrust     `json:"source_type" bson:"source_type"`
}

type Value struct {
	BaseRecord
	Name       string       `json:"name" bson:"name"`
	Content    string       `json:"content" bson:"content"`
	Importance Importance   `json:"importance" bson:"importance"`
	Status     RecordStatus `json:"status" bson:"status"`
}

type GoalStatus string

const (
	GoalStatusActive     GoalStatus = "active"
	GoalStatusPaused     GoalStatus = "paused"
	GoalStatusCompleted  GoalStatus = "completed"
	GoalStatusAbandoned  GoalStatus = "abandoned"
	GoalStatusSuperseded GoalStatus = "superseded"
)

type Goal struct {
	BaseRecord
	Goal         string     `json:"goal" bson:"goal"`
	Priority     int        `json:"priority" bson:"priority"`
	Status       GoalStatus `json:"status" bson:"status"`
	Progress     float64    `json:"progress" bson:"progress"`
	Deadline     *Timestamp `json:"deadline,omitempty" bson:"deadline,omitempty"`
	Dependencies []string   `json:"dependencies,omitempty" bson:"dependencies,omitempty"`
}

type HypothesisStatus string

const (
	HypothesisStatusUntested  HypothesisStatus = "untested"
	HypothesisStatusSupported HypothesisStatus = "supported"
	HypothesisStatusWeakened  HypothesisStatus = "weakened"
	HypothesisStatusRejected  HypothesisStatus = "rejected"
	HypothesisStatusConfirmed HypothesisStatus = "confirmed"
)

type Hypothesis struct {
	BaseRecord
	Subject               string           `json:"subject" bson:"subject"`
	Hypothesis            string           `json:"hypothesis" bson:"hypothesis"`
	Confidence            Confidence       `json:"confidence" bson:"confidence"`
	SupportingEvidence    []Evidence       `json:"supporting_evidence,omitempty" bson:"supporting_evidence,omitempty"`
	ContradictingEvidence []Evidence       `json:"contradicting_evidence,omitempty" bson:"contradicting_evidence,omitempty"`
	Status                HypothesisStatus `json:"status" bson:"status"`
}

type UncertaintyStatus string

const (
	UncertaintyStatusUnknown       UncertaintyStatus = "unknown"
	UncertaintyStatusUncertain     UncertaintyStatus = "uncertain"
	UncertaintyStatusProbable      UncertaintyStatus = "probable"
	UncertaintyStatusUnlikely      UncertaintyStatus = "unlikely"
	UncertaintyStatusContradictory UncertaintyStatus = "contradictory"
	UncertaintyStatusResolved      UncertaintyStatus = "resolved"
)

type Uncertainty struct {
	BaseRecord
	Question               string             `json:"question" bson:"question"`
	PossibleAnswers        []string           `json:"possible_answers,omitempty" bson:"possible_answers,omitempty"`
	ConfidenceDistribution map[string]float64 `json:"confidence_distribution,omitempty" bson:"confidence_distribution,omitempty"`
	Evidence               []Evidence         `json:"evidence,omitempty" bson:"evidence,omitempty"`
	Status                 UncertaintyStatus  `json:"status" bson:"status"`
}

type SelfObservation struct {
	BaseRecord
	Subject       string       `json:"subject" bson:"subject"`
	Observation   string       `json:"observation" bson:"observation"`
	Confidence    Confidence   `json:"confidence" bson:"confidence"`
	EvidenceCount int          `json:"evidence_count" bson:"evidence_count"`
	Status        RecordStatus `json:"status" bson:"status"`
}

type ContradictionStatus string

const (
	ContradictionStatusUnresolved     ContradictionStatus = "unresolved"
	ContradictionStatusContextualized ContradictionStatus = "contextualized"
	ContradictionStatusAPreferred     ContradictionStatus = "belief_a_preferred"
	ContradictionStatusBPreferred     ContradictionStatus = "belief_b_preferred"
	ContradictionStatusMerged         ContradictionStatus = "merged"
	ContradictionStatusObsoleteA      ContradictionStatus = "obsolete_a"
	ContradictionStatusObsoleteB      ContradictionStatus = "obsolete_b"
)

type Contradiction struct {
	BaseRecord
	BeliefA     string              `json:"belief_a" bson:"belief_a"`
	BeliefB     string              `json:"belief_b" bson:"belief_b"`
	ContextA    string              `json:"context_a,omitempty" bson:"context_a,omitempty"`
	ContextB    string              `json:"context_b,omitempty" bson:"context_b,omitempty"`
	ConfidenceA Confidence          `json:"confidence_a" bson:"confidence_a"`
	ConfidenceB Confidence          `json:"confidence_b" bson:"confidence_b"`
	Status      ContradictionStatus `json:"status" bson:"status"`
	Resolution  string              `json:"resolution,omitempty" bson:"resolution,omitempty"`
}

type MindQuery struct {
	IDs           []string
	Subjects      []string
	Status        RecordStatus
	MinConfidence Confidence
	Limit         int
	SourceType    SourceTrust
}

type Timestamp struct {
	time.Time `json:"-" bson:"-"`
}

func (t *Timestamp) IsZero() bool {
	return t.Time.IsZero()
}
