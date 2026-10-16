package pipeline

import (
	"context"
	"testing"

	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestStage0Perception_Fallback(t *testing.T) {
	// Use the real registry with a mock provider
	registry := llm.NewProviderRegistry(nil)
	mockProvider := llm.NewMockProvider()

	// Set a sequential response that will be returned
	mockProvider.SetSequentialResponses([]string{
		`{"language": "en", "topics": ["test"], "entities": [], "intent": "question", "questions": ["What is this?"], "requests": [], "signals": {}, "retrieval_hinted": true}`,
	})

	registry.Register("test-provider", mockProvider)

	stage := NewStage0Perception(registry, nil)

	input := StageInput{
		PipelineInput: models.PipelineInput{
			ConversationID: "test-conv",
			TurnID:         "test-turn",
			UserMessage:    "What is this?",
		},
		ModelName:    "test-model",
		ProviderName: "test-provider",
	}

	ctx := context.Background()
	output, err := stage.Execute(ctx, input)

	assert.NoError(t, err)
	assert.True(t, output.Success)

	perception, ok := output.Raw.(*models.PerceptionOutput)
	assert.True(t, ok)
	assert.Equal(t, "test-conv", perception.ConversationID)
	assert.Equal(t, "en", perception.Language)
	assert.Contains(t, perception.Topics, "test")
	assert.Contains(t, perception.DebugPrompt, "What is this?")
}

func TestStage0Perception_Validation(t *testing.T) {
	registry := llm.NewProviderRegistry(nil)
	// No provider registered - should use fallback perception

	stage := NewStage0Perception(registry, nil)

	input := StageInput{
		PipelineInput: models.PipelineInput{
			ConversationID: "test-conv",
			TurnID:         "test-turn",
			UserMessage:    "test message",
		},
		ModelName:    "test-model",
		ProviderName: "nonexistent",
	}

	ctx := context.Background()
	output, err := stage.Execute(ctx, input)

	// Should succeed using fallback perception
	assert.NoError(t, err)
	assert.True(t, output.Success)

	perception, ok := output.Raw.(*models.PerceptionOutput)
	assert.True(t, ok)
	assert.NotEmpty(t, perception.ConversationID)
}
