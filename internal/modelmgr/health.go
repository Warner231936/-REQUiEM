package modelmgr

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type HealthTester struct {
	client         *OllamaClient
	verifier       *Verifier
	testPrompt     string
	expectedOutput string
	logger         *slog.Logger
}

func NewHealthTester(client *OllamaClient, verifier *Verifier, testPrompt, expectedOutput string, logger *slog.Logger) *HealthTester {
	if logger == nil {
		logger = slog.Default()
	}
	if testPrompt == "" {
		testPrompt = "Briefly acknowledge you are ready."
	}
	return &HealthTester{
		client:         client,
		verifier:       verifier,
		testPrompt:     testPrompt,
		expectedOutput: expectedOutput,
		logger:         logger,
	}
}

func (t *HealthTester) Test(ctx context.Context, manifest *ModelManifest) (HealthStatus, error) {
	var health HealthStatus

	if manifest.FilePath != "" && manifest.Sha256 != "" {
		integrityResult, err := t.verifier.Verify(manifest.FilePath, manifest.Sha256, manifest.FileSize)
		if err != nil {
			health.Healthy = false
			health.LastError = fmt.Sprintf("integrity check failed: %v", err)
			return health, fmt.Errorf("integrity check: %w", err)
		}
		if !integrityResult.Valid || integrityResult.Corrupted {
			health.Healthy = false
			health.LastError = integrityResult.Error
			return health, fmt.Errorf("corrupt model: %s", integrityResult.Error)
		}
		t.logger.Debug("integrity check passed during health test", "model", manifest.ID)
	}

	if manifest.RuntimeName == "" {
		health.Healthy = false
		health.LastError = "model not imported into runtime"
		return health, fmt.Errorf("model %q not imported into runtime", manifest.ID)
	}

	if !t.client.IsRunning(ctx) {
		health.Healthy = false
		health.LastError = "ollama not running"
		return health, fmt.Errorf("ollama not running")
	}

	if err := t.testInference(ctx, manifest); err != nil {
		health.Healthy = false
		health.LastError = err.Error()
		return health, err
	}

	runtimeModels, err := t.client.ListModels(ctx)
	if err != nil {
		health.Healthy = false
		health.LastError = fmt.Sprintf("runtime registration check failed: %v", err)
		return health, fmt.Errorf("runtime registration: %w", err)
	}

	registered := false
	for _, rm := range runtimeModels {
		if strings.HasPrefix(rm.Name, manifest.RuntimeName) {
			registered = true
			break
		}
	}
	if !registered {
		health.Healthy = false
		health.LastError = fmt.Sprintf("model %q not registered in runtime", manifest.RuntimeName)
		return health, fmt.Errorf("runtime registration: %s", manifest.RuntimeName)
	}

	health.Healthy = true
	health.LastTestAt = time.Now().UTC()
	health.Confidence = 1.0

	t.logger.Info("model health check passed",
		"model", manifest.ID,
		"runtime_name", manifest.RuntimeName,
		"latency_ms", health.LatencyMs,
	)

	return health, nil
}

func (t *HealthTester) testInference(ctx context.Context, manifest *ModelManifest) error {
	inferenceCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	startTime := time.Now()
	output, err := t.client.Generate(inferenceCtx, manifest.RuntimeName, t.testPrompt, "")
	elapsed := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("inference test failed: %w", err)
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return fmt.Errorf("inference returned empty response")
	}

	if len(output) > 200 {
		t.logger.Debug("model output sample (truncated)",
			"model", manifest.ID,
			"sample", output[:200],
		)
	} else {
		t.logger.Debug("model output sample",
			"model", manifest.ID,
			"sample", output,
		)
	}

	if t.expectedOutput != "" {
		if !strings.Contains(strings.ToLower(output), strings.ToLower(t.expectedOutput)) {
			return fmt.Errorf("inference output did not contain expected text %q", t.expectedOutput)
		}
	}

	if manifest.Health.LatencyMs > 0 || elapsed > 0 {
		manifest.Health.LatencyMs = elapsed.Milliseconds()
	}
	manifest.Health.OutputSample = output
	manifest.Health.LastTestAt = time.Now().UTC()

	return nil
}

func (t *HealthTester) RunQuick(ctx context.Context, manifest *ModelManifest) (HealthStatus, error) {
	var health HealthStatus

	if manifest.RuntimeName == "" {
		health.Healthy = false
		health.LastError = "model not imported"
		return health, fmt.Errorf("not imported")
	}

	if !t.client.IsRunning(ctx) {
		health.Healthy = false
		health.LastError = "ollama not running"
		return health, fmt.Errorf("ollama not running")
	}

	return t.Test(ctx, manifest)
}
