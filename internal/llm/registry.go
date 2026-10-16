package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

type ProviderRegistry struct {
	providers map[string]LLMProvider
	embedders map[string]EmbeddingProvider
	mu        sync.RWMutex
	logger    *slog.Logger
}

func NewProviderRegistry(logger *slog.Logger) *ProviderRegistry {
	if logger == nil {
		logger = slog.Default()
	}
	return &ProviderRegistry{
		providers: make(map[string]LLMProvider),
		embedders: make(map[string]EmbeddingProvider),
		logger:    logger,
	}
}

func (r *ProviderRegistry) Register(name string, provider LLMProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = provider
	r.logger.Info("registered LLM provider", "name", name)
}

func (r *ProviderRegistry) RegisterEmbedder(name string, provider EmbeddingProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.embedders[name] = provider
	r.logger.Info("registered embedding provider", "name", name)
}

func (r *ProviderRegistry) Get(name string) (LLMProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("LLM provider not found: %s", name)
	}
	return provider, nil
}

func (r *ProviderRegistry) GetEmbedder(name string) (EmbeddingProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.embedders[name]
	if !ok {
		return nil, fmt.Errorf("embedding provider not found: %s", name)
	}
	return provider, nil
}

func (r *ProviderRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.providers[name]
	return ok
}

func (r *ProviderRegistry) HasEmbedder(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.embedders[name]
	return ok
}

type MockProvider struct {
	Responses           map[string]string
	Sequential          []string
	SequentialToolCalls []MockToolCallResponse
	CallCount           int
	seqIndex            int
	tcSeqIndex          int
	mu                  sync.Mutex
}

type MockToolCallResponse struct {
	ToolCalls []LLMMessageToolCall
	Content   string
}

func NewMockProvider() *MockProvider {
	return &MockProvider{
		Responses:           make(map[string]string),
		Sequential:          make([]string, 0),
		SequentialToolCalls: make([]MockToolCallResponse, 0),
	}
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) Generate(ctx context.Context, request LLMRequest) (LLMResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallCount++

	if m.tcSeqIndex < len(m.SequentialToolCalls) {
		resp := m.SequentialToolCalls[m.tcSeqIndex]
		m.tcSeqIndex++
		result := LLMResponse{
			Model:      request.Model,
			Content:    resp.Content,
			Done:       true,
			Provider:   "mock",
			DurationMs: 1,
		}
		if len(resp.ToolCalls) > 0 {
			result.ToolCalls = resp.ToolCalls
		}
		return result, nil
	}

	lastMsg := ""
	if len(request.Messages) > 0 {
		lastMsg = request.Messages[len(request.Messages)-1].Content
	}

	if m.seqIndex < len(m.Sequential) {
		resp := m.Sequential[m.seqIndex]
		m.seqIndex++
		return LLMResponse{
			Model:      request.Model,
			Content:    resp,
			Done:       true,
			Provider:   "mock",
			DurationMs: 1,
		}, nil
	}

	response, ok := m.Responses[lastMsg]
	if !ok {
		response = `{"intent": "unknown", "goal": "respond to user", "confidence": 0.5}`
	}

	return LLMResponse{
		Model:      request.Model,
		Content:    response,
		Done:       true,
		Provider:   "mock",
		DurationMs: 1,
	}, nil
}

func (m *MockProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockProvider) SetResponse(prompt, response string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Responses[prompt] = response
}

func (m *MockProvider) SetSequentialResponses(responses []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Sequential = responses
	m.seqIndex = 0
}

func (m *MockProvider) SetSequentialToolCallResponses(responses []MockToolCallResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SequentialToolCalls = responses
	m.tcSeqIndex = 0
}

func ParseJSONResponse(content string, target any) error {
	cleaned := cleanJSONOutput(content)
	if err := json.Unmarshal([]byte(cleaned), target); err != nil {
		return fmt.Errorf("parse JSON response: %w", err)
	}
	return nil
}

func cleanJSONOutput(content string) string {
	content = trimCodeFence(content)
	return content
}

func trimCodeFence(content string) string {
	if len(content) > 6 && content[:3] == "```" {
		endIdx := len(content) - 3
		for i := len(content) - 3; i >= 3; i-- {
			if content[i:i+3] == "```" {
				endIdx = i
				break
			}
		}
		content = content[3:endIdx]
		if len(content) > 0 && content[0] == '\n' {
			content = content[1:]
		}
		if len(content) > 4 && (content[:5] == "json\n" || content[:4] == "json") {
			if content[0] == 'j' {
				for i := 0; i < len(content); i++ {
					if content[i] == '\n' {
						content = content[i+1:]
						break
					}
				}
			}
		}
	}
	return content
}

func ParseToolUseResponse(content string) []LLMMessageToolCall {
	cleaned := cleanJSONOutput(content)
	if cleaned == "" {
		return nil
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil
	}

	if tc, ok := raw["tool_calls"]; ok {
		if arr, ok := tc.([]any); ok {
			var result []LLMMessageToolCall
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					name := ""
					if fn, ok := m["function"].(map[string]any); ok {
						if n, ok := fn["name"].(string); ok {
							name = n
						}
						if args, ok := fn["arguments"].(map[string]any); ok {
							result = append(result, LLMMessageToolCall{
								ID:   idToStr(m["id"]),
								Type: "function",
								Function: ToolCallFunc{
									Name:      name,
									Arguments: args,
								},
							})
						}
					}
				}
			}
			return result
		}
	}

	if name, ok := raw["tool_call_name"].(string); ok {
		args := map[string]any{}
		if a, ok := raw["tool_call_arguments"].(map[string]any); ok {
			args = a
		}
		return []LLMMessageToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: ToolCallFunc{
					Name:      name,
					Arguments: args,
				},
			},
		}
	}

	return nil
}

func idToStr(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
