package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/requiem/cognitive/internal/events"
	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

type Pipeline struct {
	config   models.PipelineConfig
	registry *llm.ProviderRegistry
	storage  *storage.MongoStore
	logger   *slog.Logger
	stages   map[int]Stage
	events   chan models.PipelineEvent
	eventBus *events.EventBus
}

func NewPipeline(
	config models.PipelineConfig,
	registry *llm.ProviderRegistry,
	store *storage.MongoStore,
	logger *slog.Logger,
) *Pipeline {
	if logger == nil {
		logger = slog.Default()
	}

	p := &Pipeline{
		config:   config,
		registry: registry,
		storage:  store,
		logger:   logger,
		stages:   make(map[int]Stage),
		events:   make(chan models.PipelineEvent, 100),
	}

	return p
}

func (p *Pipeline) SetEventBus(bus *events.EventBus) {
	p.eventBus = bus
}

func (p *Pipeline) UpdateConfig(cfg models.PipelineConfig) {
	p.config = cfg
	p.logger.Info("pipeline config updated via hot-reload",
		"synchronous_consolidation", cfg.SynchronousConsolidation,
		"debug", cfg.Debug,
	)
}

func (p *Pipeline) RegisterStage(num int, stage Stage) {
	p.stages[num] = stage
}

func (p *Pipeline) Execute(ctx context.Context, input models.PipelineInput) (models.PipelineOutput, error) {
	pipelineRunID := uuid.New().String()
	startTime := time.Now().UTC()

	// Create a logger with pipeline run ID for correlation
	runLogger := p.logger.With("pipeline_run_id", pipelineRunID)

	runLogger.Info("pipeline started",
		"conversation_id", input.ConversationID,
		"turn_id", input.TurnID,
	)

	output := models.PipelineOutput{
		ConversationID: input.ConversationID,
		TurnID:         input.TurnID,
	}

	var stage0Output *models.PerceptionOutput
	var stage1Output *models.RecallOutput
	var stage2Output *models.InterpretationOutput
	var stage3Output *models.ReflectionOutput

	stageInput := StageInput{
		PipelineInput:  input,
		ConversationID: input.ConversationID,
		TurnID:         input.TurnID,
		PipelineRunID:  pipelineRunID,
	}

	stageResults := make([]models.StageResult, 0, 6)

	if stage, ok := p.stages[0]; ok && p.config.Stage0.Enabled {
		result := p.executeStage(ctx, runLogger, 0, stage, stageInput)
		stageResult := models.StageResult{
			Stage:      0,
			Name:       stage.Name(),
			Success:    result.Success,
			DurationMs: result.DurationMs,
			Error:      result.Error,
		}
		if result.Success {
			if so, ok := result.Raw.(*models.PerceptionOutput); ok {
				stage0Output = so
				stageInput.Stage0Output = so
				if p.config.Debug {
					stageResult.DebugPrompt = so.DebugPrompt
				}
			}
		}
		stageResults = append(stageResults, stageResult)
	}

	if stage, ok := p.stages[1]; ok && p.config.Stage1.Enabled {
		result := p.executeStage(ctx, runLogger, 1, stage, stageInput)
		stageResults = append(stageResults, models.StageResult{
			Stage:      1,
			Name:       stage.Name(),
			Success:    result.Success,
			DurationMs: result.DurationMs,
			Error:      result.Error,
		})
		if result.Success {
			if so, ok := result.Raw.(*models.RecallOutput); ok {
				stage1Output = so
				stageInput.Stage1Output = so
			}
		}
	}

	if stage, ok := p.stages[2]; ok && p.config.Stage2.Enabled {
		result := p.executeStage(ctx, runLogger, 2, stage, stageInput)
		stageResult := models.StageResult{
			Stage:      2,
			Name:       stage.Name(),
			Success:    result.Success,
			DurationMs: result.DurationMs,
			Error:      result.Error,
		}
		if result.Success {
			if so, ok := result.Raw.(*models.InterpretationOutput); ok {
				stage2Output = so
				stageInput.Stage2Output = so
				if p.config.Debug {
					stageResult.DebugPrompt = so.DebugPrompt
				}
			}
		}
		stageResults = append(stageResults, stageResult)
	}

	if stage, ok := p.stages[3]; ok && p.config.Stage3.Enabled {
		result := p.executeStage(ctx, runLogger, 3, stage, stageInput)
		stageResult := models.StageResult{
			Stage:      3,
			Name:       stage.Name(),
			Success:    result.Success,
			DurationMs: result.DurationMs,
			Error:      result.Error,
		}
		if result.Success {
			if so, ok := result.Raw.(*models.ReflectionOutput); ok {
				stage3Output = so
				stageInput.Stage3Output = so
				if p.config.Debug {
					stageResult.DebugPrompt = so.DebugPrompt
				}
			}
		}
		stageResults = append(stageResults, stageResult)
	}

	if stage, ok := p.stages[4]; ok && p.config.Stage4.Enabled {
		result := p.executeStage(ctx, runLogger, 4, stage, stageInput)
		stageResult := models.StageResult{
			Stage:      4,
			Name:       stage.Name(),
			Success:    result.Success,
			DurationMs: result.DurationMs,
			Error:      result.Error,
		}
		if result.Success {
			if so, ok := result.Raw.(*models.ResponseOutput); ok {
				stageInput.Stage4Output = so
				output.Response = so.Response
				if p.config.Debug {
					stageResult.DebugPrompt = so.DebugPrompt
					output.ResponsePrompt = so.DebugPrompt
				}
			}
		}
		stageResults = append(stageResults, stageResult)
	}

	if stage, ok := p.stages[5]; ok && p.config.Stage5.Enabled {
		result := p.executeStage(ctx, runLogger, 5, stage, stageInput)
		stageResult := models.StageResult{
			Stage:      5,
			Name:       stage.Name(),
			Success:    result.Success,
			DurationMs: result.DurationMs,
			Error:      result.Error,
		}
		if result.Success {
			if so, ok := result.Raw.(*models.ResponseOutput); ok {
				output.Response = so.Response
				if p.config.Debug {
					stageResult.DebugPrompt = so.DebugPrompt
					output.ResponsePrompt = so.DebugPrompt
				}
			}
		}
		stageResults = append(stageResults, stageResult)
	}

	if output.Response == "" {
		output.Response = "I apologize, but I was unable to generate a response at this time."
	}

	if p.config.SynchronousConsolidation {
		if stage, ok := p.stages[6]; ok && p.config.Stage6.Enabled {
			stageInput.Stage0Output = stage0Output
			stageInput.Stage1Output = stage1Output
			stageInput.Stage2Output = stage2Output
			stageInput.Stage3Output = stage3Output
			result := p.executeStage(ctx, runLogger, 6, stage, stageInput)
			stageResults = append(stageResults, models.StageResult{
				Stage:      6,
				Name:       stage.Name(),
				Success:    result.Success,
				DurationMs: result.DurationMs,
				Error:      result.Error,
			})
			if result.Success {
				if consolidation, ok := result.Raw.(*models.ConsolidationOutput); ok {
					output.Consolidation = consolidation
					p.emitConsolidationEvents(consolidation)
				}
			}
		}
	} else if stage, ok := p.stages[6]; ok && p.config.Stage6.Enabled {
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			result := p.executeStage(bgCtx, runLogger, 6, stage, stageInput)
			runLogger.Info("async consolidation completed",
				"pipeline_run_id", pipelineRunID,
				"success", result.Success,
				"duration_ms", result.DurationMs,
			)
			if result.Success {
				if consolidation, ok := result.Raw.(*models.ConsolidationOutput); ok {
					p.emitConsolidationEvents(consolidation)
				}
			}
		}()
	}

	totalDuration := time.Since(startTime)
	output.StageOutputs = stageResults
	output.DurationMs = totalDuration.Milliseconds()
	output.CompletedAt = time.Now().UTC()

	allSuccess := true
	for _, sr := range stageResults {
		if !sr.Success {
			allSuccess = false
			break
		}
	}

	pipelineRun := models.PipelineRun{
		BaseRecord:      models.NewBaseRecord(pipelineRunID),
		ConversationID:  input.ConversationID,
		TurnID:          input.TurnID,
		StageResults:    stageResults,
		TotalDurationMs: totalDuration.Milliseconds(),
		Success:         allSuccess,
	}

	if err := p.storage.PipelineRun().RecordRun(ctx, pipelineRun); err != nil {
		p.logger.Error("failed to record pipeline run", "error", err)
	}

	p.logger.Info("pipeline completed",
		"pipeline_run_id", pipelineRunID,
		"duration_ms", totalDuration.Milliseconds(),
		"success", allSuccess,
	)

	return output, nil
}

