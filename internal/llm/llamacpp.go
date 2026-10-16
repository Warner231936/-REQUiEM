package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type LlamaCppProvider struct {
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewLlamaCppProvider(baseURL string, logger *slog.Logger) *LlamaCppProvider {
	if logger == nil {
		logger = slog.Default()
	}
	return &LlamaCppProvider{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		logger: logger,
	}
}

func (p *LlamaCppProvider) Name() string {
	return "llamacpp"
}

func (p *LlamaCppProvider) Generate(ctx context.Context, request LLMRequest) (LLMResponse, error) {
	startTime := time.Now()

	reqBody := map[string]any{
		"model":    request.Model,
		"messages": request.Messages,
		"stream":   false,
	}

	if request.Temperature > 0 {
		reqBody["temperature"] = request.Temperature
	}
	if request.MaxTokens > 0 {
		reqBody["max_tokens"] = request.MaxTokens
	}
	if request.Format == "json" {
		reqBody["response_format"] = map[string]string{"type": "json_object"}
	}

	if len(request.Tools) > 0 {
		tools := make([]map[string]any, len(request.Tools))
		for i, t := range request.Tools {
			tools[i] = map[string]any{
				"type": t.Type,
				"function": map[string]any{
					"name":        t.Function.Name,
					"description": t.Function.Description,
					"parameters":  t.Function.Parameters,
					"strict":      t.Function.Strict,
				},
			}
		}
		reqBody["tools"] = tools
		if request.ToolChoice != "" {
			reqBody["tool_choice"] = request.ToolChoice
		}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/chat/completions", p.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return LLMResponse{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
		httpReq = httpReq.WithContext(ctx)
	}

	p.logger.Debug("llamacpp request", "model", request.Model, "url", url, "has_tools", len(request.Tools) > 0)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("llamacpp request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return LLMResponse{}, fmt.Errorf("llamacpp error (status %d): %s", resp.StatusCode, string(body))
	}

	var openaiResp struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Index        int                     `json:"index"`
			Message      llamaCppResponseMessage `json:"message"`
			FinishReason string                  `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return LLMResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}

	duration := time.Since(startTime)
	tokenCount := openaiResp.Usage.PromptTokens + openaiResp.Usage.CompletionTokens

	content := ""
	var toolCalls []LLMMessageToolCall
	if len(openaiResp.Choices) > 0 {
		content = openaiResp.Choices[0].Message.Content
		toolCalls = openaiResp.Choices[0].Message.ToolCalls
	}

	return LLMResponse{
		Model:      request.Model,
		Content:    content,
		Done:       true,
		TokenCount: tokenCount,
		DurationMs: duration.Milliseconds(),
		Provider:   p.Name(),
		ToolCalls:  toolCalls,
	}, nil
}

type llamaCppResponseMessage struct {
	Content   string               `json:"content"`
	Role      string               `json:"role"`
	ToolCalls []LLMMessageToolCall `json:"tool_calls,omitempty"`
}

func (p *LlamaCppProvider) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1/models", p.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("llamacpp health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("llamacpp health check returned status %d", resp.StatusCode)
	}
	return nil
}
