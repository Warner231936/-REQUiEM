package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/requiem/cognitive/internal/cognitive/prompts"
	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/internal/validation"
	"github.com/requiem/cognitive/pkg/models"
)

type Stage0Perception struct {
	registry  *llm.ProviderRegistry
	sanitizer *validation.OutputSanitizer
	validator *validation.MemoryImportValidator
	logger    *slog.Logger
}

func NewStage0Perception(registry *llm.ProviderRegistry, logger *slog.Logger) *Stage0Perception {
	if logger == nil {
		logger = slog.Default()
	}
	return &Stage0Perception{
		registry:  registry,
		sanitizer: validation.NewOutputSanitizer(),
		validator: validation.NewMemoryImportValidator(),
		logger:    logger,
	}
}

func (s *Stage0Perception) Name() string {
	return "perception"
}

func (s *Stage0Perception) Execute(ctx context.Context, input StageInput) (StageOutput, error) {
	startTime := time.Now()

	if err := s.validator.ValidateMemoryContent(input.PipelineInput.UserMessage); err != nil {
		s.logger.Warn("input validation failed", "error", err)
		return StageOutput{}, fmt.Errorf("perception: input validation failed: %w", err)
	}

	if err := s.validator.ValidateMemoryContent(input.PipelineInput.UserMessage); err != nil {
		s.logger.Warn("pipeline input validation failed", "error", err)
		return StageOutput{}, fmt.Errorf("perception: pipeline input validation failed: %w", err)
	}

	perception, err := s.perceive(ctx, input.PipelineInput, input.ModelName, input.ProviderName)
	if err != nil {
		s.logger.Warn("LLM perception failed, using fallback", "error", err)
		perception = s.fallbackPerception(input.PipelineInput)
	}

	if err := validatePerceptionOutput(perception); err != nil {
		return StageOutput{}, fmt.Errorf("perception validation failed: %w", err)
	}

	perception.Timestamp = startTime.UTC()

	return NewStageOutput(0, true, time.Since(startTime).Milliseconds(), perception), nil
}

func (s *Stage0Perception) perceive(ctx context.Context, input models.PipelineInput, modelName, providerName string) (*models.PerceptionOutput, error) {
	provider, err := s.registry.Get(providerName)
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}

	sanitizedInput := s.sanitizer.SanitizeString(input.UserMessage)

	pb := prompts.NewPromptBuilder().
		SetVersion("1.0.0").
		AddSystemInstructions(perceptionSystemPrompt).
		AddCurrentInput(sanitizedInput).
		AddTaskInstructions(fmt.Sprintf("Return JSON with: language, topics, entities, intent, questions, requests, signals, retrieval_hinted. Return ONLY valid JSON."))

	prompt := pb.Build()

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			pb = prompts.NewPromptBuilder().
				SetVersion("1.0.0").
				AddSystemInstructions(perceptionSystemPrompt).
				AddCurrentInput(sanitizedInput).
				AddTaskInstructions(fmt.Sprintf("PREVIOUS ATTEMPT FAILED WITH ERROR: %s\nReturn ONLY valid JSON. Do not add any text outside JSON.", lastErr))
			prompt = pb.Build()
		}

		resp, err := provider.Generate(ctx, llm.LLMRequest{
			Model: modelName,
			Messages: []llm.LLMMessage{
				{Role: "system", Content: perceptionSystemPrompt},
				{Role: "user", Content: prompt},
			},
			Temperature: 0.1,
			Format:      "json",
		})
		if err != nil {
			lastErr = fmt.Errorf("LLM generate: %w", err)
			s.logger.Warn("LLM generate attempt failed", "attempt", attempt+1, "error", err)
			continue
		}

		var output models.PerceptionOutput
		if err := llm.ParseJSONResponse(resp.Content, &output); err != nil {
			lastErr = fmt.Errorf("parse perception output: %w", err)
			s.logger.Warn("JSON parse attempt failed", "attempt", attempt+1, "error", err)
			continue
		}

		output.ConversationID = input.ConversationID
		output.TurnID = input.TurnID
		output.RawInput = sanitizedInput
		output.DebugPrompt = prompt

		return &output, nil
	}

	return nil, lastErr
}

