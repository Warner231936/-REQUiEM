package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/requiem/cognitive/internal/cognitive/pipeline"
	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

func TestFullPipelineIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	store, err := storage.NewMongoStore(ctx, storage.MongoConfig{
		URI:      "mongodb://127.0.0.1:27017",
		Database: "requiem_test",
	}, nil)
	if err != nil {
		t.Skipf("Skipping integration test - MongoDB not available: %v", err)
	}
	defer store.Close(context.Background())

	registry := llm.NewProviderRegistry(nil)
	ollama := llm.NewOllamaProvider("http://127.0.0.1:11434", nil)
	registry.Register("ollama", ollama)

	cfg := models.PipelineConfig{
		SynchronousConsolidation: true,
		Stage0:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
		Stage1:                   models.StageConfig{Enabled: true},
		Stage2:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
		Stage3:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
		Stage4:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
		Stage5:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
		Stage6:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
	}

	p := pipeline.NewPipeline(cfg, registry, store, nil)
	p.RegisterStage(0, pipeline.NewStage0Perception(registry, nil))
	p.RegisterStage(1, pipeline.NewStage1Recall(store, nil, nil))
	p.RegisterStage(2, pipeline.NewStage2Interpretation(registry, nil))
	p.RegisterStage(3, pipeline.NewStage3Reflection(registry, nil))
	p.RegisterStage(4, pipeline.NewStage4Response(registry, nil))
	p.RegisterStage(5, pipeline.NewStage5ResponseHumanize(registry, nil))
	p.RegisterStage(6, pipeline.NewStage6Consolidation(store, nil, nil))

	convID := uuid.New().String()

	t.Run("Turn1_ExpressPreference", func(t *testing.T) {
		input := models.PipelineInput{
			ConversationID: convID,
			TurnID:         uuid.New().String(),
			UserMessage:    "I prefer architecture where persistent state is separated from model weights.",
			Timestamp:      time.Now().UTC(),
		}

		output, err := p.Execute(ctx, input)
		if err != nil {
			t.Fatalf("Pipeline execution failed: %v", err)
		}

		if output.Response == "" {
			t.Error("Expected non-empty response")
		}

		if len(output.StageOutputs) != 7 {
			t.Errorf("Expected 7 stage outputs, got %d", len(output.StageOutputs))
		}

		for _, sr := range output.StageOutputs {
			if !sr.Success {
				t.Errorf("Stage %d (%s) failed: %s", sr.Stage, sr.Name, sr.Error)
			}
		}
	})

	t.Run("Turn2_ReinforcePreference", func(t *testing.T) {
		input := models.PipelineInput{
			ConversationID: convID,
			TurnID:         uuid.New().String(),
			UserMessage:    "Yes, separating memory from reasoning makes the system more maintainable.",
			Timestamp:      time.Now().UTC(),
		}

		output, err := p.Execute(ctx, input)
		if err != nil {
			t.Fatalf("Pipeline execution failed: %v", err)
		}

		if output.Response == "" {
			t.Error("Expected non-empty response")
		}
	})

	t.Run("Turn3_ContradictPreference", func(t *testing.T) {
		input := models.PipelineInput{
			ConversationID: convID,
			TurnID:         uuid.New().String(),
			UserMessage:    "Actually, I'm not sure if external state is always necessary.",
			Timestamp:      time.Now().UTC(),
		}

		output, err := p.Execute(ctx, input)
		if err != nil {
			t.Fatalf("Pipeline execution failed: %v", err)
		}

		if output.Response == "" {
			t.Error("Expected non-empty response")
		}
	})

	t.Run("VerifyMindState", func(t *testing.T) {
		beliefs, err := store.Mind().GetBeliefs(ctx, models.MindQuery{Limit: 20})
		if err != nil {
			t.Fatalf("Failed to get beliefs: %v", err)
		}
		t.Logf("Total beliefs: %d", len(beliefs))

		memories, err := store.Memory().Search(ctx, models.MemoryQuery{Limit: 20})
		if err != nil {
			t.Fatalf("Failed to get memories: %v", err)
		}
		t.Logf("Total memories: %d", len(memories))

		contradictions, err := store.Mind().GetContradictions(ctx, models.MindQuery{Limit: 10})
		if err != nil {
			t.Fatalf("Failed to get contradictions: %v", err)
		}
		t.Logf("Total contradictions: %d", len(contradictions))

		for _, b := range beliefs {
			history, err := store.Audit().GetHistory(ctx, "belief", b.ID)
			if err != nil {
				t.Logf("Failed to get history for belief %s: %v", b.ID, err)
				continue
			}
			if len(history) > 0 {
				t.Logf("Belief %s has %d history entries", b.ID, len(history))
			}
		}
	})
}

func TestConcurrentPipelineExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	store, err := storage.NewMongoStore(ctx, storage.MongoConfig{
		URI:      "mongodb://127.0.0.1:27017",
		Database: "requiem_test",
	}, nil)
	if err != nil {
		t.Skipf("Skipping integration test - MongoDB not available: %v", err)
	}
	defer store.Close(context.Background())

	registry := llm.NewProviderRegistry(nil)
	ollama := llm.NewOllamaProvider("http://127.0.0.1:11434", nil)
	registry.Register("ollama", ollama)

	cfg := models.PipelineConfig{
		SynchronousConsolidation: true,
		Stage0:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
		Stage1:                   models.StageConfig{Enabled: true},
		Stage2:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
		Stage3:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
		Stage4:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
		Stage5:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
		Stage6:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
	}

	p := pipeline.NewPipeline(cfg, registry, store, nil)
	p.RegisterStage(0, pipeline.NewStage0Perception(registry, nil))
	p.RegisterStage(1, pipeline.NewStage1Recall(store, nil, nil))
	p.RegisterStage(2, pipeline.NewStage2Interpretation(registry, nil))
	p.RegisterStage(3, pipeline.NewStage3Reflection(registry, nil))
	p.RegisterStage(4, pipeline.NewStage4Response(registry, nil))
	p.RegisterStage(5, pipeline.NewStage5ResponseHumanize(registry, nil))
	p.RegisterStage(6, pipeline.NewStage6Consolidation(store, nil, nil))

	numConcurrent := 3
	done := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func(idx int) {
			input := models.PipelineInput{
				ConversationID: uuid.New().String(),
				TurnID:         uuid.New().String(),
				UserMessage:    "Test concurrent message",
				Timestamp:      time.Now().UTC(),
			}

			_, err := p.Execute(ctx, input)
			done <- err
		}(i)
	}

	for i := 0; i < numConcurrent; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent execution %d failed: %v", i, err)
		}
	}
}
