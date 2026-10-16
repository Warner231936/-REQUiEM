package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/requiem/cognitive/internal/cognitive/prompts"
	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/pkg/models"
)

type Stage5ResponseHumanize struct {
	registry *llm.ProviderRegistry
	logger   *slog.Logger
}

func NewStage5ResponseHumanize(registry *llm.ProviderRegistry, logger *slog.Logger) *Stage5ResponseHumanize {
	if logger == nil {
		logger = slog.Default()
	}
	return &Stage5ResponseHumanize{
		registry: registry,
		logger:   logger,
	}
}

func (s *Stage5ResponseHumanize) Name() string {
	return "response_humanize"
}

func (s *Stage5ResponseHumanize) Execute(ctx context.Context, input StageInput) (StageOutput, error) {
	startTime := time.Now()

	responseOutput := input.Stage4Output
	if responseOutput == nil || responseOutput.Response == "" {
		return NewStageOutput(5, false, time.Since(startTime).Milliseconds(), nil), fmt.Errorf("stage 4 output required")
	}

	humanizedResponse, err := s.humanizeResponse(ctx, responseOutput, input.PipelineInput, input.ModelName, input.ProviderName)
	if err != nil {
		s.logger.Warn("LLM humanization failed, returning original response", "error", err)
		return NewStageOutput(5, true, time.Since(startTime).Milliseconds(), responseOutput), nil
	}

	return NewStageOutput(5, true, time.Since(startTime).Milliseconds(), humanizedResponse), nil
}

func (s *Stage5ResponseHumanize) humanizeResponse(
	ctx context.Context,
	responseOutput *models.ResponseOutput,
	input models.PipelineInput,
	modelName, providerName string,
) (*models.ResponseOutput, error) {
	provider, err := s.registry.Get(providerName)
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}

	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		pb := prompts.NewPromptBuilder().
			SetVersion("1.0.0").
			AddSystemInstructions(humanizeSystemPrompt).
			AddCurrentInput(responseOutput.Response).
			AddTaskInstructions("Summarize and humanize the response below. Keep the core meaning intact but make it sound more natural, concise, and conversational. Remove any redundant or overly verbose phrasing. If the response is already natural, keep it as-is. Output only the refined response text, without any labels or markup.")

		if attempt > 0 {
			pb.AddTaskInstructions(fmt.Sprintf("PREVIOUS ATTEMPT FAILED: %s\nTry again, returning only the refined response text.", lastErr))
		}

		currentPrompt := pb.Build()

		messages := []llm.LLMMessage{
			{Role: "system", Content: humanizeSystemPrompt},
			{Role: "user", Content: currentPrompt},
		}

		resp, err := provider.Generate(ctx, llm.LLMRequest{
			Model:       modelName,
			Messages:    messages,
			Temperature: 0.3,
		})
		if err != nil {
			lastErr = fmt.Errorf("LLM generate: %w", err)
			s.logger.Warn("LLM humanize attempt failed", "stage", "response_humanize", "attempt", attempt+1, "error", err)
			continue
		}

		return &models.ResponseOutput{
			ConversationID: responseOutput.ConversationID,
			TurnID:         responseOutput.TurnID,
			Response:       resp.Content,
			DebugPrompt:    currentPrompt,
		}, nil
	}

	return nil, lastErr
}

const humanizeSystemPrompt = `You are a response refinement assistant. Your job is to take a raw AI response and make it more natural, concise, and human-sounding while preserving the original meaning. Do not invent new facts. Output only the refined text.`