func (s *Stage0Perception) fallbackPerception(input models.PipelineInput) *models.PerceptionOutput {
	now := time.Now().UTC()
	output := &models.PerceptionOutput{
		ConversationID:  input.ConversationID,
		TurnID:          input.TurnID,
		RawInput:        input.UserMessage,
		NormalizedInput: strings.TrimSpace(input.UserMessage),
		Language:        detectLanguage(input.UserMessage),
		Topics:          extractTopics(input.UserMessage),
		Entities:        extractEntities(input.UserMessage),
		Questions:       extractQuestions(input.UserMessage),
		Requests:        []string{},
		Signals: map[string]any{
			"fallback": true,
		},
		Timestamp:       now,
		RetrievalHinted: len(input.UserMessage) > 20,
	}

	intent := detectIntent(input.UserMessage)
	output.Intent = &intent

	return output
}

const perceptionSystemPrompt = `You are a perception module for a cognitive system.
Your task is to normalize and analyze user input.
Do not interpret deeply or reason about the content.
Focus on surface-level extraction: language, topics, entities, questions, and requests.
Return structured JSON only.`

func validatePerceptionOutput(output *models.PerceptionOutput) error {
	if output.ConversationID == "" {
		output.ConversationID = uuid.New().String()
	}
	if output.TurnID == "" {
		output.TurnID = uuid.New().String()
	}
	if output.Language == "" {
		output.Language = "en"
	}
	if output.NormalizedInput == "" {
		output.NormalizedInput = output.RawInput
	}
	return nil
}

func detectLanguage(text string) string {
	if len(text) == 0 {
		return "en"
	}
	for _, r := range text {
		if r > 0x4E00 && r < 0x9FFF {
			return "zh"
		}
		if r > 0x3040 && r < 0x30FF {
			return "ja"
		}
		if r > 0xAC00 && r < 0xD7AF {
			return "ko"
		}
	}
	return "en"
}

func extractTopics(text string) []string {
	topics := []string{}
	textLower := strings.ToLower(text)

	topicKeywords := map[string][]string{
		"architecture": {"architecture", "design", "structure", "system"},
		"memory":       {"memory", "remember", "recall", "forget"},
		"preference":   {"prefer", "like", "want", "favor"},
		"technical":    {"code", "programming", "software", "database", "api"},
		"cognitive":    {"think", "believe", "opinion", "reasoning"},
	}

	for topic, keywords := range topicKeywords {
		for _, kw := range keywords {
			if strings.Contains(textLower, kw) {
				topics = append(topics, topic)
				break
			}
		}
	}

	return topics
}

func extractEntities(text string) []string {
	entities := []string{}
	words := strings.Fields(text)
	for _, word := range words {
		if len(word) > 1 && word[0] >= 'A' && word[0] <= 'Z' {
			entities = append(entities, strings.TrimRight(word, ".,;:!?"))
		}
	}
	return entities
}

func extractQuestions(text string) []string {
	questions := []string{}
	sentences := strings.Split(text, "?")
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if strings.Contains(s, "?") || strings.HasPrefix(strings.ToLower(s), "what") ||
			strings.HasPrefix(strings.ToLower(s), "how") ||
			strings.HasPrefix(strings.ToLower(s), "why") ||
			strings.HasPrefix(strings.ToLower(s), "when") ||
			strings.HasPrefix(strings.ToLower(s), "where") ||
			strings.HasPrefix(strings.ToLower(s), "who") {
			questions = append(questions, s)
		}
	}
	return questions
}

func detectIntent(text string) string {
	textLower := strings.ToLower(text)
	if strings.Contains(textLower, "?") {
		return "question"
	}
	if strings.Contains(textLower, "please") || strings.Contains(textLower, "can you") {
		return "request"
	}
	if strings.Contains(textLower, "i think") || strings.Contains(textLower, "i believe") {
		return "opinion_expression"
	}
	if strings.Contains(textLower, "i prefer") || strings.Contains(textLower, "i like") {
		return "preference_expression"
	}
	return "statement"
}
