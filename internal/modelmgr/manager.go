package modelmgr

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/requiem/cognitive/internal/modelmgr/hardware"
)

type RuntimeBackend interface {
	IsAvailable(ctx context.Context) bool
	Import(ctx context.Context, manifest *ModelManifest, candidate *Candidate, modelsDir string) error
	VerifyImport(ctx context.Context, manifest *ModelManifest) error
	Remove(ctx context.Context, manifest *ModelManifest) error
	TestInference(ctx context.Context, modelName string, prompt string) (string, error)
}

type ollamaBackend struct {
	client   *OllamaClient
	importer *Importer
	selector *Selector
	logger   *slog.Logger
}

type ModelManager struct {
	mu           sync.RWMutex
	config       Config
	logger       *slog.Logger
	registry     *Registry
	catalog      *Catalog
	downloader   *Downloader
	verifier     *Verifier
	healthTester *HealthTester
	selector     *Selector
	detector     hardware.Detector
	backends     map[string]RuntimeBackend
	hardwareSpec *hardware.HardwareSpec
}

type ModelManagerConfig struct {
	BaseDir           string
	Config            Config
	Registry          *Registry
	Catalog           *Catalog
	Downloader        *Downloader
	Verifier          *Verifier
	HealthTester      *HealthTester
	Selector          *Selector
	Detector          hardware.Detector
	OllamaBackend     RuntimeBackend
	HardwareSpec      *hardware.HardwareSpec
	UtilizationTarget float64
}

func NewModelManager(cfg ModelManagerConfig, logger *slog.Logger) *ModelManager {
	if logger == nil {
		logger = slog.Default()
	}

	c := cfg

	if c.Registry == nil {
		c.Registry = NewRegistry(cfg.Config.RegistryFile, c.Catalog, logger)
	}
	if c.Catalog == nil {
		c.Catalog = NewCatalog(logger)
	}
	if c.Downloader == nil {
		c.Downloader = NewDownloader(cfg.Config.DownloadRetries, cfg.Config.DownloadTimeout, cfg.Config.ChunkSize, logger)
	}
	if c.Verifier == nil {
		c.Verifier = NewVerifier(logger)
	}
	if c.Selector == nil {
		c.Selector = NewSelector(c.Catalog, logger)
	}
	if c.Detector == nil {
		c.Detector = hardware.NewDetector(logger, &hardware.DetectorConfig{
			OllamaURL: cfg.Config.OllamaURL,
		})
	}

	if c.HealthTester == nil {
		c.HealthTester = NewHealthTester(nil, c.Verifier, cfg.Config.HealthTestPrompt, cfg.Config.HealthTestExpect, logger)
	}

	if c.OllamaBackend == nil {
		client := NewOllamaClient(cfg.Config.OllamaURL, logger)
		importer := NewImporter(client, logger)
		backend := &ollamaBackend{
			client:   client,
			importer: importer,
			selector: c.Selector,
			logger:   logger,
		}
		c.OllamaBackend = backend
		c.HealthTester = NewHealthTester(client, c.Verifier, cfg.Config.HealthTestPrompt, cfg.Config.HealthTestExpect, logger)
	}

	if c.HardwareSpec == nil {
		c.HardwareSpec = &hardware.HardwareSpec{}
	}

	return &ModelManager{
		config:       cfg.Config,
		logger:       logger,
		registry:     c.Registry,
		catalog:      c.Catalog,
		downloader:   c.Downloader,
		verifier:     c.Verifier,
		healthTester: c.HealthTester,
		selector:     c.Selector,
		detector:     c.Detector,
		backends:     map[string]RuntimeBackend{"ollama": c.OllamaBackend},
		hardwareSpec: c.HardwareSpec,
	}
}

func (m *ModelManager) DetectHardware(ctx context.Context) (*hardware.HardwareSpec, error) {
	spec, err := m.detector.Detect(ctx)
	if err != nil {
		return nil, fmt.Errorf("hardware detection: %w", err)
	}
	m.mu.Lock()
	m.hardwareSpec = spec
	m.mu.Unlock()

	m.logger.Info("hardware detection complete",
		"cpu", spec.CPU.Model,
		"ram_mb", spec.RAMMB,
		"gpu_count", len(spec.GPUs),
		"disk_mb", spec.DiskMB,
	)

	return spec, nil
}

func (m *ModelManager) Hardware() *hardware.HardwareSpec {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hardwareSpec
}

func (m *ModelManager) Registry() *Registry {
	return m.registry
}

