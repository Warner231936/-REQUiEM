package models

import "time"

type PipelineConfig struct {
	SynchronousConsolidation bool          `json:"synchronous_consolidation" bson:"-"`
	Debug                    bool          `json:"debug" bson:"-"`
	ToolUse                  ToolUseConfig `json:"tool_use" bson:"-"`
	Stage0                   StageConfig   `json:"stage0" bson:"-"`
	Stage1                   StageConfig   `json:"stage1" bson:"-"`
	Stage2                   StageConfig   `json:"stage2" bson:"-"`
	Stage3                   StageConfig   `json:"stage3" bson:"-"`
	Stage4                   StageConfig   `json:"stage4" bson:"-"`
	Stage5                   StageConfig   `json:"stage5" bson:"-"`
	Stage6                   StageConfig   `json:"stage6" bson:"-"`
}

type StageConfig struct {
	Enabled  bool   `json:"enabled" bson:"-"`
	Provider string `json:"provider" bson:"-"`
	Model    string `json:"model" bson:"-"`
}

type PipelineInput struct {
	ConversationID string    `json:"conversation_id"`
	TurnID         string    `json:"turn_id"`
	UserID         string    `json:"user_id,omitempty"`
	UserMessage    string    `json:"user_message"`
	Timestamp      time.Time `json:"timestamp"`
	Metadata       AnyMap    `json:"metadata,omitempty"`
}

type PipelineOutput struct {
	ConversationID string               `json:"conversation_id"`
	TurnID         string               `json:"turn_id"`
	Response       string               `json:"response"`
	ResponsePrompt string               `json:"response_prompt,omitempty"`
	StageOutputs   []StageResult        `json:"stage_outputs,omitempty"`
	Consolidation  *ConsolidationOutput `json:"consolidation,omitempty"`
	DurationMs     int64                `json:"duration_ms"`
	CompletedAt    time.Time            `json:"completed_at"`
	Events         []PipelineEvent      `json:"events,omitempty"`
}

type StageResult struct {
	Stage       int    `json:"stage"`
	Name        string `json:"name"`
	Success     bool   `json:"success"`
	DurationMs  int64  `json:"duration_ms"`
	Error       string `json:"error,omitempty"`
	DebugPrompt string `json:"debug_prompt,omitempty"`
}

type PipelineEvent struct {
	Event          string    `json:"event"`
	Stage          int       `json:"stage"`
	ConversationID string    `json:"conversation_id"`
	TurnID         string    `json:"turn_id"`
	DurationMs     int64     `json:"duration_ms"`
	Model          string    `json:"model"`
	TokenCount     int       `json:"token_count,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
	Details        AnyMap    `json:"details,omitempty"`
}

type PipelineRun struct {
	BaseRecord
	ConversationID  string          `json:"conversation_id" bson:"conversation_id"`
	TurnID          string          `json:"turn_id" bson:"turn_id"`
	StageResults    []StageResult   `json:"stage_results" bson:"stage_results"`
	Events          []PipelineEvent `json:"events" bson:"events"`
	TotalDurationMs int64           `json:"total_duration_ms" bson:"total_duration_ms"`
	Success         bool            `json:"success" bson:"success"`
}
