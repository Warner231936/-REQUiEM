package modelmgr

import (
	"context"
	"fmt"
	"time"
)

type BootstrapConfig struct {
	Tiers        []CapabilityTier
	AutoActivate bool
	SkipExisting bool
	Parallel     bool
	RecoveryMode bool
}

func DefaultBootstrapConfig() BootstrapConfig {
	return BootstrapConfig{
		Tiers:        []CapabilityTier{TierFast, TierGeneral, TierHigh},
		AutoActivate: true,
		SkipExisting: true,
		Parallel:     false,
		RecoveryMode: false,
	}
}

type BootstrapResult struct {
	Tier    CapabilityTier
	ModelID string
	Name    string
	Status  ModelStatus
	Active  bool
	Error   string
}

type BootstrapResults struct {
	Results []BootstrapResult
	Summary string
	AllOK   bool
	Started time.Time
	Ended   time.Time
}

func (m *ModelManager) Bootstrap(ctx context.Context, cfg BootstrapConfig) (*BootstrapResults, error) {
	m.logger.Info("starting bootstrap", "tiers", cfg.Tiers, "recovery_mode", cfg.RecoveryMode)

	if len(cfg.Tiers) == 0 {
		cfg.Tiers = DefaultBootstrapConfig().Tiers
	}

	if m.hardwareSpec == nil || m.hardwareSpec.RAMMB == 0 {
		if _, err := m.DetectHardware(ctx); err != nil {
			return nil, fmt.Errorf("hardware detection failed: %w", err)
		}
	}

	results := &BootstrapResults{
		Started: time.Now().UTC(),
	}

	if cfg.Parallel {
		results = m.bootstrapParallel(ctx, cfg, results)
	} else {
		results = m.bootstrapSequential(ctx, cfg, results)
	}

	results.Ended = time.Now().UTC()
	results.AllOK = true
	for _, r := range results.Results {
		if r.Error != "" || r.Status != StatusHealthy {
			results.AllOK = false
		}
	}

	if cfg.AutoActivate {
		for i := range results.Results {
			r := &results.Results[i]
			if r.Error == "" && r.Status == StatusHealthy {
				m.logger.Info("activating model", "tier", r.Tier, "model_id", r.ModelID)
				if err := m.registry.Activate(r.Tier, r.ModelID); err != nil {
					m.logger.Warn("auto-activation failed", "tier", r.Tier, "error", err)
				} else {
					r.Active = true
				}
			}
		}
	}

	results.Summary = fmt.Sprintf("Bootstrap complete: %d models processed, %d healthy",
		len(results.Results), countHealthy(results.Results))

	m.logger.Info(results.Summary)
	return results, nil
}

func (m *ModelManager) bootstrapSequential(ctx context.Context, cfg BootstrapConfig, results *BootstrapResults) *BootstrapResults {
	utilTarget := m.config.UtilizationTarget
	if utilTarget <= 0 {
		utilTarget = 0.70
	}

	for _, tier := range cfg.Tiers {
		if ctx.Err() != nil {
			break
		}

		m.logger.Info("bootstrap tier", "tier", string(tier))

		result := BootstrapResult{Tier: tier}

		if cfg.SkipExisting {
			if existing, err := m.registry.GetActive(tier); err == nil && existing != nil && existing.IsUsable() {
				result.ModelID = existing.ID
				result.Name = existing.Name
				result.Status = existing.Status
				result.Active = true
				results.Results = append(results.Results, result)
				m.logger.Info("tier already has active model", "tier", string(tier), "model_id", existing.ID)
				continue
			}
		}

		candidate, err := m.selector.Select(tier, m.hardwareSpec, utilTarget)
		if err != nil {
			result.Error = fmt.Sprintf("SELECTOR_FAILED: %v", err)
			results.Results = append(results.Results, result)
			continue
		}

		result.ModelID = candidate.ID

		acqResult, err := m.acquireForBootstrap(ctx, *candidate)
		if err != nil {
			result.Error = fmt.Sprintf("ACQUISITION_ERROR: %v", err)
			results.Results = append(results.Results, result)
			continue
		}

		result.Name = candidate.Name
		result.Status = acqResult.Status

		if acqResult.Status == StatusFailed {
			result.Error = acqResult.Message
		}

		results.Results = append(results.Results, result)
	}

	return results
}

func (m *ModelManager) bootstrapParallel(ctx context.Context, cfg BootstrapConfig, results *BootstrapResults) *BootstrapResults {
	utilTarget := m.config.UtilizationTarget
	if utilTarget <= 0 {
		utilTarget = 0.70
	}

	type asyncResult struct {
		tier   CapabilityTier
		result BootstrapResult
	}

	resultCh := make(chan asyncResult, len(cfg.Tiers))

	for _, tier := range cfg.Tiers {
		go func(tier CapabilityTier) {
			result := BootstrapResult{Tier: tier}

			if cfg.SkipExisting {
				if existing, err := m.registry.GetActive(tier); err == nil && existing != nil && existing.IsUsable() {
					result.ModelID = existing.ID
					result.Name = existing.Name
					result.Status = existing.Status
					result.Active = true
					resultCh <- asyncResult{tier, result}
					return
				}
			}

			candidate, err := m.selector.Select(tier, m.hardwareSpec, utilTarget)
			if err != nil {
				result.Error = fmt.Sprintf("SELECTOR_FAILED: %v", err)
				resultCh <- asyncResult{tier, result}
				return
			}

			result.ModelID = candidate.ID
			acqResult, err := m.acquireForBootstrap(ctx, *candidate)
			if err != nil {
				result.Error = fmt.Sprintf("ACQUISITION_ERROR: %v", err)
			}
			result.Name = candidate.Name
			result.Status = acqResult.Status
			if acqResult.Status == StatusFailed {
				result.Error = acqResult.Message
			}
			resultCh <- asyncResult{tier, result}
		}(tier)
	}

	for i := 0; i < len(cfg.Tiers); i++ {
		r := <-resultCh
		results.Results = append(results.Results, r.result)
	}

	return results
}