func (m *ModelManager) Catalog() *Catalog {
	return m.catalog
}

func (m *ModelManager) RequestModel(ctx context.Context, req ModelRequest) (*AcquisitionResult, error) {
	logger := m.logger.With("request_id", req.Capability)

	logger.Info("model request received",
		"action", req.Action,
		"capability", req.Capability,
		"tier", req.Tier,
		"reason", req.Reason,
	)

	tier, err := ParseCapabilityTier(req.Tier)
	if err != nil {
		return nil, fmt.Errorf("invalid tier %q: %w", req.Tier, err)
	}

	if req.Action != "acquire_model" {
		req.Action = "acquire_model"
	}

	existing, err := m.registry.GetByTier(tier)
	if err == nil && existing != nil && existing.IsUsable() {
		logger.Info("existing usable model found for tier",
			"tier", string(tier),
			"model_id", existing.ID,
		)
		return &AcquisitionResult{
			ModelID: existing.ID,
			Name:    existing.Name,
			Tier:    tier,
			Status:  existing.Status,
			Active:  existing.Status == StatusActive,
			Message: "existing model reused",
		}, nil
	}

	if m.hardwareSpec == nil || m.hardwareSpec.RAMMB == 0 {
		if _, err := m.DetectHardware(ctx); err != nil {
			return nil, fmt.Errorf("hardware detection required: %w", err)
		}
	}

	if err := m.checkResources(ctx, tier); err != nil {
		return nil, err
	}

	utilTarget := m.config.UtilizationTarget
	if utilTarget <= 0 {
		utilTarget = 0.70
	}

	candidate, err := m.catalog.BestForTier(tier, m.hardwareSpec, utilTarget)
	if err != nil {
		return nil, fmt.Errorf("select candidate: %w", err)
	}

	logger.Info("selected candidate",
		"candidate_id", candidate.ID,
		"name", candidate.Name,
		"vram_mb", candidate.VRAMReqMB,
		"ram_mb", candidate.RAMReqMB,
		"file_size", candidate.FileSizeBytes,
	)

	result, err := m.AcquireModel(ctx, *candidate)
	if err != nil {
		return nil, fmt.Errorf("acquire model: %w", err)
	}

	if req.Priority == "immediate" || req.Priority == "high" {
		if err := m.ActivateIfHealthy(ctx, tier, result.ModelID); err != nil {
			logger.Warn("activation failed", "error", err)
		}
	}

	return result, nil
}

func (m *ModelManager) AcquireModel(ctx context.Context, candidate Candidate) (*AcquisitionResult, error) {
	logger := m.logger.With("candidate_id", candidate.ID)

	manifest, err := m.buildManifest(candidate)
	if err != nil {
		return nil, err
	}

	manifest.Status = StatusDownloading
	_ = m.registry.Register(manifest)

	logger.Info("acquiring model",
		"name", candidate.Name,
		"tier", string(candidate.Tier),
		"source", candidate.SourceType,
	)

	if candidate.SourceType == SourceOllamaPull {
		return m.acquireOllamaNative(ctx, candidate, manifest)
	}

	return m.acquireGGUF(ctx, candidate, manifest)
}

func (m *ModelManager) acquireOllamaNative(ctx context.Context, candidate Candidate, manifest *ModelManifest) (*AcquisitionResult, error) {
	startTime := time.Now()
	m.reportProgress(ctx, "IMPORTING", manifest.ID, "importing native ollama model")

	backend := m.backends["ollama"]
	if !backend.IsAvailable(ctx) {
		return nil, NewError("acquire", manifest.ID, "runtime_unavailable", ErrRuntimeUnavailable)
	}

	existing, err := m.checkExistingAcquisition(ctx, manifest, candidate)
	if err != nil {
		return nil, err
	}
	if existing != manifest {
		manifest = existing
	}

	if err := backend.Import(ctx, manifest, &candidate, m.config.ModelsDir); err != nil {
		m.registry.RecordFailure(manifest.ID, StageImport, "ollama pull/create failed", err)
		return &AcquisitionResult{
			ModelID: manifest.ID,
			Name:    candidate.Name,
			Tier:    candidate.Tier,
			Status:  StatusFailed,
			Message: fmt.Sprintf("MODEL_IMPORT_FAILED: %v", err),
		}, nil
	}

	health, err := m.healthTester.Test(ctx, manifest)
	if err != nil {
		m.registry.RecordFailure(manifest.ID, StageHealthTest, "health test failed", err)
		return &AcquisitionResult{
			ModelID:  manifest.ID,
			Name:     candidate.Name,
			Tier:     candidate.Tier,
			Status:   StatusFailed,
			Message:  fmt.Sprintf("MODEL_HEALTH_TEST_FAILED: %v", err),
			Duration: time.Since(startTime),
		}, nil
	}

	return m.finalizeAcquisition(ctx, manifest, candidate, health, time.Since(startTime))
}

