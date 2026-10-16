package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/requiem/cognitive/internal/cognitive/prompts"
	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/pkg/models"
)

type Stage2Interpretation struct {
	registry *llm.ProviderRegistry
	logger   *slog.Logger
}

func NewStage2Interpretation(registry *llm.ProviderRegistry, logger *slog.Logger) *Stage2Interpretation {
	if logger == nil {
		logger = slog.Default()
	}
	return &Stage2Interpretation{
		registry: registry,
		logger:   logger,
	}
}

func (s *Stage2Interpretation) Name() string {
	return "interpretation"
}

func (s *Stage2Interpretation) Execute(ctx context.Context, input StageInput) (StageOutput, error) {
	startTime := time.Now()

	perception := input.Stage0Output
	recall := input.Stage1Output

	if perception == nil {
		return StageOutput{}, fmt.Errorf("stage 0 output required for interpretation")
	}

	interpretation, err := s.interpret(ctx, perception, recall, input.PipelineInput, input.ModelName, input.ProviderName)
	if err != nil {
		s.logger.Warn("LLM interpretation failed, using fallback", "error", err)
		interpretation = s.fallbackInterpretation(perception, recall)
	}

	if err := validateInterpretationOutput(interpretation); err != nil {
		return StageOutput{}, fmt.Errorf("interpretation validation failed: %w", err)
	}

	return NewStageOutput(2, true, time.Since(startTime).Milliseconds(), interpretation), nil
}

func (s *Stage2Interpretation) interpret(
	ctx context.Context,
	perception *models.PerceptionOutput,
	recall *models.RecallOutput,
	input models.PipelineInput,
	modelName, providerName string,
) (*models.InterpretationOutput, error) {
	provider, err := s.registry.Get(providerName)
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}

	var lastErr error
	var prompt string

	for attempt := 0; attempt < 3; attempt++ {
		pb := prompts.NewPromptBuilder().
			SetVersion("1.0.0").
			AddSystemInstructions(interpretationSystemPrompt).
			AddCurrentInput(input.UserMessage).
			AddTaskInstructions("Determine what is happening. Distinguish OBSERVATION, INFERENCE, ASSUMPTION, BELIEF, UNCERTAINTY. Return ONLY valid JSON.")

		if recall != nil && len(recall.Beliefs) > 0 {
			var sb strings.Builder
			for _, b := range recall.Beliefs {
				sb.WriteString(fmt.Sprintf("- %s (confidence: %.2f)\n", b.Content, b.Confidence))
			}
			pb.AddRetrievedMemories("Retrieved beliefs:\n" + sb.String())
		}
		if recall != nil && (len(recall.EpisodicMemories) > 0 || len(recall.SemanticMemories) > 0) {
			var sb strings.Builder
			for _, m := range recall.EpisodicMemories {
				sb.WriteString(fmt.Sprintf("- [episodic] %s\n", m.Memory.Content))
			}
			for _, m := range recall.SemanticMemories {
				sb.WriteString(fmt.Sprintf("- [semantic] %s\n", m.Memory.Content))
			}
			pb.AddRetrievedMemories("Retrieved memories:\n" + sb.String())
		}

		if perception != nil {
			pb.AddInterpretation(fmt.Sprintf("Language: %s\nTopics: %v\nEntities: %v\nIntent: %v\nQuestions: %v",
				perception.Language, perception.Topics, perception.Entities, perception.Intent,
				perception.Questions))
		}

		if attempt > 0 {
			pb.AddTaskInstructions(fmt.Sprintf("PREVIOUS ATTEMPT FAILED: %s\nReturn ONLY valid JSON. Do not add any text outside JSON.", lastErr))
		}

		prompt = pb.Build()

		resp, err := provider.Generate(ctx, llm.LLMRequest{
			Model: modelName,
			Messages: []llm.LLMMessage{
				{Role: "system", Content: interpretationSystemPrompt},
				{Role: "user", Content: prompt},
			},
			Temperature: 0.2,
			Format:      "json",
		})
		if err != nil {
			lastErr = fmt.Errorf("LLM generate: %w", err)
			s.logger.Warn("LLM generate attempt failed", "stage", "interpretation", "attempt", attempt+1, "error", err)
			continue
		}

		var output models.InterpretationOutput
		if err := llm.ParseJSONResponse(resp.Content, &output); err != nil {
			lastErr = fmt.Errorf("parse interpretation output: %w", err)
			s.logger.Warn("JSON parse attempt failed", "stage", "interpretation", "attempt", attempt+1, "error", err)
			continue
		}

		output.ConversationID = perception.ConversationID
		output.TurnID = perception.TurnID
		output.DebugPrompt = prompt

		return &output, nil
	}

	return nil, lastErr
}

func (s *Stage2Interpretation) fallbackInterception(perception *models.PerceptionOutput, recall *models.RecallOutput) *models.InterpretationOutput {
	return s.fallbackInterpretation(perception, recall)
}

func (s *Stage2Interpretation) fallbackInterpretation(perception *models.PerceptionOutput, recall *models.RecallOutput) *models.InterpretationOutput {
	intent := "unknown"
	if perception.Intent != nil {
		intent = *perception.Intent
	}

	relevantBeliefs := []string{}
	relevantMemories := []string{}
	if recall != nil {
		relevantBeliefs = make([]string, 0, len(recall.Beliefs))
		for _, b := range recall.Beliefs {
			relevantBeliefs = append(relevantBeliefs, b.Content)
		}

		relevantMemories = make([]string, 0, len(recall.EpisodicMemories))
		for _, m := range recall.EpisodicMemories {
			relevantMemories = append(relevantMemories, m.Memory.Content)
		}
	}

	return &models.InterpretationOutput{
		ConversationID:   perception.ConversationID,
		TurnID:           perception.TurnID,
		Intent:           intent,
		Goal:             "respond to user input",
		Context:          perception.Topics,
		RelevantBeliefs:  relevantBeliefs,
		RelevantMemories: relevantMemories,
		Confidence:       0.5,
	}
}

const interpretationSystemPrompt = `You are an interpretation module for a cognitive system.
Your task is to determine what is happening in an interaction.
Distinguish carefully between observations, inferences, assumptions, beliefs, and uncertainties.
Never collapse these categories into one.
Return structured JSON only.`

func validateInterpretationOutput(output *models.InterpretationOutput) error {
	if output.Intent == "" {
		output.Intent = "unknown"
	}
	if output.Goal == "" {
		output.Goal = "respond to user"
	}
	if output.Confidence < 0 || output.Confidence > 1.0 {
		output.Confidence = 0.5
	}
	return nil
}
