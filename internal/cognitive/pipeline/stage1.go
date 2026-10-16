package pipeline

import (
	"context"
	"log/slog"
	"time"

	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

type Stage1Recall struct {
	store            storage.Store
	embeddingService *llm.EmbeddingService
	logger           *slog.Logger
}

func NewStage1Recall(store storage.Store, embeddingService *llm.EmbeddingService, logger *slog.Logger) *Stage1Recall {
	if logger == nil {
		logger = slog.Default()
	}
	return &Stage1Recall{
		store:            store,
		embeddingService: embeddingService,
		logger:           logger,
	}
}

func (s *Stage1Recall) Name() string {
	return "recall"
}

func (s *Stage1Recall) Execute(ctx context.Context, input StageInput) (StageOutput, error) {
	startTime := time.Now()

	perception := input.Stage0Output
	if perception == nil {
		return StageOutput{}, nil
	}

	recall := &models.RecallOutput{
		ConversationID: perception.ConversationID,
		TurnID:         perception.TurnID,
	}

	topics := perception.Topics

	if err := s.retrieveEpisodicMemories(ctx, recall, perception); err != nil {
		s.logger.Warn("episodic memory retrieval failed", "error", err)
	}

	if err := s.retrieveSemanticMemories(ctx, recall, perception); err != nil {
		s.logger.Warn("semantic memory retrieval failed", "error", err)
	}

	if err := s.retrieveMindState(ctx, recall, topics); err != nil {
		s.logger.Warn("mind state retrieval failed", "error", err)
	}

	if err := s.retrieveUserInfoMemories(ctx, recall, perception.ConversationID); err != nil {
		s.logger.Warn("user info memory retrieval failed", "error", err)
	}

	recall.RetrievalTimeMs = time.Since(startTime).Milliseconds()

	s.logger.Info("recall completed",
		"episodic", len(recall.EpisodicMemories),
		"semantic", len(recall.SemanticMemories),
		"beliefs", len(recall.Beliefs),
		"opinions", len(recall.Opinions),
		"duration_ms", recall.RetrievalTimeMs,
	)

	return NewStageOutput(1, true, time.Since(startTime).Milliseconds(), recall), nil
}

func (s *Stage1Recall) retrieveEpisodicMemories(ctx context.Context, recall *models.RecallOutput, perception *models.PerceptionOutput) error {
	query := models.MemoryQuery{
		Type:          models.MemoryTypeEpisodic,
		Limit:         10,
		Tags:          perception.Topics,
		MinSimilarity: 0.5,
	}

	results, err := s.store.Memory().Search(ctx, query)
	if err != nil {
		return err
	}

	if s.embeddingService != nil && perception.NormalizedInput != "" {
		queryEmbedding, err := s.embeddingService.Embed(ctx, perception.NormalizedInput)
		if err == nil {
			for i := range results {
				if len(results[i].Memory.Embedding) > 0 {
					similarity := float64(llm.CosineSimilarity(queryEmbedding, results[i].Memory.Embedding))
					results[i].Similarity = similarity
					results[i].Relevance = (results[i].Relevance + similarity) / 2
				}
			}
			sortMemoryResults(results)

			if query.MinSimilarity > 0 {
				filtered := results[:0]
				for _, r := range results {
					if r.Similarity >= float64(query.MinSimilarity) {
						filtered = append(filtered, r)
					}
				}
				results = filtered
			}
		}
	}

	recall.EpisodicMemories = results
	return nil
}

func (s *Stage1Recall) retrieveSemanticMemories(ctx context.Context, recall *models.RecallOutput, perception *models.PerceptionOutput) error {
	query := models.MemoryQuery{
		Type:          models.MemoryTypeSemantic,
		Limit:         10,
		Tags:          perception.Topics,
		MinSimilarity: 0.5,
	}

	results, err := s.store.Memory().Search(ctx, query)
	if err != nil {
		return err
	}

	if s.embeddingService != nil && perception.NormalizedInput != "" {
		queryEmbedding, err := s.embeddingService.Embed(ctx, perception.NormalizedInput)
		if err == nil {
			for i := range results {
				if len(results[i].Memory.Embedding) > 0 {
					similarity := float64(llm.CosineSimilarity(queryEmbedding, results[i].Memory.Embedding))
					results[i].Similarity = similarity
					results[i].Relevance = (results[i].Relevance + similarity) / 2
				}
			}
			sortMemoryResults(results)

			if query.MinSimilarity > 0 {
				filtered := results[:0]
				for _, r := range results {
					if r.Similarity >= float64(query.MinSimilarity) {
						filtered = append(filtered, r)
					}
				}
				results = filtered
			}
		}
	}

	recall.SemanticMemories = results
	return nil
}

func sortMemoryResults(results []models.MemorySearchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Relevance > results[i].Relevance {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

func (s *Stage1Recall) retrieveMindState(ctx context.Context, recall *models.RecallOutput, topics []string) error {
	mindQuery := models.MindQuery{
		Subjects: topics,
		Limit:    20,
	}

	beliefs, err := s.store.Mind().GetBeliefs(ctx, mindQuery)
	if err != nil {
		return err
	}
	recall.Beliefs = beliefs

	opinions, err := s.store.Mind().GetOpinions(ctx, mindQuery)
	if err != nil {
		return err
	}
	recall.Opinions = opinions

	preferences, err := s.store.Mind().GetPreferences(ctx, mindQuery)
	if err != nil {
		return err
	}
	recall.Preferences = preferences

	goals, err := s.store.Mind().GetGoals(ctx, models.MindQuery{
		Status: models.StatusActive,
		Limit:  10,
	})
	if err != nil {
		return err
	}
	recall.Goals = goals

	hypotheses, err := s.store.Mind().GetHypotheses(ctx, mindQuery)
	if err != nil {
		return err
	}
	recall.Hypotheses = hypotheses

	uncertainties, err := s.store.Mind().GetUncertainties(ctx, mindQuery)
	if err != nil {
		return err
	}
	recall.Uncertainties = uncertainties

	selfObs, err := s.store.Mind().GetSelfObservations(ctx, mindQuery)
	if err != nil {
		return err
	}
	recall.SelfObservations = selfObs

	contradictions, err := s.store.Mind().GetContradictions(ctx, models.MindQuery{
		Limit: 10,
	})
	if err != nil {
		return err
	}
	recall.Contradictions = contradictions

	return nil
}

func (s *Stage1Recall) retrieveUserInfoMemories(ctx context.Context, recall *models.RecallOutput, conversationID string) error {
	if conversationID == "" {
		return nil
	}

	query := models.MemoryQuery{
		Type:           models.MemoryTypeEpisodic,
		Limit:          20,
		Tags:           []string{"user_facts"},
		MinSimilarity:  0.0,
		ConversationID: conversationID,
	}

	results, err := s.store.Memory().Search(ctx, query)
	if err != nil {
		return err
	}

	for _, mem := range results {
		recall.UserInfoMemories = append(recall.UserInfoMemories, mem)
	}

	return nil
}
