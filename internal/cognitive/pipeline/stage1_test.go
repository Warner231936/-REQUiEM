package pipeline

import (
	"context"
	"fmt"
	"testing"

	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
	"github.com/stretchr/testify/assert"
)

// MockStoreWithMind provides a simple in-memory store for testing
type MockStoreWithMind struct {
	memories  []models.Memory
	beliefs   []models.Belief
	opinions  []models.Opinion
	convStore *MockConversationStore
}

func NewMockStoreWithMind() *MockStoreWithMind {
	return &MockStoreWithMind{
		memories:  make([]models.Memory, 0),
		beliefs:   make([]models.Belief, 0),
		opinions:  make([]models.Opinion, 0),
		convStore: NewMockConversationStore(),
	}
}

func (m *MockStoreWithMind) Memory() storage.MemoryStore {
	return m
}

func (m *MockStoreWithMind) Mind() storage.MindStore {
	return m
}

func (m *MockStoreWithMind) Conversation() storage.ConversationStore {
	return m.convStore
}

func (m *MockStoreWithMind) Event() storage.EventStore {
	return nil
}

func (m *MockStoreWithMind) Audit() storage.AuditStore {
	return nil
}

func (m *MockStoreWithMind) PipelineRun() storage.PipelineRunStore {
	return nil
}

func (m *MockStoreWithMind) Auth() storage.AuthStore {
	return nil
}

func (m *MockStoreWithMind) Evidence() storage.EvidenceStore {
	return nil
}

func (m *MockStoreWithMind) Claim() storage.ClaimStore {
	return nil
}

func (m *MockStoreWithMind) Verification() storage.VerificationStore {
	return nil
}

