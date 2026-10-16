package pipeline

import (
	"context"
	"testing"

	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/pkg/models"
	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string {
	return &s
}

// TestStage2Interpretation tests the interpretation stage
func TestStage2Interpretation(t *testing.T) {
	registry := llm.NewProviderRegistry(nil)
	mockProvider := llm.NewMockProvider()

	mockProvider.SetSequentialResponses([]string{
		`{"intent": "question", "goal": "answer question", "context": ["test"], "confidence": 0.8, "observations": [], "inferences": [], "assumptions": [], "beliefs": [], "uncertainties": []}`,
	})
	registry.Register("test-provider", mockProvider)

	stage := NewStage2Interpretation(registry, nil)

	input := StageInput{
		PipelineInput: models.PipelineInput{
			ConversationID: "test-conv",
			TurnID:         "test-turn",
			UserMessage:    "What is this?",
		},
		ModelName:    "test-model",
		ProviderName: "test-provider",
		Stage0Output: &models.PerceptionOutput{
			ConversationID:  "test-conv",
			TurnID:          "test-turn",
			Language:        "en",
			Topics:          []string{"test"},
			Entities:        []string{},
			Intent:          strPtr("question"),
			Questions:       []string{"What is this?"},
			Requests:        []string{},
			Signals:         map[string]any{},
			RetrievalHinted: true,
		},
		Stage1Output: &models.RecallOutput{
			ConversationID: "test-conv",
			TurnID:         "test-turn",
			Beliefs:        []models.Belief{},
			Opinions:       []models.Opinion{},
		},
	}

	ctx := context.Background()
	output, err := stage.Execute(ctx, input)

	assert.NoError(t, err)
	assert.True(t, output.Success)

	interpretation, ok := output.Raw.(*models.InterpretationOutput)
	assert.True(t, ok)
	assert.Equal(t, "question", interpretation.Intent)
	assert.Equal(t, models.Confidence(0.8), interpretation.Confidence)
}

// TestStage3Reflection tests the reflection stage
func TestStage3Reflection(t *testing.T) {
	registry := llm.NewProviderRegistry(nil)
	mockProvider := llm.NewMockProvider()

	mockProvider.SetSequentialResponses([]string{
		`{"observations": ["User asked a question"], "learned_facts": [], "belief_updates": [], "new_beliefs": [], "opinion_updates": [], "new_opinions": [], "preference_updates": [], "goal_updates": [], "hypotheses": [], "uncertainties": [], "self_observations": [], "contradictions": [], "memory_candidates": []}`,
	})
	registry.Register("test-provider", mockProvider)

	stage := NewStage3Reflection(registry, nil)

	input := StageInput{
		PipelineInput: models.PipelineInput{
			ConversationID: "test-conv",
			TurnID:         "test-turn",
			UserMessage:    "What is this?",
		},
		ModelName:    "test-model",
		ProviderName: "test-provider",
		Stage0Output: &models.PerceptionOutput{
			ConversationID:  "test-conv",
			TurnID:          "test-turn",
			Language:        "en",
			Topics:          []string{"test"},
			Entities:        []string{},
			Intent:          strPtr("question"),
			Questions:       []string{"What is this?"},
			Requests:        []string{},
			Signals:         map[string]any{},
			RetrievalHinted: true,
		},
		Stage1Output: &models.RecallOutput{
			ConversationID: "test-conv",
			TurnID:         "test-turn",
			Beliefs:        []models.Belief{},
			Opinions:       []models.Opinion{},
		},
		Stage2Output: &models.InterpretationOutput{
			ConversationID: "test-conv",
			TurnID:         "test-turn",
			Intent:         "question",
			Goal:           "answer question",
			Confidence:     0.8,
		},
	}

	ctx := context.Background()
	output, err := stage.Execute(ctx, input)

	assert.NoError(t, err)
	assert.True(t, output.Success)

	reflection, ok := output.Raw.(*models.ReflectionOutput)
	assert.True(t, ok)
	assert.Contains(t, reflection.Observations, "User asked a question")
}

// TestStage4Response tests the response generation stage
func TestStage4Response(t *testing.T) {
	registry := llm.NewProviderRegistry(nil)
	mockProvider := llm.NewMockProvider()

	mockProvider.SetSequentialResponses([]string{
		"This is a test response.",
	})
	registry.Register("test-provider", mockProvider)

	stage := NewStage4Response(registry, nil)

	input := StageInput{
		PipelineInput: models.PipelineInput{
			ConversationID: "test-conv",
			TurnID:         "test-turn",
			UserMessage:    "Hello",
		},
		ModelName:    "test-model",
		ProviderName: "test-provider",
		Stage0Output: &models.PerceptionOutput{
			ConversationID:  "test-conv",
			TurnID:          "test-turn",
			Language:        "en",
			Topics:          []string{"greeting"},
			Entities:        []string{},
			Intent:          strPtr("greeting"),
			Questions:       []string{},
			Requests:        []string{},
			Signals:         map[string]any{},
			RetrievalHinted: false,
		},
		Stage1Output: &models.RecallOutput{
			ConversationID:   "test-conv",
			TurnID:           "test-turn",
			Beliefs:          []models.Belief{},
			Opinions:         []models.Opinion{},
			Preferences:      []models.Preference{},
			Goals:            []models.Goal{},
			Hypotheses:       []models.Hypothesis{},
			Uncertainties:    []models.Uncertainty{},
			SelfObservations: []models.SelfObservation{},
			Contradictions:   []models.Contradiction{},
		},
		Stage2Output: &models.InterpretationOutput{
			ConversationID: "test-conv",
			TurnID:         "test-turn",
			Intent:         "greeting",
			Goal:           "respond politely",
			Confidence:     0.9,
		},
		Stage3Output: &models.ReflectionOutput{
			ConversationID: "test-conv",
			TurnID:         "test-turn",
			Observations:   []string{"User greeted"},
		},
	}

	ctx := context.Background()
	output, err := stage.Execute(ctx, input)

	assert.NoError(t, err)
	assert.True(t, output.Success)

	response, ok := output.Raw.(*models.ResponseOutput)
	assert.True(t, ok)
	assert.Equal(t, "This is a test response.", response.Response)
}
