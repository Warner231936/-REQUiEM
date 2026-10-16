package pipeline

import (
	"context"
	"time"

	"github.com/requiem/cognitive/pkg/models"
)

type Stage interface {
	Name() string
	Execute(ctx context.Context, input StageInput) (StageOutput, error)
}

type StageInput struct {
	PipelineInput  models.PipelineInput
	Stage0Output   *models.PerceptionOutput
	Stage1Output   *models.RecallOutput
	Stage2Output   *models.InterpretationOutput
	Stage3Output   *models.ReflectionOutput
	Stage4Output   *models.ResponseOutput
	ConversationID string
	TurnID         string
	PipelineRunID  string
	ModelName      string
	ProviderName   string
}

type StageOutput struct {
	StageNum   int
	Success    bool
	DurationMs int64
	Error      string
	Raw        any
}

func NewStageOutput(stageNum int, success bool, durationMs int64, raw any) StageOutput {
	return StageOutput{
		StageNum:   stageNum,
		Success:    success,
		DurationMs: durationMs,
		Raw:        raw,
	}
}

func (so StageOutput) WithError(err error) StageOutput {
	so.Success = false
	if err != nil {
		so.Error = err.Error()
	}
	return so
}

type StageContext struct {
	ConversationID string
	TurnID         string
	PipelineRunID  string
	StartTime      time.Time
}

func NewStageContext(convID, turnID, runID string) StageContext {
	return StageContext{
		ConversationID: convID,
		TurnID:         turnID,
		PipelineRunID:  runID,
		StartTime:      time.Now().UTC(),
	}
}

func (sc StageContext) ElapsedMs() int64 {
	return time.Since(sc.StartTime).Milliseconds()
}
