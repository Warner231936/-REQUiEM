package modelmgr

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Importer struct {
	client *OllamaClient
	logger *slog.Logger
}

func NewImporter(client *OllamaClient, logger *slog.Logger) *Importer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Importer{
		client: client,
		logger: logger,
	}
}

func (imp *Importer) Import(ctx context.Context, manifest *ModelManifest, candidate *Candidate, modelsDir string) error {
	modelfileContent := candidate.GGUFModelfile
	if modelfileContent == "" {
		modelfileContent = imp.generateModelfile(manifest, candidate)
	}

	manifest.ModelfileContent = modelfileContent

	if candidate.SourceType == SourceOllamaPull {
		return imp.importOllamaNative(ctx, manifest, candidate)
	}

	return imp.importGGUF(ctx, manifest, candidate, modelsDir, modelfileContent)
}

func (imp *Importer) importOllamaNative(ctx context.Context, manifest *ModelManifest, candidate *Candidate) error {
	if !imp.client.IsRunning(ctx) {
		return NewError("import", manifest.ID, "ollama_not_running", ErrOllamaNotRunning)
	}

	modelName := candidate.OllamaModel

	imp.logger.Info("importing ollama-native model", "model", modelName)

	if imp.client.HasModel(ctx, modelName) {
		imp.logger.Debug("ollama model already exists", "model", modelName)
		manifest.RuntimeName = modelName
		return nil
	}

	pullCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	if err := imp.client.PullModel(pullCtx, modelName); err != nil {
		return NewError("import", manifest.ID, "pull_failed", err)
	}

	manifest.RuntimeName = modelName
	return nil
}

func (imp *Importer) importGGUF(ctx context.Context, manifest *ModelManifest, candidate *Candidate, modelsDir string, modelfileContent string) error {
	if !imp.client.IsRunning(ctx) {
		return NewError("import", manifest.ID, "ollama_not_running", ErrOllamaNotRunning)
	}

	ggufPath := manifest.FilePath
	if ggufPath == "" {
		ggufPath = filepath.Join(modelsDir, sanitizeFilename(candidate.QuantizedName)+".gguf")
	}

	if _, err := os.Stat(ggufPath); err != nil {
		return NewError("import", manifest.ID, "gguf_not_found", fmt.Errorf("gguf file not found at %s", ggufPath))
	}

	modelName := candidate.QuantizedName
	if candidate.ID != "" {
		modelName = "requiem-" + sanitizeModelID(candidate.ID)
	}

	imp.logger.Info("importing GGUF model into ollama", "model", modelName, "gguf", ggufPath)

	if imp.client.HasModel(ctx, modelName) {
		imp.logger.Debug("ollama model already exists", "model", modelName)
		manifest.RuntimeName = modelName
		return nil
	}

	// Generate clean modelfile without duplicate FROM
	// Use only the filename for FROM, not the full path
	ggufFilename := filepath.Base(ggufPath)
	cleanModelfile := strings.TrimSpace(modelfileContent)
	lines := strings.Split(cleanModelfile, "\n")
	var filtered []string
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "FROM") {
			filtered = append(filtered, line)
		}
	}
	modelfile := fmt.Sprintf("FROM %s\n%s", ggufFilename, strings.Join(filtered, "\n"))

	createCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if err := imp.client.CreateModel(createCtx, modelName, modelfile); err != nil {
		return NewError("import", manifest.ID, "modelfile_creation_failed", err)
	}

	manifest.RuntimeName = modelName
	return nil
}

func (imp *Importer) generateModelfile(manifest *ModelManifest, candidate *Candidate) string {
	var sb strings.Builder

	if candidate.SourceType == SourceFileGGUF {
		quantName := sanitizeFilename(candidate.QuantizedName)
		sb.WriteString(fmt.Sprintf("FROM %s.gguf\n", quantName))
	} else {
		sb.WriteString(fmt.Sprintf("FROM %s\n", candidate.OllamaModel))
	}

	sb.WriteString("\n# Requiem model parameters\n")
	sb.WriteString(fmt.Sprintf("PARAMETER num_ctx %d\n", candidate.ContextLength))

	switch manifest.CapabilityTier {
	case TierFast:
		sb.WriteString("PARAMETER temperature 0.1\n")
		sb.WriteString("PARAMETER num_predict 512\n")
	case TierGeneral:
		sb.WriteString("PARAMETER temperature 0.3\n")
		sb.WriteString("PARAMETER num_predict 2048\n")
	case TierHigh:
		sb.WriteString("PARAMETER temperature 0.2\n")
		sb.WriteString("PARAMETER num_predict 4096\n")
	default:
		sb.WriteString("PARAMETER temperature 0.3\n")
		sb.WriteString("PARAMETER num_predict 1024\n")
	}

	return sb.String()
}

func (imp *Importer) VerifyImported(ctx context.Context, manifest *ModelManifest) error {
	if manifest.RuntimeName == "" {
		return fmt.Errorf("runtime name not set")
	}

	if !imp.client.IsRunning(ctx) {
		return NewError("verify_import", manifest.ID, "ollama_not_running", ErrOllamaNotRunning)
	}

	models, err := imp.client.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("list models: %w", err)
	}

	found := false
	for _, m := range models {
		if strings.HasPrefix(m.Name, manifest.RuntimeName) {
			found = true
			break
		}
	}

	if !found {
		return NewError("verify_import", manifest.ID, "model_not_registered",
			fmt.Errorf("model %q not found in ollama", manifest.RuntimeName))
	}

	return nil
}

func (imp *Importer) RemoveModel(ctx context.Context, manifest *ModelManifest) error {
	if manifest.RuntimeName == "" {
		return nil
	}

	if !imp.client.IsRunning(ctx) {
		return fmt.Errorf("ollama not running")
	}

	if err := imp.client.DeleteModel(ctx, manifest.RuntimeName); err != nil {
		return fmt.Errorf("delete model %q: %w", manifest.RuntimeName, err)
	}

	manifest.RuntimeName = ""
	return nil
}
