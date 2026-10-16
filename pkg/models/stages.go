package models

import (
	"encoding/json"
	"strings"
	"time"
)

type StringOrEntity []string

func (s *StringOrEntity) UnmarshalJSON(data []byte) error {
	str := strings.TrimSpace(string(data))
	if str == "null" || str == "" {
		return nil
	}

	var strSlice []string
	if err := json.Unmarshal(data, &strSlice); err == nil {
		*s = strSlice
		return nil
	}

	var objSlice []map[string]any
	if err := json.Unmarshal(data, &objSlice); err == nil {
		for _, obj := range objSlice {
			if text, ok := obj["text"].(string); ok {
				*s = append(*s, text)
			} else if name, ok := obj["name"].(string); ok {
				*s = append(*s, name)
			} else if v, ok := obj["value"].(string); ok {
				*s = append(*s, v)
			} else {
				if b, err := json.Marshal(obj); err == nil {
					*s = append(*s, string(b))
				}
			}
		}
		return nil
	}

	var singleObj map[string]any
	if err := json.Unmarshal(data, &singleObj); err == nil {
		if b, err := json.Marshal(singleObj); err == nil {
			*s = []string{string(b)}
		}
		return nil
	}

	var singleStr string
	if err := json.Unmarshal(data, &singleStr); err == nil {
		*s = []string{singleStr}
		return nil
	}

	return nil
}

func (s StringOrEntity) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string(s))
}

func (s StringOrEntity) Slice() []string {
	return []string(s)
}

type PerceptionOutput struct {
	ConversationID  string         `json:"conversation_id"`
	TurnID          string         `json:"turn_id"`
	RawInput        string         `json:"raw_input"`
	NormalizedInput string         `json:"normalized_input"`
	Language        string         `json:"language"`
	Topics          StringOrEntity `json:"topics"`
	Entities        StringOrEntity `json:"entities"`
	Intent          *string        `json:"intent,omitempty"`
	Questions       StringOrEntity `json:"questions"`
	Requests        StringOrEntity `json:"requests"`
	Signals         AnyMap         `json:"signals"`
	Timestamp       time.Time      `json:"timestamp"`
	RetrievalHinted bool           `json:"retrieval_hinted"`
	DebugPrompt     string         `json:"debug_prompt,omitempty"`
}

type RecallOutput struct {
	ConversationID   string                `json:"conversation_id"`
	TurnID           string                `json:"turn_id"`
	EpisodicMemories []MemorySearchResult  `json:"episodic_memories"`
	SemanticMemories []MemorySearchResult  `json:"semantic_memories"`
	UserInfoMemories []MemorySearchResult  `json:"user_info_memories"`
	Beliefs          []Belief              `json:"beliefs"`
	Opinions         []Opinion             `json:"opinions"`
	Preferences      []Preference          `json:"preferences"`
	Goals            []Goal                `json:"goals"`
	Hypotheses       []Hypothesis          `json:"hypotheses"`
	Uncertainties    []Uncertainty         `json:"uncertainties"`
	SelfObservations []SelfObservation     `json:"self_observations"`
	Contradictions   []Contradiction       `json:"contradictions"`
	RecentMessages   []ConversationMessage `json:"recent_messages"`
	RetrievalTimeMs  int64                 `json:"retrieval_time_ms"`
}

type ConversationMessage struct {
	ID             string    `json:"id" bson:"_id"`
	ConversationID string    `json:"conversation_id" bson:"conversation_id"`
	Title          string    `json:"title,omitempty" bson:"title,omitempty"`
	Role           string    `json:"role" bson:"role"`
	Content        string    `json:"content" bson:"content"`
	Timestamp      time.Time `json:"timestamp" bson:"timestamp"`
}

type InterpretationOutput struct {
	ConversationID   string     `json:"conversation_id"`
	TurnID           string     `json:"turn_id"`
	Intent           string     `json:"intent"`
	Goal             string     `json:"goal"`
	Context          []string   `json:"context"`
	RelevantFacts    []string   `json:"relevant_facts"`
	RelevantMemories []string   `json:"relevant_memories"`
	RelevantBeliefs  []string   `json:"relevant_beliefs"`
	Ambiguities      []string   `json:"ambiguities"`
	Assumptions      []string   `json:"assumptions"`
	Uncertainties    []string   `json:"uncertainties"`
	Observations     []string   `json:"observations"`
	Inferences       []string   `json:"inferences"`
	Confidence       Confidence `json:"confidence"`
	DebugPrompt      string     `json:"debug_prompt,omitempty"`
}

