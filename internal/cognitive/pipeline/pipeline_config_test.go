package pipeline

import (
	"log/slog"
	"os"
	"testing"

	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/pkg/models"
)

func TestPipelineUpdateConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	registry := llm.NewProviderRegistry(logger)

	originalCfg := models.PipelineConfig{
		SynchronousConsolidation: false,
		Debug:                    false,
	}

	p := NewPipeline(originalCfg, registry, nil, logger)

	if p.config.Debug != false {
		t.Error("expected Debug to be false initially")
	}

	newCfg := models.PipelineConfig{
		SynchronousConsolidation: true,
		Debug:                    true,
	}

	p.UpdateConfig(newCfg)

	if p.config.Debug != true {
		t.Error("expected Debug to be true after UpdateConfig")
	}
	if p.config.SynchronousConsolidation != true {
		t.Error("expected SynchronousConsolidation to be true after UpdateConfig")
	}
}