func (m *ModelManager) acquireGGUF(ctx context.Context, candidate Candidate, manifest *ModelManifest) (*AcquisitionResult, error) {
	startTime := time.Now()
	if candidate.GGUFURL == "" {
		return nil, fmt.Errorf("no download URL specified for candidate %s", candidate.ID)
	}

	m.reportProgress(ctx, "DOWNLOADING", manifest.ID, fmt.Sprintf("downloading %s", candidate.Name))

	tmpPath := filepath.Join(m.config.TempDir, sanitizeFilename(candidate.QuantizedName)+".gguf.tmp")
	finalPath := filepath.Join(m.config.ModelsDir, sanitizeFilename(candidate.QuantizedName)+".gguf")

	manifest.FilePath = finalPath

	dlResult, err := m.downloader.Download(ctx, candidate.GGUFURL, tmpPath)
	if err != nil {
		if dlResult != nil && dlResult.FromCache {
			m.logger.Info("using cached download", "path", dlResult.FilePath)
		} else {
			m.registry.RecordFailure(manifest.ID, StageDownload, "download failed", err)
			return &AcquisitionResult{
				ModelID: manifest.ID,
				Name:    candidate.Name,
				Tier:    candidate.Tier,
				Status:  StatusFailed,
				Message: fmt.Sprintf("MODEL_DOWNLOAD_FAILED: %v", err),
			}, nil
		}
	}

	m.reportProgress(ctx, "VERIFYING", manifest.ID, "verifying integrity")

	manifest.FilePath = dlResult.FilePath
	manifest.FileSize = dlResult.Size
	manifest.Sha256 = dlResult.Sha256

	integrity, err := m.verifier.Verify(dlResult.FilePath, candidate.Sha256, candidate.FileSizeBytes)
	if err != nil {
		m.registry.RecordFailure(manifest.ID, StageVerify, "verification error", err)
		return &AcquisitionResult{
			ModelID: manifest.ID,
			Name:    candidate.Name,
			Tier:    candidate.Tier,
			Status:  StatusFailed,
			Message: fmt.Sprintf("MODEL_VERIFY_FAILED: %v", err),
		}, nil
	}
	if !integrity.Valid || integrity.Corrupted {
		m.registry.RecordFailure(manifest.ID, StageVerify, "corruption detected", fmt.Errorf(integrity.Error))
		return &AcquisitionResult{
			ModelID: manifest.ID,
			Name:    candidate.Name,
			Tier:    candidate.Tier,
			Status:  StatusFailed,
			Message: fmt.Sprintf("MODEL_VERIFY_FAILED: %s", integrity.Error),
		}, nil
	}

	if err := m.finalizeFile(ctx, dlResult.FilePath, finalPath); err != nil {
		return nil, fmt.Errorf("finalize file: %w", err)
	}
	manifest.FilePath = finalPath

	m.reportProgress(ctx, "IMPORTING", manifest.ID, "importing into runtime")

	backend := m.backends["ollama"]
	if !backend.IsAvailable(ctx) {
		return nil, NewError("acquire", manifest.ID, "runtime_unavailable", ErrRuntimeUnavailable)
	}

	if err := backend.Import(ctx, manifest, &candidate, m.config.ModelsDir); err != nil {
		m.registry.RecordFailure(manifest.ID, StageImport, "import failed", err)
		return &AcquisitionResult{
			ModelID: manifest.ID,
			Name:    candidate.Name,
			Tier:    candidate.Tier,
			Status:  StatusFailed,
			Message: fmt.Sprintf("MODEL_IMPORT_FAILED: %v", err),
		}, nil
	}

	m.reportProgress(ctx, "TESTING", manifest.ID, "running health test")

	health, err := m.healthTester.Test(ctx, manifest)
	if err != nil {
		m.registry.RecordFailure(manifest.ID, StageHealthTest, "health test failed", err)
		manifest.Health = health
		manifest.Status = StatusFailed
		_ = m.registry.Register(manifest)
		return &AcquisitionResult{
			ModelID:  manifest.ID,
			Name:     candidate.Name,
			Tier:     candidate.Tier,
			Status:   StatusFailed,
			Message:  fmt.Sprintf("MODEL_HEALTH_TEST_FAILED: %v", err),
			Duration: time.Since(startTime),
		}, nil
	}

	return m.finalizeAcquisition(ctx, manifest, candidate, health, time.Since(startTime))
}