func (p *Pipeline) emitConsolidationEvents(consolidation *models.ConsolidationOutput) {
	if p.eventBus == nil {
		return
	}
	for _, action := range consolidation.Actions {
		if action.EntityType == "memory" {
			p.eventBus.PublishMemoryEvent(events.EventMemoryCreated, consolidation.ConversationID, consolidation.TurnID, action.EntityID, models.AnyMap{
				"action":  action.Type,
				"reason":  action.Reason,
				"success": action.Success,
			})
		} else if action.EntityType == "belief" {
			switch action.Type {
			case "CREATE":
				details := models.AnyMap{
					"belief_id": action.EntityID,
					"action":    action.Type,
					"reason":    action.Reason,
					"status":    "created",
				}
				mergeActionDetails(details, action.Details)
				p.eventBus.PublishBeliefEventWithData(events.EventBeliefCreated, consolidation.ConversationID, consolidation.TurnID, action.EntityID, details)
			case "UPDATE":
				details := models.AnyMap{
					"belief_id": action.EntityID,
					"action":    action.Type,
					"reason":    action.Reason,
					"success":   action.Success,
				}
				mergeActionDetails(details, action.Details)
				p.eventBus.PublishBeliefEventWithData(events.EventBeliefUpdated, consolidation.ConversationID, consolidation.TurnID, action.EntityID, details)
			case "REJECT":
				details := models.AnyMap{
					"belief_id": action.EntityID,
					"reason":    action.Reason,
				}
				mergeActionDetails(details, action.Details)
				p.eventBus.PublishBeliefEventWithData(events.EventBeliefRejected, consolidation.ConversationID, consolidation.TurnID, action.EntityID, details)
			case "PROMOTE":
				details := models.AnyMap{
					"belief_id": action.EntityID,
					"reason":    action.Reason,
				}
				mergeActionDetails(details, action.Details)
				p.eventBus.PublishBeliefEventWithData(events.EventBeliefPromoted, consolidation.ConversationID, consolidation.TurnID, action.EntityID, details)
			}
		} else if action.EntityType == "opinion" {
			p.eventBus.PublishBeliefEventWithData(events.EventOpinionCommitted, consolidation.ConversationID, consolidation.TurnID, action.EntityID, models.AnyMap{
				"action": action.Type,
				"reason": action.Reason,
			})
		} else if action.EntityType == "hypothesis" || action.EntityType == "uncertainty" {
			details := models.AnyMap{
				"entity_id":   action.EntityID,
				"entity_type": action.EntityType,
				"action":      action.Type,
				"reason":      action.Reason,
				"success":     action.Success,
			}
			mergeActionDetails(details, action.Details)
			p.eventBus.Publish(events.Event{
				Type:           events.EventHypothesisCreated,
				ConversationID: consolidation.ConversationID,
				TurnID:         consolidation.TurnID,
				Details:        details,
			})
		} else if action.EntityType == "contradiction" {
			p.eventBus.PublishContradictionEvent(consolidation.ConversationID, consolidation.TurnID, action.EntityID, models.AnyMap{
				"action": action.Type,
				"reason": action.Reason,
			})
			p.eventBus.PublishBeliefEventWithData(events.EventDriftDetected, consolidation.ConversationID, consolidation.TurnID, action.EntityID, models.AnyMap{
				"belief_id": action.EntityID,
				"reason":    action.Reason,
				"severity":  "high",
			})
		}
	}
}

