package modelmgr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/requiem/cognitive/internal/modelmgr/hardware"
)

type OllamaClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewOllamaClient(baseURL string, logger *slog.Logger) *OllamaClient {
	if logger == nil {
		logger = slog.Default()
	}
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	return &OllamaClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

func (c *OllamaClient) BaseURL() string {
	return c.baseURL
}

func (c *OllamaClient) IsRunning(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (c *OllamaClient) Version(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/version", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama version endpoint returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var verResp struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &verResp); err != nil {
		return "", err
	}
	return verResp.Version, nil
}

func (c *OllamaClient) ListModels(ctx context.Context) ([]hardware.OllamaModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama /api/tags returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tagsResp struct {
		Models []struct {
			Name     string `json:"name"`
			Size     int64  `json:"size"`
			Modified string `json:"modified_at"`
			Digest   string `json:"digest"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &tagsResp); err != nil {
		return nil, err
	}

	models := make([]hardware.OllamaModelInfo, 0, len(tagsResp.Models))
	for _, m := range tagsResp.Models {
		digest := m.Digest
		if len(digest) > 8 {
			digest = digest[:8]
		}
		modified, _ := time.Parse(time.RFC3339Nano, m.Modified)
		if modified.IsZero() {
			modified, _ = time.Parse(time.RFC3339, m.Modified)
		}
		models = append(models, hardware.OllamaModelInfo{
			Name:     m.Name,
			Size:     m.Size,
			Modified: modified.UTC(),
			Digest:   digest,
		})
	}
	return models, nil
}

func (c *OllamaClient) HasModel(ctx context.Context, modelName string) bool {
	models, err := c.ListModels(ctx)
	if err != nil {
		return false
	}
	for _, m := range models {
		if strings.HasPrefix(m.Name, modelName+":") || m.Name == modelName {
			return true
		}
	}
	return false
}

func (c *OllamaClient) CreateModel(ctx context.Context, modelName, modelfile string) error {
	payload := map[string]string{
		"name":      modelName,
		"modelfile": modelfile,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal create payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/create", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("create model request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("create model failed (status %d): %s", resp.StatusCode, string(body))
	}

	var createResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &createResp); err == nil && createResp.Error != "" {
		return fmt.Errorf("create model error: %s", createResp.Error)
	}

	return nil
}

func (c *OllamaClient) DeleteModel(ctx context.Context, modelName string) error {
	payload := map[string]string{
		"name": modelName,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/delete", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete model failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *OllamaClient) Generate(ctx context.Context, modelName, prompt string, format string) (string, error) {
	reqBody := map[string]any{
		"model":  modelName,
		"prompt": prompt,
		"stream": false,
	}
	if format == "json" {
		reqBody["format"] = "json"
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("generate request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("generate failed (status %d): %s", resp.StatusCode, string(body))
	}

	var fullResponse strings.Builder
	decoder := json.NewDecoder(resp.Body)
	for {
		var partial struct {
			Model    string `json:"model"`
			Done     bool   `json:"done"`
			Response string `json:"response"`
		}
		if err := decoder.Decode(&partial); err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("decode generate response: %w", err)
		}
		fullResponse.WriteString(partial.Response)
		if partial.Done {
			break
		}
	}

	return fullResponse.String(), nil
}

func (c *OllamaClient) ChatRaw(ctx context.Context, modelName string, messages []OllamaMessage) (string, error) {
	reqBody := map[string]any{
		"model":    modelName,
		"messages": messages,
		"stream":   false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("chat failed (status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp struct {
		Model   string        `json:"model"`
		Message OllamaMessage `json:"message"`
		Done    bool          `json:"done"`
		Error   string        `json:"error,omitempty"`
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(body, &chatResp); err != nil {
		var genResp struct {
			Response string `json:"response"`
			Done     bool   `json:"done"`
		}
		if err2 := json.Unmarshal(body, &genResp); err2 == nil {
			return genResp.Response, nil
		}
		return "", err
	}

	if chatResp.Error != "" {
		return "", fmt.Errorf("ollama error: %s", chatResp.Error)
	}
	return chatResp.Message.Content, nil
}

type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (c *OllamaClient) PullModel(ctx context.Context, modelName string) error {
	payload := map[string]string{
		"name": modelName,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/pull", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pull failed (status %d): %s", resp.StatusCode, string(body))
	}

	var lastErr error
	decoder := json.NewDecoder(resp.Body)
	for {
		var partial struct {
			Status    string `json:"status"`
			Error     string `json:"error"`
			Total     int64  `json:"total"`
			Completed int64  `json:"completed"`
		}
		if err := decoder.Decode(&partial); err != nil {
			if err == io.EOF {
				break
			}
			break
		}
		if partial.Error != "" {
			lastErr = fmt.Errorf("pull error: %s", partial.Error)
		}
		c.logger.Debug("pull progress", "status", partial.Status, "total", partial.Total, "done", partial.Completed)
	}

	if lastErr != nil {
		return lastErr
	}
	return nil
}
