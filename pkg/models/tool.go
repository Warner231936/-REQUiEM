package models

type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	Strict      bool           `json:"strict,omitempty"`
}

type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	Error      string `json:"error,omitempty"`
}

type LLMToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type LLMToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	Error      string `json:"error,omitempty"`
}

type ToolCallTrace struct {
	ToolCallID string         `json:"tool_call_id"`
	ToolName   string         `json:"tool_name"`
	Arguments  map[string]any `json:"arguments"`
	Result     string         `json:"result"`
	Error      string         `json:"error,omitempty"`
	Success    bool           `json:"success"`
}

type ToolUseResult struct {
	Trace        []ToolCallTrace `json:"trace"`
	SolutionText string          `json:"solution_text,omitempty"`
}

type ToolUseConfig struct {
	Enabled       bool `json:"enabled" bson:"-"`
	MaxIterations int  `json:"max_iterations" bson:"-"`
}