func mergeActionDetails(target models.AnyMap, source models.AnyMap) {
	if source == nil {
		return
	}
	for k, v := range source {
		if _, exists := target[k]; !exists {
			target[k] = v
		}
	}
}

func (p *Pipeline) executeStage(ctx context.Context, logger *slog.Logger, num int, stage Stage, input StageInput) StageOutput {
	stageStart := time.Now()
	logger.Debug("stage started", "stage", num, "name", stage.Name())

	// Set model and provider from config for this stage
	cfg, err := p.getStageConfig(num)
	if err == nil {
		input.ModelName = cfg.Model
		input.ProviderName = cfg.Provider
	}

	p.publishEvent(events.EventType("stage.started"), num, input.ConversationID, input.TurnID, 0, "", 0)

	result, err := stage.Execute(ctx, input)
	duration := time.Since(stageStart)

	model := ""
	if cfg, cerr := p.getStageConfig(num); cerr == nil {
		model = cfg.Model
	}

	if err != nil {
		logger.Error("stage failed",
			"stage", num,
			"name", stage.Name(),
			"error", err,
			"duration_ms", duration.Milliseconds(),
		)
		out := NewStageOutput(num, false, duration.Milliseconds(), nil).WithError(err)
		p.publishEvent(events.EventStageFailed, num, input.ConversationID, input.TurnID, duration.Milliseconds(), model, 0)
		return out
	}

	result.DurationMs = duration.Milliseconds()
	logger.Debug("stage completed",
		"stage", num,
		"name", stage.Name(),
		"duration_ms", duration.Milliseconds(),
	)

	tokenCount := 0
	if resp, ok := result.Raw.(interface{ GetTokenCount() int }); ok {
		tokenCount = resp.GetTokenCount()
	}
	p.publishEvent(events.EventStageCompleted, num, input.ConversationID, input.TurnID, duration.Milliseconds(), model, tokenCount)

	p.recordEvent(events.EventType(stage.Name()+".completed"), num, input.ConversationID, input.TurnID, duration.Milliseconds(), model, map[string]any{
		"success": result.Success,
	})

	return result
}

