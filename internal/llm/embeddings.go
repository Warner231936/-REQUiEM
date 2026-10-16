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

type OllamaEmbeddingProvider struct {
	baseURL    string
	model      string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewOllamaEmbeddingProvider(baseURL, model string, logger *slog.Logger) *OllamaEmbeddingProvider {
	if logger == nil {
		logger = slog.Default()
	}
	if model == "" {
		model = "nomic-embed-text"
	}
	return &OllamaEmbeddingProvider{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

func (p *OllamaEmbeddingProvider) Name() string {
	return "ollama-embeddings"
}

func (p *OllamaEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]any{
		"model":  p.model,
		"prompt": text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	url := fmt.Sprintf("%s/api/embeddings", p.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding error (status %d): %s", resp.StatusCode, string(body))
	}

	var embedResp struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := json.Unmarshal(body, &embedResp); err != nil {
		return nil, fmt.Errorf("unmarshal embedding response: %w", err)
	}

	result := make([]float32, len(embedResp.Embedding))
	for i, v := range embedResp.Embedding {
		result[i] = float32(v)
	}

	return result, nil
}

type MockEmbeddingProvider struct {
	dimension int
}

func NewMockEmbeddingProvider(dimension int) *MockEmbeddingProvider {
	if dimension <= 0 {
		dimension = 384
	}
	return &MockEmbeddingProvider{dimension: dimension}
}

func (m *MockEmbeddingProvider) Name() string {
	return "mock-embeddings"
}

func (m *MockEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	embedding := make([]float32, m.dimension)
	hash := hashString(text)
	for i := range embedding {
		embedding[i] = float32(hash%1000) / 1000.0
		hash = hash*31 + uint32(i)
	}
	return embedding, nil
}

func hashString(s string) uint32 {
	var h uint32 = 5381
	for i := 0; i < len(s); i++ {
		h = h*31 + uint32(s[i])
	}
	return h
}

func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(float64(normA) * float64(normB)))
}
