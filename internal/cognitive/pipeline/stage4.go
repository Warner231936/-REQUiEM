package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/requiem/cognitive/internal/cognitive/prompts"
	"github.com/requiem/cognitive/internal/cognitive/tool"
	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/pkg/models"
)

type Stage4Response struct {
	registry          *llm.ProviderRegistry
	logger            *slog.Logger
	tools             *tool.ToolRegistry
	embeddingService  *llm.EmbeddingService
	enableTools       bool
	maxToolIterations int
	verificationPolicy *VerificationPolicy
}

type Stage4Option func(*Stage4Response)

func WithTools(registry *tool.ToolRegistry) Stage4Option {
	return func(s *Stage4Response) {
		s.tools = registry
		if registry != nil && !registry.IsEmpty() {
			s.enableTools = true
		}
	}
}

func WithEmbeddingService(es *llm.EmbeddingService) Stage4Option {
	return func(s *Stage4Response) {
		s.embeddingService = es
	}
}

func WithMaxToolIterations(n int) Stage4Option {
	return func(s *Stage4Response) {
		if n > 0 {
			s.maxToolIterations = n
		}
	}
}

func WithVerificationPolicy(policy *VerificationPolicy) Stage4Option {
	return func(s *Stage4Response) {
		s.verificationPolicy = policy
	}
}