func (m *ModelManager) checkExistingAcquisition(ctx context.Context, manifest *ModelManifest, candidate Candidate) (*ModelManifest, error) {
	existing, err := m.registry.Get(manifest.ID)
	if err == nil && existing != nil {
		if existing.IsUsable() && existing.RuntimeName != "" {
			return existing, nil
		}
	}
	return manifest, nil
}

func (m *ModelManager) finalizeFile(ctx context.Context, srcPath, dstPath string) error {
	if srcPath == dstPath {
		return nil
	}

	if _, err := osStat(dstPath); err == nil {
		return nil
	}

	if err := osRename(srcPath, dstPath); err != nil {
		if err := copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("move model file: %w", err)
		}
		_, _ = osRemove(srcPath)
	}
	return nil
}

func (m *ModelManager) finalizeAcquisition(ctx context.Context, manifest *ModelManifest, candidate Candidate, health HealthStatus, duration time.Duration) (*AcquisitionResult, error) {
	manifest.Health = health
	manifest.Status = StatusHealthy
	manifest.UpdatedAt = time.Now().UTC()

	if err := m.registry.Register(manifest); err != nil {
		return nil, fmt.Errorf("register model: %w", err)
	}

	m.reportProgress(ctx, "HEALTHY", manifest.ID, "model is healthy and registered")

	return &AcquisitionResult{
		ModelID:  manifest.ID,
		Name:     manifest.Name,
		Tier:     manifest.CapabilityTier,
		Status:   manifest.Status,
		Active:   false,
		Message:  "model acquired, verified, imported, and tested successfully",
		Duration: duration,
	}, nil
}

func (m *ModelManager) buildManifest(candidate Candidate) (*ModelManifest, error) {
	now := time.Now().UTC()
	return &ModelManifest{
		ID:                candidate.ID,
		Name:              candidate.Name,
		Provider:          candidate.Provider,
		Source:            string(candidate.SourceType),
		Format:            "GGUF",
		Quantization:      candidate.Quantization,
		ParameterCount:    candidate.ParameterCount,
		FileSize:          candidate.FileSizeBytes,
		Sha256:            candidate.Sha256,
		ContextLength:     candidate.ContextLength,
		CapabilityTier:    candidate.Tier,
		Runtime:           "ollama",
		Status:            StatusAbsent,
		Capabilities:      candidate.Capabilities,
		VRAMRequirementMB: candidate.VRAMReqMB,
		RAMRequirementMB:  candidate.RAMReqMB,
		CreatedAt:         now,
		UpdatedAt:         now,
		Version:           "1.0.0",
	}, nil
}

func (m *ModelManager) checkResources(ctx context.Context, tier CapabilityTier) error {
	if m.hardwareSpec == nil {
		return fmt.Errorf("hardware not detected")
	}

	if m.hardwareSpec.DiskMB < 1000 {
		return NewError("resource_check", string(tier), "insufficient_disk", ErrInsufficientDisk)
	}

	if m.hardwareSpec.Ollama.Available && !m.hardwareSpec.Ollama.Running {
		m.logger.Warn("ollama not running", "attempting_to_continue", true)
	}

	return nil
}

func (m *ModelManager) ActivateIfHealthy(ctx context.Context, tier CapabilityTier, modelID string) error {
	manifest, err := m.registry.Get(modelID)
	if err != nil {
		return err
	}

	if manifest.Status != StatusHealthy && manifest.Status != StatusActive {
		return fmt.Errorf("model %q is not healthy (status: %s)", modelID, manifest.Status)
	}

	if manifest.Health.Healthy != true && manifest.Status != StatusActive {
		return fmt.Errorf("model %q health check is not clean", modelID)
	}

	oldActive, err := m.registry.GetActive(tier)
	errOldActive := err == nil && oldActive != nil

	if manifest.Status == StatusFailed {
		return fmt.Errorf("cannot activate failed model")
	}

	if manifest.Health.Healthy {
		if err := m.registry.Activate(tier, modelID); err != nil {
			if errOldActive {
				m.logger.Error("activation failed, rolling back", "error", err, "rollback_to", oldActive.ID)
				_ = m.registry.Activate(tier, oldActive.ID)
			}
			return err
		}
	}

	return nil
}

