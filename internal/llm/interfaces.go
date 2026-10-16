package llm

import (
	"context"
	"time"
)

type LLMRequest struct {
	Model       string         `json:"model"`
	Messages    []LLMMessage   `json:"messages"`
	Temperature float64        `json:"temperature,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Stream      bool           `json:"stream,omitempty"`
	Format      string         `json:"format,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Timeout     time.Duration  `json:"-"`
	Tools       []ToolSpec     `json:"tools,omitempty"`
	ToolChoice  string         `json:"tool_choice,omitempty"`
}

type LLMMessage struct {
	Role       string               `json:"role"`
	Content    string               `json:"content"`
	ToolCalls  []LLMMessageToolCall `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
	Name       string               `json:"name,omitempty"`
}

type LLMMessageToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolCallFunc `json:"function"`
}

type ToolCallFunc struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type ToolSpec struct {
	Type     string       `json:"type"`
	Function ToolSpecFunc `json:"function"`
}

type ToolSpecFunc struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	Strict      bool           `json:"strict,omitempty"`
}

type LLMResponse struct {
	Model      string               `json:"model"`
	Content    string               `json:"content"`
	Done       bool                 `json:"done"`
	TokenCount int                  `json:"token_count,omitempty"`
	DurationMs int64                `json:"duration_ms"`
	Provider   string               `json:"provider"`
	Raw        any                  `json:"-"`
	ToolCalls  []LLMMessageToolCall `json:"tool_calls,omitempty"`
}

type LLMProvider interface {
	Name() string
	Generate(ctx context.Context, request LLMRequest) (LLMResponse, error)
	HealthCheck(ctx context.Context) error
}

type EmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type EmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
	Model     string    `json:"model"`
}

type EmbeddingProvider interface {
	Name() string
	Embed(ctx context.Context, text string) ([]float32, error)
}
