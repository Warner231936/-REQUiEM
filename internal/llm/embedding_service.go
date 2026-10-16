package llm

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type EmbeddingCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	ttl     time.Duration
	maxSize int
}

type cacheEntry struct {
	embedding []float32
	timestamp time.Time
}

func NewEmbeddingCache(ttl time.Duration, maxSize int) *EmbeddingCache {
	if ttl <= 0 {
		ttl = 1 * time.Hour
	}
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &EmbeddingCache{
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

func (c *EmbeddingCache) Get(text string) ([]float32, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[text]
	if !ok {
		return nil, false
	}

	if time.Since(entry.timestamp) > c.ttl {
		return nil, false
	}

	return entry.embedding, true
}

func (c *EmbeddingCache) Put(text string, embedding []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[text] = cacheEntry{
		embedding: embedding,
		timestamp: time.Now(),
	}
}

func (c *EmbeddingCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, v := range c.entries {
		if first || v.timestamp.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.timestamp
			first = false
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

type EmbeddingService struct {
	provider EmbeddingProvider
	cache    *EmbeddingCache
}

func NewEmbeddingService(provider EmbeddingProvider) *EmbeddingService {
	return &EmbeddingService{
		provider: provider,
		cache:    NewEmbeddingCache(1*time.Hour, 10000),
	}
}

func (s *EmbeddingService) Embed(ctx context.Context, text string) ([]float32, error) {
	if cached, ok := s.cache.Get(text); ok {
		return cached, nil
	}

	embedding, err := s.provider.Embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	s.cache.Put(text, embedding)
	return embedding, nil
}

func (s *EmbeddingService) BatchEmbed(ctx context.Context, texts []string) (map[string][]float32, error) {
	results := make(map[string][]float32)
	for _, text := range texts {
		embedding, err := s.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		results[text] = embedding
	}
	return results, nil
}
