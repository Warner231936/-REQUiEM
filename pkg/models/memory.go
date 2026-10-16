package models

import "time"

type MemoryType string

const (
	MemoryTypeEpisodic MemoryType = "episodic"
	MemoryTypeSemantic MemoryType = "semantic"
)

type Memory struct {
	BaseRecord
	Type               MemoryType `json:"type" bson:"type"`
	UserID             string     `json:"user_id,omitempty" bson:"user_id,omitempty"`
	Content            string     `json:"content" bson:"content"`
	ConversationID     string     `json:"conversation_id,omitempty" bson:"conversation_id,omitempty"`
	TurnID             string     `json:"turn_id,omitempty" bson:"turn_id,omitempty"`
	Importance         Importance `json:"importance" bson:"importance"`
	Confidence         Confidence `json:"confidence" bson:"confidence"`
	Source             Provenance `json:"source" bson:"source"`
	Tags               []string   `json:"tags,omitempty" bson:"tags,omitempty"`
	LastAccessedAt     *time.Time `json:"last_accessed_at,omitempty" bson:"last_accessed_at,omitempty"`
	EffectiveRelevance float64    `json:"effective_relevance,omitempty" bson:"effective_relevance,omitempty"`
	Embedding          []float32  `json:"embedding,omitempty" bson:"embedding,omitempty"`
}

type MemoryQuery struct {
	ConversationID string
	UserID         string
	Tags           []string
	Type           MemoryType
	Limit          int
	MinImportance  Importance
	SearchText     string
	MinSimilarity  float64
}

type MemorySearchResult struct {
	Memory     Memory  `json:"memory" bson:"-"`
	Similarity float64 `json:"similarity"`
	Relevance  float64 `json:"relevance"`
}

func (m *Memory) CalculateEffectiveRelevance(now time.Time) float64 {
	recencyFactor := calculateRecencyFactor(m.CreatedAt, now)
	contextualRelevance := 1.0
	m.EffectiveRelevance = float64(m.Importance) * recencyFactor * float64(m.Confidence) * contextualRelevance
	return m.EffectiveRelevance
}

func calculateRecencyFactor(createdAt, now time.Time) float64 {
	hoursOld := now.Sub(createdAt).Hours()
	if hoursOld < 1 {
		return 1.0
	}
	if hoursOld < 24 {
		return 0.95
	}
	if hoursOld < 168 {
		return 0.85
	}
	if hoursOld < 720 {
		return 0.70
	}
	return 0.50
}