type ReflectionOutput struct {
	ConversationID    string               `json:"conversation_id"`
	TurnID            string               `json:"turn_id"`
	Observations      []string             `json:"observations"`
	LearnedFacts      []LearnedFact        `json:"learned_facts"`
	BeliefUpdates     []BeliefUpdate       `json:"belief_updates"`
	NewBeliefs        []NewBelief          `json:"new_beliefs"`
	OpinionUpdates    []OpinionUpdate      `json:"opinion_updates"`
	NewOpinions       []NewOpinion         `json:"new_opinions"`
	PreferenceUpdates []PreferenceUpdate   `json:"preference_updates"`
	GoalUpdates       []GoalUpdate         `json:"goal_updates"`
	Hypotheses        []NewHypothesis      `json:"hypotheses"`
	Uncertainties     []NewUncertainty     `json:"uncertainties"`
	SelfObservations  []NewSelfObservation `json:"self_observations"`
	Contradictions    []NewContradiction   `json:"contradictions"`
	MemoryCandidates  []MemoryCandidate    `json:"memory_candidates"`
	DiscardCandidates []string             `json:"discard_candidates"`
	Confidence        Confidence           `json:"confidence"`
	DebugPrompt       string               `json:"debug_prompt,omitempty"`
}

type StringOrStringArray []string

func (s *StringOrStringArray) UnmarshalJSON(data []byte) error {
	str := strings.TrimSpace(string(data))
	if str == "null" || str == "" {
		return nil
	}
	if str[0] == '"' {
		var single string
		if err := json.Unmarshal(data, &single); err != nil {
			return err
		}
		*s = []string{single}
		return nil
	}
	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	*s = arr
	return nil
}

func (s StringOrStringArray) MarshalJSON() ([]byte, error) {
	if len(s) == 0 {
		return json.Marshal([]string{})
	}
	return json.Marshal([]string(s))
}

func (s StringOrStringArray) Slice() []string {
	return []string(s)
}

type ProposedMutation struct {
	Reason         string              `json:"reason"`
	Evidence       StringOrStringArray `json:"evidence"`
	Confidence     Confidence          `json:"confidence"`
	SourceEventIDs []string            `json:"source_event_ids"`
}

type LearnedFact struct {
	ProposedMutation
	Content string `json:"content"`
}

type BeliefUpdate struct {
	ProposedMutation
	BeliefID      string     `json:"belief_id"`
	NewConfidence Confidence `json:"new_confidence"`
}

type NewBelief struct {
	ProposedMutation
	Subject string `json:"subject"`
	Content string `json:"content"`
}

type OpinionUpdate struct {
	ProposedMutation
	OpinionID   string     `json:"opinion_id"`
	NewStrength Confidence `json:"new_strength"`
}

type NewOpinion struct {
	ProposedMutation
	Subject string `json:"subject"`
	Content string `json:"content"`
}

type PreferenceUpdate struct {
	ProposedMutation
	PreferenceID string     `json:"preference_id"`
	NewStrength  Confidence `json:"new_strength"`
}

type GoalUpdate struct {
	ProposedMutation
	GoalID      string     `json:"goal_id"`
	NewStatus   GoalStatus `json:"new_status"`
	NewProgress float64    `json:"new_progress"`
}

type NewHypothesis struct {
	ProposedMutation
	Subject    string `json:"subject"`
	Hypothesis string `json:"hypothesis"`
}

type NewUncertainty struct {
	ProposedMutation
	Question string `json:"question"`
}

type NewSelfObservation struct {
	ProposedMutation
	Subject     string `json:"subject"`
	Observation string `json:"observation"`
}

type NewContradiction struct {
	ProposedMutation
	BeliefA string `json:"belief_a"`
	BeliefB string `json:"belief_b"`
}

type MemoryCandidate struct {
	ProposedMutation
	Type    MemoryType `json:"type"`
	Content string     `json:"content"`
	Tags    []string   `json:"tags"`
}

type ResponseOutput struct {
	ConversationID string          `json:"conversation_id"`
	TurnID         string          `json:"turn_id"`
	Response       string          `json:"response"`
	DebugPrompt    string          `json:"debug_prompt,omitempty"`
	ToolUseResults []ToolUseResult `json:"tool_use_results,omitempty"`
}

type ConsolidationOutput struct {
	ConversationID string                `json:"conversation_id"`
	TurnID         string                `json:"turn_id"`
	Actions        []ConsolidationAction `json:"actions"`
	ConsolidatedAt time.Time             `json:"consolidated_at"`
	Success        bool                  `json:"success"`
}

type ConsolidationAction struct {
	Type       string `json:"type"`
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Reason     string `json:"reason"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	Details    AnyMap `json:"details,omitempty"`
}