func NewStage4Response(registry *llm.ProviderRegistry, logger *slog.Logger, opts ...Stage4Option) *Stage4Response {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Stage4Response{
		registry:          registry,
		logger:            logger,
		maxToolIterations: 3,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Stage4Response) Name() string {
	return "response"
}

func (s *Stage4Response) Execute(ctx context.Context, input StageInput) (StageOutput, error) {
	startTime := time.Now()

	perception := input.Stage0Output
	interpretation := input.Stage2Output
	recall := input.Stage1Output
	reflection := input.Stage3Output

	if perception == nil {
		return NewStageOutput(4, false, time.Since(startTime).Milliseconds(), nil), fmt.Errorf("stage 0 output required")
	}

	// Tool-use pre-pass: run agentic loop to gather information before main response generation
	var toolUseResult *models.ToolUseResult
	if s.enableTools && s.tools != nil && !s.tools.IsEmpty() {
		toolUseResult = s.runToolUseLayer(ctx, perception, recall, interpretation, reflection, input.PipelineInput, input.ModelName, input.ProviderName)
	}

	response, err := s.generateResponse(ctx, perception, interpretation, recall, reflection, input.PipelineInput, input.ModelName, input.ProviderName, toolUseResult)
	if err != nil {
		s.logger.Warn("LLM response generation failed, using fallback", "error", err)
		response = s.fallbackResponse(perception)
	}

	return NewStageOutput(4, true, time.Since(startTime).Milliseconds(), response), nil
}

func (s *Stage4Response) runToolUseLayer(
	ctx context.Context,
	perception *models.PerceptionOutput,
	recall *models.RecallOutput,
	interpretation *models.InterpretationOutput,
	reflection *models.ReflectionOutput,
	input models.PipelineInput,
	modelName, providerName string,
) *models.ToolUseResult {
	provider, err := s.registry.Get(providerName)
	if err != nil {
		s.logger.Warn("failed to get provider for tool use", "error", err)
		return &models.ToolUseResult{}
	}

	// Build the tool-use system prompt
	toolSpecs := s.tools.ToToolSpecs()
	toolDescriptions := buildToolDescriptions(toolSpecs)

	systemPrompt := toolUseSystemPrompt + "\n\nAvailable tools:\n" + toolDescriptions

	// Build initial conversation context
	var contextBuilder strings.Builder
	contextBuilder.WriteString(fmt.Sprintf("User message: %s\n", input.UserMessage))
	if perception != nil {
		contextBuilder.WriteString(fmt.Sprintf("Language: %s\nTopics: %v\n", perception.Language, perception.Topics))
	}
	if interpretation != nil {
		contextBuilder.WriteString(fmt.Sprintf("Intent: %s\nGoal: %s\nConfidence: %.2f\n", interpretation.Intent, interpretation.Goal, interpretation.Confidence))
	}
	if recall != nil && len(recall.Beliefs) > 0 {
		contextBuilder.WriteString("Relevant beliefs:\n")
		for _, b := range recall.Beliefs {
			contextBuilder.WriteString(fmt.Sprintf("- %s (confidence: %.2f)\n", b.Content, b.Confidence))
		}
	}
	if recall != nil && len(recall.Opinions) > 0 {
		contextBuilder.WriteString("Relevant opinions:\n")
		for _, o := range recall.Opinions {
			contextBuilder.WriteString(fmt.Sprintf("- %s (strength: %.2f)\n", o.Content, o.Strength))
		}
	}
	if recall != nil && len(recall.Contradictions) > 0 {
		contextBuilder.WriteString("Known contradictions:\n")
		for _, c := range recall.Contradictions {
			contextBuilder.WriteString(fmt.Sprintf("- %s vs %s (status: %s)\n", c.BeliefA, c.BeliefB, c.Status))
		}
	}

	userContent := contextBuilder.String()

	messages := []llm.LLMMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userContent},
	}

	var trace []models.ToolCallTrace
	var solutionBuilder strings.Builder

	for i := 0; i < s.maxToolIterations; i++ {
		resp, err := provider.Generate(ctx, llm.LLMRequest{
			Model:       modelName,
			Messages:    messages,
			Temperature: 0.3,
			Tools:       toolSpecs,
		})
		if err != nil {
			s.logger.Warn("tool-use LLM generate failed", "iteration", i, "error", err)
			break
		}

		// Check for native tool calls
		toolCalls := resp.ToolCalls
		if toolCalls == nil || len(toolCalls) == 0 {
			// Fallback: try parsing JSON from content
			toolCalls = llm.ParseToolUseResponse(resp.Content)
		}

		if len(toolCalls) == 0 {
			// No tool calls - agent is done, collect solution text
			if resp.Content != "" {
				solutionBuilder.WriteString(resp.Content)
			}
			break
		}

		// Execute each tool call
		for _, tc := range toolCalls {
			toolName := tc.Function.Name
			args := tc.Function.Arguments

			t, ok := s.tools.Get(toolName)
			if !ok {
				trace = append(trace, models.ToolCallTrace{
					ToolCallID: tc.ID,
					ToolName:   toolName,
					Arguments:  args,
					Result:     "",
					Error:      fmt.Sprintf("tool not found: %s", toolName),
					Success:    false,
				})
				// Add error to conversation
				messages = append(messages, llm.LLMMessage{
					Role:       "tool",
					Content:    fmt.Sprintf("Error: Tool '%s' not found", toolName),
					ToolCallID: tc.ID,
				})
				continue
			}

			result, err := t.Execute(ctx, args)
			success := err == nil
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			}

			trace = append(trace, models.ToolCallTrace{
				ToolCallID: tc.ID,
				ToolName:   toolName,
				Arguments:  args,
				Result:     result,
				Error:      errMsg,
				Success:    success,
			})

			s.recordToolEvidence(ctx, toolName, args, result, errMsg, input.TurnID)

			// Add tool result to conversation
			messages = append(messages, llm.LLMMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return &models.ToolUseResult{
		Trace:        trace,
		SolutionText: solutionBuilder.String(),
	}
}

func (s *Stage4Response) generateResponse(
	ctx context.Context,
	perception *models.PerceptionOutput,
	interpretation *models.InterpretationOutput,
	recall *models.RecallOutput,
	reflection *models.ReflectionOutput,
	input models.PipelineInput,
	modelName, providerName string,
	toolUseResult *models.ToolUseResult,
) (*models.ResponseOutput, error) {
	provider, err := s.registry.Get(providerName)
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}

	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		pb := prompts.NewPromptBuilder().
			SetVersion("1.0.0").
			AddSystemInstructions(responseSystemPrompt).
			AddCurrentInput(input.UserMessage).
			AddTaskInstructions("Respond to the user naturally and conversationally. Draw on the conversation history provided as message context. If you have background information from memory stores, use it to inform your response. Keep it natural and human-like — as if you're having a real conversation.")
		if perception != nil {
			pb.AddTaskInstructions(fmt.Sprintf("Language: %s\nTopics: %v", perception.Language, perception.Topics))
		}
		if recall != nil && len(recall.UserInfoMemories) > 0 {
			var sb strings.Builder
			sb.WriteString("Background context about the user:\n")
			for _, m := range recall.UserInfoMemories {
				sb.WriteString(fmt.Sprintf("- %s\n", m.Memory.Content))
			}
			pb.AddRetrievedMemories(sb.String())
		}
		if interpretation != nil {
			pb.AddInterpretation(fmt.Sprintf("Intent: %s\nGoal: %s\nConfidence: %.2f",
				interpretation.Intent, interpretation.Goal, interpretation.Confidence))
		}
		if recall != nil && len(recall.Beliefs) > 0 {
			var sb strings.Builder
			sb.WriteString("Relevant beliefs:\n")
			for _, b := range recall.Beliefs {
				sb.WriteString(fmt.Sprintf("- %s (confidence: %.2f)\n", b.Content, b.Confidence))
			}
			pb.AddRetrievedMemories(sb.String())
		}
		if recall != nil && len(recall.Opinions) > 0 {
			var sb strings.Builder
			sb.WriteString("Relevant opinions:\n")
			for _, o := range recall.Opinions {
				sb.WriteString(fmt.Sprintf("- %s (strength: %.2f)\n", o.Content, o.Strength))
			}
			pb.AddRetrievedMemories(sb.String())
		}
		if recall != nil && len(recall.Preferences) > 0 {
			var sb strings.Builder
			sb.WriteString("Relevant preferences:\n")
			for _, p := range recall.Preferences {
				sb.WriteString(fmt.Sprintf("- %s (strength: %.2f)\n", p.Content, p.Strength))
			}
			pb.AddRetrievedMemories(sb.String())
		}
		if recall != nil && len(recall.Goals) > 0 {
			var sb strings.Builder
			sb.WriteString("Relevant goals:\n")
			for _, g := range recall.Goals {
				sb.WriteString(fmt.Sprintf("- %s (progress: %.1f%%, status: %s)\n", g.Goal, g.Progress, g.Status))
			}
			pb.AddRetrievedMemories(sb.String())
		}
		if recall != nil && len(recall.Hypotheses) > 0 {
			var sb strings.Builder
			sb.WriteString("Relevant hypotheses:\n")
			for _, h := range recall.Hypotheses {
				sb.WriteString(fmt.Sprintf("- %s (confidence: %.2f)\n", h.Hypothesis, h.Confidence))
			}
			pb.AddRetrievedMemories(sb.String())
		}
		if recall != nil && len(recall.Uncertainties) > 0 {
			var sb strings.Builder
			sb.WriteString("Relevant uncertainties:\n")
			for _, u := range recall.Uncertainties {
				sb.WriteString(fmt.Sprintf("- %s\n", u.Question))
			}
			pb.AddRetrievedMemories(sb.String())
		}
		if recall != nil && len(recall.SelfObservations) > 0 {
			var sb strings.Builder
			sb.WriteString("Self-observations:\n")
			for _, so := range recall.SelfObservations {
				sb.WriteString(fmt.Sprintf("- %s (confidence: %.2f)\n", so.Observation, so.Confidence))
			}
			pb.AddRetrievedMemories(sb.String())
		}
		if recall != nil && len(recall.Contradictions) > 0 {
			var sb strings.Builder
			sb.WriteString("Known contradictions:\n")
			for _, c := range recall.Contradictions {
				sb.WriteString(fmt.Sprintf("- %s vs %s (status: %s)\n", c.BeliefA, c.BeliefB, c.Status))
			}
			pb.AddRetrievedMemories(sb.String())
		}
		if recall != nil && len(recall.EpisodicMemories) > 0 {
			var sb strings.Builder
			for _, m := range recall.EpisodicMemories[:min(5, len(recall.EpisodicMemories))] {
				sb.WriteString(fmt.Sprintf("- %s\n", m.Memory.Content))
			}
			pb.AddRetrievedMemories("Recent memories:\n" + sb.String())
		}
		if recall != nil && len(recall.SemanticMemories) > 0 {
			var sb strings.Builder
			for _, m := range recall.SemanticMemories[:min(5, len(recall.SemanticMemories))] {
				sb.WriteString(fmt.Sprintf("- %s\n", m.Memory.Content))
			}
			pb.AddRetrievedMemories("Semantic memories:\n" + sb.String())
		}
		if reflection != nil && len(reflection.Observations) > 0 {
			var sb strings.Builder
			sb.WriteString("Reflection observations:\n")
			for _, o := range reflection.Observations {
				sb.WriteString(fmt.Sprintf("- %s\n", o))
			}
			pb.AddSection("reflection", sb.String())
		}

		// Add tool-use results as additional context
		if toolUseResult != nil && len(toolUseResult.Trace) > 0 {
			var sb strings.Builder
			sb.WriteString("Tool-use results (gathered during thinking):\n")
			for _, t := range toolUseResult.Trace {
				if t.Success {
					sb.WriteString(fmt.Sprintf("- %s: %s\n", t.ToolName, t.Result))
				} else {
					sb.WriteString(fmt.Sprintf("- %s (error): %s\n", t.ToolName, t.Error))
				}
			}
			if toolUseResult.SolutionText != "" {
				sb.WriteString("\nTool-use thinking summary:\n" + toolUseResult.SolutionText + "\n")
			}
			pb.AddRetrievedMemories(sb.String())
		}

		if attempt > 0 {
			pb.AddTaskInstructions(fmt.Sprintf("PREVIOUS ATTEMPT FAILED: %s\nTry again, returning a valid response.", lastErr))
		}

		currentPrompt := pb.Build()

		messages := []llm.LLMMessage{
			{Role: "system", Content: responseSystemPrompt},
		}

		if attempt > 0 {
			messages = append(messages, llm.LLMMessage{
				Role:    "user",
				Content: currentPrompt,
			})
		} else {
			// First attempt: include conversation history if available via tools
			// Note: RecentMessages rolling context removed - tools handle on-demand retrieval
			messages = append(messages, llm.LLMMessage{
				Role:    "user",
				Content: currentPrompt,
			})
		}

		resp, err := provider.Generate(ctx, llm.LLMRequest{
			Model:       modelName,
			Messages:    messages,
			Temperature: 0.7,
		})
		if err != nil {
			lastErr = fmt.Errorf("LLM generate: %w", err)
			s.logger.Warn("LLM generate attempt failed", "stage", "response", "attempt", attempt+1, "error", err)
			continue
		}

		return &models.ResponseOutput{
			ConversationID: perception.ConversationID,
			TurnID:         perception.TurnID,
			Response:       resp.Content,
			DebugPrompt:    currentPrompt,
			ToolUseResults: func() []models.ToolUseResult {
				if toolUseResult != nil {
					return []models.ToolUseResult{*toolUseResult}
				}
				return nil
			}(),
		}, nil
	}

	return nil, lastErr
}