func (m *ModelManager) acquireForBootstrap(ctx context.Context, candidate Candidate) (*AcquisitionResult, error) {
	manifest, err := m.buildManifest(candidate)
	if err != nil {
		return nil, err
	}

	manifest.Status = StatusDownloading
	_ = m.registry.Register(manifest)

	if candidate.SourceType == SourceOllamaPull {
		return m.acquireOllamaNative(ctx, candidate, manifest)
	}

	return m.acquireGGUF(ctx, candidate, manifest)
}

func countHealthy(results []BootstrapResult) int {
	count := 0
	for _, r := range results {
		if r.Error == "" && r.Status == StatusHealthy {
			count++
		}
	}
	return count
}

type RecoveryReport struct {
	CheckedAt   time.Time
	TotalModels int
	NeedsRepair int
	Entries     []RecoveryEntry
	Errors      []string
}

type RecoveryEntry struct {
	ModelID        string
	Name           string
	Tier           string
	Status         ModelStatus
	Healthy        bool
	FilePresent    bool
	RuntimePresent bool
	Issue          string
	NeedsRepair    bool
}

func (m *ModelManager) ScanForRecovery() *RecoveryReport {
	report := &RecoveryReport{
		CheckedAt: time.Now().UTC(),
	}

	manifests, err := m.registry.List()
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("registry error: %v", err))
		return report
	}

	for _, man := range manifests {
		entry := RecoveryEntry{
			ModelID: man.ID,
			Name:    man.Name,
			Tier:    string(man.CapabilityTier),
			Status:  man.Status,
			Healthy: man.Health.Healthy,
		}

		if man.FilePath != "" && fileExists(man.FilePath) {
			entry.FilePresent = true
		}
		if man.RuntimeName != "" {
			entry.RuntimePresent = m.ollamaHasModel(man.RuntimeName)
		}

		needsRepair := false
		if man.HasFailed() {
			needsRepair = true
			entry.Issue = "has_failures"
		}
		if !entry.FilePresent && man.Status != StatusAbsent && man.Sha256 != "" {
			needsRepair = true
			entry.Issue = "missing_file"
		}
		if entry.FilePresent && man.Sha256 != "" {
			result, err := m.verifier.Verify(man.FilePath, man.Sha256, man.FileSize)
			if err != nil || result == nil || !result.Valid || result.Corrupted {
				needsRepair = true
				entry.Issue = "corrupt_file"
			}
		}
		if !entry.RuntimePresent && man.Status == StatusHealthy {
			needsRepair = true
			entry.Issue = "missing_runtime"
		}

		entry.NeedsRepair = needsRepair
		report.Entries = append(report.Entries, entry)
	}

	report.TotalModels = len(manifests)
	for _, e := range report.Entries {
		if e.NeedsRepair {
			report.NeedsRepair++
		}
	}

	return report
}

func (m *ModelManager) RunRecovery(ctx context.Context) (*RecoveryReport, error) {
	m.logger.Info("starting recovery scan")

	report := m.ScanForRecovery()

	m.logger.Info("recovery scan complete",
		"total_models", report.TotalModels,
		"needs_repair", report.NeedsRepair,
	)

	for _, entry := range report.Entries {
		if !entry.NeedsRepair {
			continue
		}

		m.logger.Info("repairing model", "model_id", entry.ModelID, "issue", entry.Issue)

		manifest, err := m.registry.Get(entry.ModelID)
		if err != nil {
			m.logger.Warn("cannot find manifest for repair", "model_id", entry.ModelID, "error", err)
			continue
		}

		switch entry.Issue {
		case "has_failures", "corrupt_file", "missing_file", "missing_runtime":
			candidate, err := m.catalog.Get(manifest.ID)
			if err != nil {
				m.logger.Warn("candidate not found for re-acquisition", "model_id", entry.ModelID)
				continue
			}

			manifest.Failures = nil
			manifest.Status = StatusAbsent
			manifest.Health = HealthStatus{}
			manifest.UpdatedAt = time.Now().UTC()
			_ = m.registry.Register(manifest)

			acqResult, err := m.AcquireModel(ctx, *candidate)
			if err != nil {
				m.logger.Error("re-acquisition failed", "model_id", entry.ModelID, "error", err)
				report.Errors = append(report.Errors, fmt.Sprintf("re-acquire %s: %v", entry.ModelID, err))
				continue
			}
			if acqResult.Status == StatusFailed {
				report.Errors = append(report.Errors, fmt.Sprintf("re-acquire failed for %s: %s", entry.ModelID, acqResult.Message))
			}
		}
	}

	return report, nil
}

func (m *ModelManager) ollamaHasModel(name string) bool {
	backend, ok := m.backends["ollama"]
	if !ok {
		return false
	}
	ob, ok := backend.(*ollamaBackend)
	if !ok || ob == nil || ob.client == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return ob.client.HasModel(ctx, name)
}
