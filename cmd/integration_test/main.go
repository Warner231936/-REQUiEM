package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/requiem/cognitive/internal/cognitive/pipeline"
	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("=== REQUIEM COGNITIVE PIPELINE INTEGRATION TEST ===")
	fmt.Println()

	fmt.Println("[1] Connecting to MongoDB...")
	store, err := storage.NewMongoStore(ctx, storage.MongoConfig{
		URI:      "mongodb://127.0.0.1:27017",
		Database: "requiem",
	}, logger)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer store.Close(context.Background())
	fmt.Println("    Connected to MongoDB successfully")

	fmt.Println("[2] Setting up LLM provider (requiem1)...")
	registry := llm.NewProviderRegistry(logger)
	ollamaProvider := llm.NewOllamaProvider("http://127.0.0.1:11434", logger)
	registry.Register("ollama", ollamaProvider)
	fmt.Println("    Ollama provider registered")

	fmt.Println("[3] Initializing cognitive pipeline...")
	cfg := models.PipelineConfig{
		SynchronousConsolidation: true,
		Debug:                    true,
		Stage0:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "reqiquick"},
		Stage1:                   models.StageConfig{Enabled: true},
		Stage2:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
		Stage3:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
		Stage4:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
		Stage5:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
		Stage6:                   models.StageConfig{Enabled: true, Provider: "ollama", Model: "requiem1"},
	}

	p := pipeline.NewPipeline(cfg, registry, store, logger)
	p.RegisterStage(0, pipeline.NewStage0Perception(registry, logger))
	p.RegisterStage(1, pipeline.NewStage1Recall(store, nil, logger))
	p.RegisterStage(2, pipeline.NewStage2Interpretation(registry, logger))
	p.RegisterStage(3, pipeline.NewStage3Reflection(registry, logger))
	p.RegisterStage(4, pipeline.NewStage4Response(registry, logger))
	p.RegisterStage(5, pipeline.NewStage5ResponseHumanize(registry, logger))
	p.RegisterStage(6, pipeline.NewStage6Consolidation(store, nil, logger))
	fmt.Println("    Pipeline initialized with 6 stages")

	fmt.Println()
	fmt.Println("=== TURN 1: User expresses architectural preference ===")
	convID := uuid.New().String()
	turn1ID := uuid.New().String()

	input1 := models.PipelineInput{
		ConversationID: convID,
		TurnID:         turn1ID,
		UserMessage:    "I prefer architecture where persistent state is separated from model weights.",
		Timestamp:      time.Now().UTC(),
	}

	output1, err := p.Execute(ctx, input1)
	if err != nil {
		log.Fatalf("Pipeline execution failed: %v", err)
	}

	fmt.Printf("    Response: %s\n", output1.Response)
	fmt.Printf("    Duration: %dms\n", output1.DurationMs)
	fmt.Printf("    Stages completed: %d\n", len(output1.StageOutputs))
	for _, sr := range output1.StageOutputs {
		fmt.Printf("      - Stage %d (%s): success=%v, %dms\n", sr.Stage, sr.Name, sr.Success, sr.DurationMs)
	}

	fmt.Println()
	fmt.Println("=== VERIFY: Check MongoDB state ===")

	beliefs, err := store.Mind().GetBeliefs(ctx, models.MindQuery{Limit: 10})
	if err != nil {
		fmt.Printf("    Beliefs query: %v\n", err)
	} else {
		fmt.Printf("    Beliefs stored: %d\n", len(beliefs))
		for _, b := range beliefs {
			fmt.Printf("      - [%s] %s (confidence: %.2f)\n", b.Subject, b.Content, b.Confidence)
		}
	}

	opinions, err := store.Mind().GetOpinions(ctx, models.MindQuery{Limit: 10})
	if err != nil {
		fmt.Printf("    Opinions query: %v\n", err)
	} else {
		fmt.Printf("    Opinions stored: %d\n", len(opinions))
		for _, o := range opinions {
			fmt.Printf("      - [%s] %s (strength: %.2f)\n", o.Subject, o.Content, o.Strength)
		}
	}

	memories, err := store.Memory().Search(ctx, models.MemoryQuery{Limit: 10})
	if err != nil {
		fmt.Printf("    Memories query: %v\n", err)
	} else {
		fmt.Printf("    Memories stored: %d\n", len(memories))
		for _, m := range memories {
			fmt.Printf("      - [%s] %s (importance: %.2f)\n", m.Memory.Type, m.Memory.Content, m.Memory.Importance)
		}
	}

	fmt.Println()
	fmt.Println("=== TURN 2: Reinforce the same concept ===")
	turn2ID := uuid.New().String()

	input2 := models.PipelineInput{
		ConversationID: convID,
		TurnID:         turn2ID,
		UserMessage:    "Yes, separating memory from reasoning makes the system more maintainable.",
		Timestamp:      time.Now().UTC(),
	}

	output2, err := p.Execute(ctx, input2)
	if err != nil {
		log.Fatalf("Pipeline execution failed: %v", err)
	}

	fmt.Printf("    Response: %s\n", output2.Response)
	fmt.Printf("    Duration: %dms\n", output2.DurationMs)

	fmt.Println()
	fmt.Println("=== TURN 3: Contradictory information ===")
	turn3ID := uuid.New().String()

	input3 := models.PipelineInput{
		ConversationID: convID,
		TurnID:         turn3ID,
		UserMessage:    "Actually, I'm not sure if external state is always necessary.",
		Timestamp:      time.Now().UTC(),
	}

	output3, err := p.Execute(ctx, input3)
	if err != nil {
		log.Fatalf("Pipeline execution failed: %v", err)
	}

	fmt.Printf("    Response: %s\n", output3.Response)
	fmt.Printf("    Duration: %dms\n", output3.DurationMs)

	fmt.Println()
	fmt.Println("=== FINAL STATE VERIFICATION ===")

	beliefs, _ = store.Mind().GetBeliefs(ctx, models.MindQuery{Limit: 20})
	fmt.Printf("    Total beliefs: %d\n", len(beliefs))

	contradictions, _ := store.Mind().GetContradictions(ctx, models.MindQuery{Limit: 10})
	fmt.Printf("    Total contradictions: %d\n", len(contradictions))
	for _, c := range contradictions {
		fmt.Printf("      - A: %s vs B: %s (status: %s)\n", c.BeliefA, c.BeliefB, c.Status)
	}

	uncertainties, _ := store.Mind().GetUncertainties(ctx, models.MindQuery{Limit: 10})
	fmt.Printf("    Total uncertainties: %d\n", len(uncertainties))

	memories, _ = store.Memory().Search(ctx, models.MemoryQuery{Limit: 20})
	fmt.Printf("    Total memories: %d\n", len(memories))

	fmt.Println()
	fmt.Println("=== INTEGRATION TEST COMPLETE ===")
}