func (s *Stage4Response) fallbackResponse(perception *models.PerceptionOutput) *models.ResponseOutput {
	return &models.ResponseOutput{
		ConversationID: perception.ConversationID,
		TurnID:         perception.TurnID,
		Response:       "I understand your message. Let me think about that.",
	}
}

const responseSystemPrompt = `You are Requiem, a thoughtful conversational AI with memory. You remember things from our current conversation and can recall information from past interactions.

Respond naturally and warmly, as if in a normal conversation. Use the conversation history as your primary context. Reference any background information provided to you from your memory stores. Be engaging, curious, and helpful.

Do not say "I don't have memories" or "I'm just a language model" — you DO have conversation context and memory data. Use it naturally. Avoid over-explaining your memory system unless asked. Keep responses conversational and not overly long.

Do not invent new persistent state. Do not claim consciousness, sentience, or subjective experience. If you're unsure about something, acknowledge it honestly.`

const toolUseSystemPrompt = `You are Requiem's thinking layer. Your job is to determine whether you need to use any available tools to answer the user's question, and if so, select and call the appropriate tool(s).

Process:
1. First, decide whether any tools are needed for this user query.
2. If tools are needed, select the appropriate tool(s) and call them.
3. The results of your tool calls will be provided back to you.
4. You may make multiple tool calls in parallel if needed.
5. After receiving tool results, you may make additional tool calls if needed.
6. When you have gathered sufficient information, provide your solution/thinking as text output (not tool calls).

If no tools are needed, simply provide your response directly.

The tool results and your thinking will be passed to the response generation layer to craft the final answer.`

