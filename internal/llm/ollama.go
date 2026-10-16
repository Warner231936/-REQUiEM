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

type OllamaProvider struct {
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewOllamaProvider(baseURL string, logger *slog.Logger) *OllamaProvider {
	if logger == nil {
		logger = slog.Default()
	}
	return &OllamaProvider{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		logger: logger,
	}
}

func (p *OllamaProvider) Name() string {
	return "ollama"
}

func (p *OllamaProvider) Generate(ctx context.Context, request LLMRequest) (LLMResponse, error) {
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
		reqBody["num_predict"] = request.MaxTokens
	}
	if request.Format == "json" {
		reqBody["format"] = "json"
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

	url := fmt.Sprintf("%s/api/chat", p.baseURL)
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

	p.logger.Debug("ollama request", "model", request.Model, "url", url, "has_tools", len(request.Tools) > 0)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return LLMResponse{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return LLMResponse{}, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(body))
	}

	var ollamaResp struct {
		Message struct {
			Content   string               `json:"content"`
			ToolCalls []LLMMessageToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		Model           string `json:"model"`
		Done            bool   `json:"done"`
		TotalDuration   int64  `json:"total_duration"`
		EvalCount       int    `json:"eval_count"`
		PromptEvalCount int    `json:"prompt_eval_count"`
	}

	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return LLMResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}

	duration := time.Since(startTime)
	tokenCount := ollamaResp.EvalCount + ollamaResp.PromptEvalCount

	return LLMResponse{
		Model:      ollamaResp.Model,
		Content:    ollamaResp.Message.Content,
		Done:       ollamaResp.Done,
		TokenCount: tokenCount,
		DurationMs: duration.Milliseconds(),
		Provider:   p.Name(),
		ToolCalls:  ollamaResp.Message.ToolCalls,
	}, nil
}

func (p *OllamaProvider) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/tags", p.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama health check returned status %d", resp.StatusCode)
	}
	return nil
}
