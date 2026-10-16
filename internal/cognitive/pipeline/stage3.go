package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/requiem/cognitive/internal/cognitive/prompts"
	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/pkg/models"
)

type Stage3Reflection struct {
	registry *llm.ProviderRegistry
	logger   *slog.Logger
}

func NewStage3Reflection(registry *llm.ProviderRegistry, logger *slog.Logger) *Stage3Reflection {
	if logger == nil {
		logger = slog.Default()
	}
	return &Stage3Reflection{
		registry: registry,
		logger:   logger,
	}
}

func (s *Stage3Reflection) Name() string {
	return "reflection"
}

func (s *Stage3Reflection) Execute(ctx context.Context, input StageInput) (StageOutput, error) {
	startTime := time.Now()

	perception := input.Stage0Output
	recall := input.Stage1Output
	interpretation := input.Stage2Output

	if perception == nil || interpretation == nil {
		return NewStageOutput(3, false, time.Since(startTime).Milliseconds(), nil), fmt.Errorf("stage 0 and 2 outputs required")
	}

	reflection, err := s.reflect(ctx, perception, recall, interpretation, input.PipelineInput, input.ModelName, input.ProviderName)
	if err != nil {
		s.logger.Warn("LLM reflection failed, using fallback", "error", err)
		reflection = s.fallbackReflection(perception, interpretation)
	}

	if reflection.ConversationID == "" {
		reflection.ConversationID = perception.ConversationID
	}
	if reflection.TurnID == "" {
		reflection.TurnID = perception.TurnID
	}

	return NewStageOutput(3, true, time.Since(startTime).Milliseconds(), reflection), nil
}

func (s *Stage3Reflection) reflect(
	ctx context.Context,
	perception *models.PerceptionOutput,
	recall *models.RecallOutput,
	interpretation *models.InterpretationOutput,
	input models.PipelineInput,
	modelName, providerName string,
) (*models.ReflectionOutput, error) {
	provider, err := s.registry.Get(providerName)
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}

	var lastErr error
	var prompt string

	for attempt := 0; attempt < 3; attempt++ {
		pb := prompts.NewPromptBuilder().
			SetVersion("1.0.0").
			AddSystemInstructions(reflectionSystemPrompt).
			AddCurrentInput(input.UserMessage).
			AddTaskInstructions("Reflect on this interaction and propose cognitive updates. Return ONLY valid JSON.\nExtract user-specific facts (name, preferences, interests, etc.) as memory_candidates with tag 'user_info' so they can be recalled in future turns.")

		if interpretation != nil {
			pb.AddInterpretation(fmt.Sprintf("Intent: %s\nGoal: %s\nConfidence: %.2f",
				interpretation.Intent, interpretation.Goal, interpretation.Confidence))
		}
		if perception != nil {
			pb.AddInterpretation(fmt.Sprintf("Language: %s\nTopics: %v\nEntities: %v", perception.Language, perception.Topics, perception.Entities))
		}
		if recall != nil && len(recall.Beliefs) > 0 {
			var sb strings.Builder
			for _, b := range recall.Beliefs {
				sb.WriteString(fmt.Sprintf("- %s (confidence: %.2f)\n", b.Content, b.Confidence))
			}
			pb.AddRetrievedMemories("Current beliefs:\n" + sb.String())
		}
		if recall != nil && len(recall.Opinions) > 0 {
			var sb strings.Builder
			for _, o := range recall.Opinions {
				sb.WriteString(fmt.Sprintf("- %s (strength: %.2f)\n", o.Content, o.Strength))
			}
			pb.AddRetrievedMemories("Current opinions:\n" + sb.String())
		}

		if attempt > 0 {
			pb.AddTaskInstructions(fmt.Sprintf("PREVIOUS ATTEMPT FAILED: %s\nReturn ONLY valid JSON. Do not add any text outside JSON.", lastErr))
		}

		prompt = pb.Build()

		resp, err := provider.Generate(ctx, llm.LLMRequest{
			Model: modelName,
			Messages: []llm.LLMMessage{
				{Role: "system", Content: reflectionSystemPrompt},
				{Role: "user", Content: prompt},
			},
			Temperature: 0.3,
			Format:      "json",
		})
		if err != nil {
			lastErr = fmt.Errorf("LLM generate: %w", err)
			s.logger.Warn("LLM generate attempt failed", "stage", "reflection", "attempt", attempt+1, "error", err)
			continue
		}

		var output models.ReflectionOutput
		normalized := normalizeReflectionJSON(resp.Content)
		if err := llm.ParseJSONResponse(normalized, &output); err != nil {
			if rawErr := llm.ParseJSONResponse(resp.Content, &output); rawErr != nil {
				lastErr = fmt.Errorf("parse reflection output: %w", err)
				s.logger.Warn("JSON parse attempt failed", "stage", "reflection", "attempt", attempt+1, "error", err)
				continue
			}
		}

		output.ConversationID = perception.ConversationID
		output.TurnID = perception.TurnID
		output.DebugPrompt = prompt

		return &output, nil
	}

	return nil, lastErr
}