func (p *Pipeline) publishEvent(eventType events.EventType, stage int, convID, turnID string, durationMs int64, model string, tokenCount int) {
	if p.eventBus == nil {
		p.emitEvent(models.PipelineEvent{
			Event:          string(eventType),
			Stage:          stage,
			ConversationID: convID,
			TurnID:         turnID,
			DurationMs:     durationMs,
			Model:          model,
			TokenCount:     tokenCount,
			Timestamp:      time.Now().UTC(),
		})
		return
	}

	p.eventBus.Publish(events.Event{
		Type:           eventType,
		Stage:          stage,
		ConversationID: convID,
		TurnID:         turnID,
		DurationMs:     durationMs,
		Model:          model,
		Details:        map[string]any{"token_count": tokenCount},
	})
}

func (p *Pipeline) recordEvent(eventType events.EventType, stage int, convID, turnID string, durationMs int64, model string, details map[string]any) {
	if p.eventBus == nil {
		return
	}

	p.eventBus.Publish(events.Event{
		Type:           eventType,
		Stage:          stage,
		ConversationID: convID,
		TurnID:         turnID,
		DurationMs:     durationMs,
		Model:          model,
		Details:        details,
	})
}

func (p *Pipeline) Events() <-chan models.PipelineEvent {
	return p.events
}

func (p *Pipeline) emitEvent(event models.PipelineEvent) {
	select {
	case p.events <- event:
	default:
		p.logger.Warn("event channel full, dropping event", "event", event.Event)
	}
}

func (p *Pipeline) getProvider(providerName string) (llm.LLMProvider, error) {
	if providerName == "" || providerName == "none" {
		return nil, nil
	}
	return p.registry.Get(providerName)
}

func (p *Pipeline) getStageConfig(stageNum int) (models.StageConfig, error) {
	switch stageNum {
	case 0:
		return p.config.Stage0, nil
	case 1:
		return p.config.Stage1, nil
	case 2:
		return p.config.Stage2, nil
	case 3:
		return p.config.Stage3, nil
	case 4:
		return p.config.Stage4, nil
	case 5:
		return p.config.Stage5, nil
	case 6:
		return p.config.Stage6, nil
	default:
		return models.StageConfig{}, fmt.Errorf("unknown stage: %d", stageNum)
	}
}