func (m *MockStoreWithMind) StartTransaction(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func (m *MockStoreWithMind) CommitTransaction(ctx context.Context) error {
	return nil
}

func (m *MockStoreWithMind) AbortTransaction(ctx context.Context) error {
	return nil
}

func (m *MockStoreWithMind) WithRetry(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// MemoryStore implementation
func (m *MockStoreWithMind) Create(ctx context.Context, memory models.Memory) error {
	m.memories = append(m.memories, memory)
	return nil
}

func (m *MockStoreWithMind) Get(ctx context.Context, id string) (models.Memory, error) {
	for _, mem := range m.memories {
		if mem.ID == id {
			return mem, nil
		}
	}
	return models.Memory{}, assert.AnError
}

func (m *MockStoreWithMind) Search(ctx context.Context, query models.MemoryQuery) ([]models.MemorySearchResult, error) {
	results := make([]models.MemorySearchResult, 0)
	for _, mem := range m.memories {
		if query.Type == "" || mem.Type == query.Type {
			results = append(results, models.MemorySearchResult{
				Memory:     mem,
				Similarity: 0.9,
				Relevance:  0.9,
			})
		}
	}
	return results, nil
}

func (m *MockStoreWithMind) Update(ctx context.Context, memory models.Memory) error {
	for i, mem := range m.memories {
		if mem.ID == memory.ID {
			m.memories[i] = memory
			return nil
		}
	}
	return assert.AnError
}

func (m *MockStoreWithMind) Delete(ctx context.Context, id string) error {
	for i, mem := range m.memories {
		if mem.ID == id {
			m.memories = append(m.memories[:i], m.memories[i+1:]...)
			return nil
		}
	}
	return assert.AnError
}

func (m *MockStoreWithMind) GetByConversation(ctx context.Context, conversationID string, limit int) ([]models.Memory, error) {
	var results []models.Memory
	for _, mem := range m.memories {
		if mem.ConversationID == conversationID {
			results = append(results, mem)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// MindStore implementation
func (m *MockStoreWithMind) GetBeliefs(ctx context.Context, query models.MindQuery) ([]models.Belief, error) {
	return m.beliefs, nil
}

func (m *MockStoreWithMind) CreateBelief(ctx context.Context, belief models.Belief) error {
	m.beliefs = append(m.beliefs, belief)
	return nil
}

func (m *MockStoreWithMind) UpdateBelief(ctx context.Context, belief models.Belief) error {
	for i, b := range m.beliefs {
		if b.ID == belief.ID {
			m.beliefs[i] = belief
			return nil
		}
	}
	return assert.AnError
}

func (m *MockStoreWithMind) GetOpinions(ctx context.Context, query models.MindQuery) ([]models.Opinion, error) {
	return m.opinions, nil
}

func (m *MockStoreWithMind) CreateOpinion(ctx context.Context, opinion models.Opinion) error {
	m.opinions = append(m.opinions, opinion)
	return nil
}

func (m *MockStoreWithMind) UpdateOpinion(ctx context.Context, opinion models.Opinion) error {
	for i, o := range m.opinions {
		if o.ID == opinion.ID {
			m.opinions[i] = opinion
			return nil
		}
	}
	return assert.AnError
}

func (m *MockStoreWithMind) GetPreferences(ctx context.Context, query models.MindQuery) ([]models.Preference, error) {
	return []models.Preference{}, nil
}

func (m *MockStoreWithMind) CreatePreference(ctx context.Context, preference models.Preference) error {
	return nil
}

func (m *MockStoreWithMind) UpdatePreference(ctx context.Context, preference models.Preference) error {
	return nil
}

func (m *MockStoreWithMind) GetValues(ctx context.Context, query models.MindQuery) ([]models.Value, error) {
	return []models.Value{}, nil
}

func (m *MockStoreWithMind) CreateValue(ctx context.Context, value models.Value) error {
	return nil
}

func (m *MockStoreWithMind) GetGoals(ctx context.Context, query models.MindQuery) ([]models.Goal, error) {
	return []models.Goal{}, nil
}

func (m *MockStoreWithMind) CreateGoal(ctx context.Context, goal models.Goal) error {
	return nil
}

func (m *MockStoreWithMind) UpdateGoal(ctx context.Context, goal models.Goal) error {
	return nil
}

func (m *MockStoreWithMind) GetHypotheses(ctx context.Context, query models.MindQuery) ([]models.Hypothesis, error) {
	return []models.Hypothesis{}, nil
}

func (m *MockStoreWithMind) CreateHypothesis(ctx context.Context, hypothesis models.Hypothesis) error {
	return nil
}

func (m *MockStoreWithMind) UpdateHypothesis(ctx context.Context, hypothesis models.Hypothesis) error {
	return nil
}

func (m *MockStoreWithMind) GetUncertainties(ctx context.Context, query models.MindQuery) ([]models.Uncertainty, error) {
	return []models.Uncertainty{}, nil
}

func (m *MockStoreWithMind) CreateUncertainty(ctx context.Context, uncertainty models.Uncertainty) error {
	return nil
}

func (m *MockStoreWithMind) UpdateUncertainty(ctx context.Context, uncertainty models.Uncertainty) error {
	return nil
}

func (m *MockStoreWithMind) GetSelfObservations(ctx context.Context, query models.MindQuery) ([]models.SelfObservation, error) {
	return []models.SelfObservation{}, nil
}

func (m *MockStoreWithMind) CreateSelfObservation(ctx context.Context, observation models.SelfObservation) error {
	return nil
}

func (m *MockStoreWithMind) UpdateSelfObservation(ctx context.Context, observation models.SelfObservation) error {
	return nil
}

func (m *MockStoreWithMind) GetContradictions(ctx context.Context, query models.MindQuery) ([]models.Contradiction, error) {
	return []models.Contradiction{}, nil
}

func (m *MockStoreWithMind) CreateContradiction(ctx context.Context, contradiction models.Contradiction) error {
	return nil
}

func (m *MockStoreWithMind) UpdateContradiction(ctx context.Context, contradiction models.Contradiction) error {
	return nil
}

// MockConversationStore for testing
type MockConversationStore struct {
	messages map[string][]models.ConversationMessage
}

func NewMockConversationStore() *MockConversationStore {
	return &MockConversationStore{
		messages: make(map[string][]models.ConversationMessage),
	}
}

func (m *MockConversationStore) CreateConversation(ctx context.Context, message models.ConversationMessage) error {
	m.messages[message.ConversationID] = []models.ConversationMessage{message}
	return nil
}

func (m *MockConversationStore) AddMessage(ctx context.Context, conversationID string, message models.ConversationMessage) error {
	m.messages[conversationID] = append(m.messages[conversationID], message)
	return nil
}

func (m *MockConversationStore) GetMessages(ctx context.Context, conversationID string, limit int) ([]models.ConversationMessage, error) {
	msgs := m.messages[conversationID]
	if limit > 0 && limit < len(msgs) {
		return msgs[:limit], nil
	}
	return msgs, nil
}

func (m *MockConversationStore) GetMessagesPaginated(ctx context.Context, conversationID string, limit, offset int) ([]models.ConversationMessage, error) {
	msgs := m.messages[conversationID]
	if offset >= len(msgs) {
		return []models.ConversationMessage{}, nil
	}
	end := offset + limit
	if end > len(msgs) {
		end = len(msgs)
	}
	return msgs[offset:end], nil
}

func (m *MockConversationStore) ListConversations(ctx context.Context, limit, offset int) ([]storage.ConversationSummary, error) {
	return []storage.ConversationSummary{}, nil
}

func (m *MockConversationStore) DeleteAll(ctx context.Context, conversationID string) (int64, error) {
	count := int64(len(m.messages[conversationID]))
	delete(m.messages, conversationID)
	return count, nil
}

func TestStage1Recall(t *testing.T) {
	store := &MockStoreWithMind{
		memories:  make([]models.Memory, 0),
		beliefs:   make([]models.Belief, 0),
		opinions:  make([]models.Opinion, 0),
		convStore: NewMockConversationStore(),
	}

	// Add a test memory
	store.Create(context.Background(), models.Memory{
		BaseRecord: models.NewBaseRecord("mem-1"),
		Type:       models.MemoryTypeEpisodic,
		Content:    "Test memory content",
		Confidence: 0.8,
	})

	// Since Stage1 requires embeddingService which is nil, we test without it
	stage := NewStage1Recall(store, nil, nil)

	input := StageInput{
		PipelineInput: models.PipelineInput{
			ConversationID: "test-conv",
			TurnID:         "test-turn",
			UserMessage:    "test query",
		},
		Stage0Output: &models.PerceptionOutput{
			ConversationID:  "test-conv",
			TurnID:          "test-turn",
			NormalizedInput: "test query",
			Topics:          []string{"test"},
		},
		ModelName:    "test-model",
		ProviderName: "test-provider",
	}

	ctx := context.Background()
	output, err := stage.Execute(ctx, input)

	assert.NoError(t, err)
	assert.True(t, output.Success)

	recall, ok := output.Raw.(*models.RecallOutput)
	assert.True(t, ok)
	assert.NotNil(t, recall.EpisodicMemories)
	assert.Equal(t, "test-conv", recall.ConversationID)
}

type controllableEmbeddingProvider struct {
	embeddings map[string][]float32
}

func (c *controllableEmbeddingProvider) Name() string { return "controllable" }
func (c *controllableEmbeddingProvider) Embed(_ context.Context, text string) ([]float32, error) {
	if emb, ok := c.embeddings[text]; ok {
		return emb, nil
	}
	return []float32{0, 0, 0}, nil
}

func TestStage1RecallRecentConversationRelevanceFilter(t *testing.T) {
	store := &MockStoreWithMind{
		memories:  make([]models.Memory, 0),
		beliefs:   make([]models.Belief, 0),
		opinions:  make([]models.Opinion, 0),
		convStore: NewMockConversationStore(),
	}

	// Set up messages in the conversation
	convID := "test-conv-relevance"
	relevantContent := "discuss project architecture"
	irrelevantContent := "the weather is nice today"

	store.convStore.AddMessage(context.Background(), convID, models.ConversationMessage{
		ConversationID: convID,
		Role:           "user",
		Content:        "What is the architecture?",
	})
	store.convStore.AddMessage(context.Background(), convID, models.ConversationMessage{
		ConversationID: convID,
		Role:           "assistant",
		Content:        relevantContent,
	})
	store.convStore.AddMessage(context.Background(), convID, models.ConversationMessage{
		ConversationID: convID,
		Role:           "user",
		Content:        irrelevantContent,
	})

	// Embeddings: query matches relevantContent exactly (similarity 1.0),
	// irrelevantContent is orthogonal (similarity 0)
	shared := []float32{1, 0, 0}
	queryText := "discuss project architecture"
	provider := &controllableEmbeddingProvider{
		embeddings: map[string][]float32{
			queryText:                   shared,
			relevantContent:             shared,
			irrelevantContent:           {0, 1, 0},
			"What is the architecture?": {0.9, 0.1, 0},
		},
	}
	embeddingService := llm.NewEmbeddingService(provider)

	stage := NewStage1Recall(store, embeddingService, nil)

	input := StageInput{
		PipelineInput: models.PipelineInput{
			ConversationID: convID,
			TurnID:         "test-turn",
			UserMessage:    queryText,
		},
		Stage0Output: &models.PerceptionOutput{
			ConversationID:  convID,
			TurnID:          "test-turn",
			NormalizedInput: queryText,
			Topics:          []string{"architecture"},
		},
	}

	ctx := context.Background()
	output, err := stage.Execute(ctx, input)

	assert.NoError(t, err)
	assert.True(t, output.Success)

	recall, ok := output.Raw.(*models.RecallOutput)
	assert.True(t, ok)

	// Note: RecentMessages no longer populated - tools handle on-demand retrieval
	assert.NotNil(t, recall)
}

func TestStage1RecallRecentConversationFallbackWhenNoEmbedding(t *testing.T) {
	store := &MockStoreWithMind{
		memories:  make([]models.Memory, 0),
		beliefs:   make([]models.Belief, 0),
		opinions:  make([]models.Opinion, 0),
		convStore: NewMockConversationStore(),
	}

	convID := "test-conv-no-emb"
	store.convStore.AddMessage(context.Background(), convID, models.ConversationMessage{
		ConversationID: convID,
		Role:           "user",
		Content:        "message one",
	})
	store.convStore.AddMessage(context.Background(), convID, models.ConversationMessage{
		ConversationID: convID,
		Role:           "assistant",
		Content:        "message two",
	})

	stage := NewStage1Recall(store, nil, nil)

	input := StageInput{
		PipelineInput: models.PipelineInput{
			ConversationID: convID,
			TurnID:         "test-turn",
			UserMessage:    "test query",
		},
		Stage0Output: &models.PerceptionOutput{
			ConversationID:  convID,
			TurnID:          "test-turn",
			NormalizedInput: "test query",
			Topics:          []string{"test"},
		},
	}

	ctx := context.Background()
	output, err := stage.Execute(ctx, input)

	assert.NoError(t, err)
	assert.True(t, output.Success)

	recall, ok := output.Raw.(*models.RecallOutput)
	assert.True(t, ok)
	// Note: RecentMessages no longer populated - tools handle on-demand retrieval
	assert.Equal(t, 0, len(recall.RecentMessages))
}

func TestStage1RecallRecentConversationFallbackWhenAllIrrelevant(t *testing.T) {
	store := &MockStoreWithMind{
		memories:  make([]models.Memory, 0),
		beliefs:   make([]models.Belief, 0),
		opinions:  make([]models.Opinion, 0),
		convStore: NewMockConversationStore(),
	}

	convID := "test-conv-all-irrelevant"
	for i := 0; i < 10; i++ {
		store.convStore.AddMessage(context.Background(), convID, models.ConversationMessage{
			ConversationID: convID,
			Role:           "user",
			Content:        fmt.Sprintf("unrelated message %d", i),
		})
	}

	provider := &controllableEmbeddingProvider{
		embeddings: map[string][]float32{
			"test query": {1, 0, 0},
		},
	}
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("unrelated message %d", i)
		provider.embeddings[key] = []float32{0, 1, 0}
	}
	embeddingService := llm.NewEmbeddingService(provider)

	stage := NewStage1Recall(store, embeddingService, nil)

	input := StageInput{
		PipelineInput: models.PipelineInput{
			ConversationID: convID,
			TurnID:         "test-turn",
			UserMessage:    "test query",
		},
		Stage0Output: &models.PerceptionOutput{
			ConversationID:  convID,
			TurnID:          "test-turn",
			NormalizedInput: "test query",
			Topics:          []string{"test"},
		},
	}

	ctx := context.Background()
	output, err := stage.Execute(ctx, input)

	assert.NoError(t, err)
	assert.True(t, output.Success)

	recall, ok := output.Raw.(*models.RecallOutput)
	assert.True(t, ok)
	// Note: RecentMessages no longer populated - tools handle on-demand retrieval
	assert.Equal(t, 0, len(recall.RecentMessages))
}