func normalizeReflectionJSON(content string) string {
	var raw map[string]any
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return content
	}

	normalizeItemArray := func(items any) any {
		if arr, ok := items.([]any); ok {
			for i, item := range arr {
				if m, ok := item.(map[string]any); ok {
					if v, exists := m["strength"]; exists {
						if _, has := m["confidence"]; !has {
							m["confidence"] = v
						}
					}
					if v, exists := m["belief"]; exists {
						if _, has := m["content"]; !has {
							m["content"] = v
						}
					}
					if _, has := m["question"]; !has {
						if reason, ok := m["reason"].(string); ok && reason != "" {
							m["question"] = reason
						} else if content, ok := m["content"].(string); ok && content != "" {
							m["question"] = content
						}
					}
					arr[i] = m
				}
			}
			return arr
		}
		return items
	}

	for _, key := range []string{"new_beliefs", "learned_facts", "memory_candidates", "hypotheses", "new_opinions"} {
		if v, ok := raw[key]; ok {
			raw[key] = normalizeItemArray(v)
		}
	}

	if unc, ok := raw["uncertainties"].([]any); ok {
		for i, item := range unc {
			if m, ok := item.(map[string]any); ok {
				if v, exists := m["strength"]; exists {
					if _, has := m["confidence"]; !has {
						m["confidence"] = v
					}
				}
				if _, has := m["question"]; !has {
					if reason, ok := m["reason"].(string); ok && reason != "" {
						m["question"] = reason
					} else {
						m["question"] = "What is the user uncertain about?"
					}
				}
				if m["confidence"] == nil {
					m["confidence"] = 0.5
				}
				unc[i] = m
			} else {
				unc[i] = map[string]any{
					"question":   fmt.Sprintf("Uncertainty value: %v", item),
					"confidence": 0.5,
				}
			}
		}
		raw["uncertainties"] = unc
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return content
	}
	return string(b)
}

func (s *Stage3Reflection) fallbackReflection(perception *models.PerceptionOutput, interpretation *models.InterpretationOutput) *models.ReflectionOutput {
	return &models.ReflectionOutput{
		ConversationID: perception.ConversationID,
		TurnID:         perception.TurnID,
		Observations: []string{
			fmt.Sprintf("User expressed: %s", perception.NormalizedInput),
		},
		Confidence: 0.5,
	}
}

const reflectionSystemPrompt = `You are a reflection module for a cognitive system.
Your task is to propose cognitive updates based on interactions.
You MUST NOT directly produce final state. Propose changes only.
Distinguish carefully between facts, opinions, beliefs, hypotheses, and uncertainties.
Opinions must be evidence-linked and contextual.
Never manufacture opinions merely to appear autonomous.
If no meaningful opinion can be justified, return empty arrays.
CRITICAL: Extract user-specific facts (name, preferences, interests, topics they discuss) as memory_candidates with tag "user_info" so they persist for future recall.
Return structured JSON only.`