func (m *ModelManager) Activate(ctx context.Context, tier CapabilityTier, modelID string) error {
	return m.registry.Activate(tier, modelID)
}

func (m *ModelManager) Deactivate(ctx context.Context, tier CapabilityTier) error {
	active, err := m.registry.GetActive(tier)
	if err != nil {
		return err
	}
	return m.registry.Remove(active.ID)
}

func (m *ModelManager) reportProgress(ctx context.Context, stage string, modelID string, detail string) {
	m.logger.Info("model progress",
		"stage", stage,
		"model_id", modelID,
		"detail", detail,
	)
}

func (m *ModelManager) ListModels() ([]*ModelManifest, error) {
	return m.registry.List()
}

func (m *ModelManager) GetModel(modelID string) (*ModelManifest, error) {
	return m.registry.Get(modelID)
}

func (m *ModelManager) GetActiveModel(tier CapabilityTier) (*ModelManifest, error) {
	return m.registry.GetActive(tier)
}

func (m *ModelManager) GetModelByName(name string) (*ModelManifest, error) {
	return m.registry.GetByName(name)
}

func (m *ModelManager) TestModel(ctx context.Context, modelID string) (HealthStatus, error) {
	manifest, err := m.registry.Get(modelID)
	if err != nil {
		return HealthStatus{}, err
	}

	health, err := m.healthTester.Test(ctx, manifest)
	if err != nil {
		m.registry.RecordFailure(modelID, StageHealthTest, "health test failed", err)
		return health, fmt.Errorf("health test: %w", err)
	}

	_ = m.registry.UpdateHealth(modelID, health)
	return health, nil
}

func (m *ModelManager) RemoveModel(ctx context.Context, modelID string) error {
	manifest, err := m.registry.Get(modelID)
	if err != nil {
		return err
	}

	backend := m.backends["ollama"]
	if backend != nil {
		if err := backend.Remove(ctx, manifest); err != nil {
			m.logger.Warn("failed to remove from runtime", "error", err, "model", modelID)
		}
	}

	if manifest.FilePath != "" {
		if _, err := osRemove(manifest.FilePath); err != nil {
			m.logger.Warn("failed to remove model file", "path", manifest.FilePath, "error", err)
		}
	}

	return m.registry.Remove(modelID)
}

func (m *ModelManager) RepairModel(ctx context.Context, modelID string) error {
	manifest, err := m.registry.Get(modelID)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.mu.Unlock()

	if len(manifest.Failures) == 0 && manifest.Status == StatusHealthy {
		return nil
	}

	manifest.Failures = nil
	manifest.Status = StatusInstalled
	manifest.UpdatedAt = time.Now().UTC()

	if manifest.FilePath != "" && manifest.Sha256 != "" {
		result, err := m.verifier.Verify(manifest.FilePath, manifest.Sha256, manifest.FileSize)
		if err != nil || !result.Valid || result.Corrupted {
			manifest.Status = StatusCorrupt
			_ = m.registry.Register(manifest)
			return fmt.Errorf("model file is corrupt and cannot be repaired in place")
		}
	}

	if err := m.registry.Register(manifest); err != nil {
		return err
	}
	return m.registry.ClearFailures(modelID)
}

func (m *ModelManager) UpdateModel(ctx context.Context, modelID string) error {
	candidate, err := m.catalog.Get(modelID)
	if err != nil {
		return fmt.Errorf("candidate definition not found for update: %w", err)
	}

	_, err = m.AcquireModel(ctx, *candidate)
	return err
}

func (m *ModelManager) SaveRegistry() error {
	return m.registry.Save()
}

func (mb *ollamaBackend) IsAvailable(ctx context.Context) bool {
	return mb.client.IsRunning(ctx)
}

func (mb *ollamaBackend) Import(ctx context.Context, manifest *ModelManifest, candidate *Candidate, modelsDir string) error {
	return mb.importer.Import(ctx, manifest, candidate, modelsDir)
}

func (mb *ollamaBackend) VerifyImport(ctx context.Context, manifest *ModelManifest) error {
	return mb.importer.VerifyImported(ctx, manifest)
}

func (mb *ollamaBackend) Remove(ctx context.Context, manifest *ModelManifest) error {
	return mb.importer.RemoveModel(ctx, manifest)
}

func (mb *ollamaBackend) TestInference(ctx context.Context, modelName string, prompt string) (string, error) {
	return mb.client.Generate(ctx, modelName, prompt, "")
}
