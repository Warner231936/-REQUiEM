package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/pkg/models"
)

type Tool interface {
	Name() string
	Description() string
	ParameterSchema() map[string]any
	Execute(ctx context.Context, args map[string]any) (string, error)
}

type ToolRegistry struct {
	tools map[string]Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

func (r *ToolRegistry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *ToolRegistry) List() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

func (r *ToolRegistry) Has(name string) bool {
	_, ok := r.tools[name]
	return ok
}

func (r *ToolRegistry) ToToolSpecs() []llm.ToolSpec {
	specs := make([]llm.ToolSpec, 0, len(r.tools))
	for _, t := range r.tools {
		specs = append(specs, ToolToSpec(t))
	}
	return specs
}

func (r *ToolRegistry) Execute(ctx context.Context, call models.ToolCall) models.ToolResult {
	tool, ok := r.tools[call.Name]
	if !ok {
		return models.ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Sprintf("tool not found: %s", call.Name),
		}
	}

	result, err := tool.Execute(ctx, call.Arguments)
	if err != nil {
		return models.ToolResult{
			ToolCallID: call.ID,
			Content:    result,
			Error:      err.Error(),
		}
	}

	return models.ToolResult{
		ToolCallID: call.ID,
		Content:    result,
	}
}

func (r *ToolRegistry) IsEmpty() bool {
	return len(r.tools) == 0
}

func ToolToSpec(t Tool) llm.ToolSpec {
	return llm.ToolSpec{
		Type: "function",
		Function: llm.ToolSpecFunc{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.ParameterSchema(),
			Strict:      true,
		},
	}
}

func ParseArguments(rawArgs string) (map[string]any, error) {
	if rawArgs == "" {
		return map[string]any{}, nil
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		return nil, fmt.Errorf("parse tool arguments: %w", err)
	}
	return args, nil
}

func ParseArgumentsMap(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	return args
}