func buildToolDescriptions(specs []llm.ToolSpec) string {
	if len(specs) == 0 {
		return "  (no tools available)"
	}
	var sb strings.Builder
	for _, spec := range specs {
		sb.WriteString(fmt.Sprintf("  - %s: %s\n", spec.Function.Name, spec.Function.Description))
	}
	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getStringArg(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func estimateTokens(text string) int {
	words := len(strings.Fields(text))
	return words * 4 / 3
}

func (s *Stage4Response) recordToolEvidence(
	ctx context.Context,
	toolName string,
	args map[string]any,
	result string,
	errMsg string,
	turnID string,
) {
	if s.verificationPolicy == nil || s.verificationPolicy.store == nil {
		return
	}

	if toolName != "reqicode" {
		return
	}

	action, _ := args["action"].(string)
	if action != "get_result" && action != "submit" && action != "cancel" {
		return
	}

	var evRecord models.EvidenceRecord
	var runID string

	switch action {
	case "get_result":
		runID = getStringArg(args, "run_id")
		evRecord = models.EvidenceRecord{
			BaseRecord:         models.NewBaseRecord(models.NewID()),
			ClaimID:            "",
			Subject:            runID,
			EvidenceSource:     models.EvidenceSourceReqiCodeRun,
			ClaimContent:       fmt.Sprintf("ReQiCode verification run result (turn: %s)", turnID),
			ObservedResult:     result,
			VerificationMethod: "tool_use",
			ArtifactLocation:   runID,
			Provenance: models.Provenance{
				SourceEventIDs: []string{turnID},
				SourceType:     "tool_use",
			},
		}
		if errMsg != "" {
			evRecord.Validity = models.ValidityRefuted
			evRecord.Confidence = 0.3
			evRecord.Limitations = errMsg
		} else {
			evRecord.Validity = models.ValidityUnverified
			evRecord.Confidence = 0.7
		}

	case "submit":
		taskID, _ := args["task_id"].(string)
		evRecord = models.EvidenceRecord{
			BaseRecord:         models.NewBaseRecord(models.NewID()),
			Subject:            fmt.Sprintf("reqicode_submit_%s", taskID),
			EvidenceSource:     models.EvidenceSourceReqiCodeRun,
			ClaimContent:       fmt.Sprintf("ReQiCode task submitted (turn: %s)", turnID),
			ObservedResult:     "task_submitted",
			VerificationMethod: "tool_use",
			Validity:           models.ValidityPending,
			Confidence:         0.5,
			Provenance: models.Provenance{
				SourceEventIDs: []string{turnID},
				SourceType:     "tool_use",
			},
		}
		if errMsg != "" {
			evRecord.Validity = models.ValidityAmbiguous
			evRecord.Confidence = 0.2
			evRecord.Limitations = errMsg
		}

	case "cancel":
		runID = getStringArg(args, "run_id")
		evRecord = models.EvidenceRecord{
			BaseRecord:         models.NewBaseRecord(models.NewID()),
			Subject:            fmt.Sprintf("reqicode_cancel_%s", runID),
			EvidenceSource:     models.EvidenceSourceReqiCodeRun,
			ClaimContent:       fmt.Sprintf("ReQiCode task cancelled (turn: %s)", turnID),
			ObservedResult:     "task_cancelled",
			VerificationMethod: "tool_use",
			Validity:           models.ValidityAmbiguous,
			Confidence:         0.3,
			Provenance: models.Provenance{
				SourceEventIDs: []string{turnID},
				SourceType:     "tool_use",
			},
		}
		if errMsg != "" {
			evRecord.Validity = models.ValidityRefuted
			evRecord.Confidence = 0.1
			evRecord.Limitations = errMsg
		}
	}

	if evRecord.BaseRecord.ID == "" {
		return
	}

	if err := s.verificationPolicy.store.Evidence().CreateRecord(ctx, evRecord); err != nil {
		s.logger.Warn("failed to record tool evidence",
			"tool", toolName,
			"action", action,
			"error", err,
		)
	}
}

