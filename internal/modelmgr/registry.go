package modelmgr

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

type Registry struct {
	mu           sync.RWMutex
	registryFile string
	catalog      *Catalog
	logger       *slog.Logger
	manifests    map[string]*ModelManifest
	activeModel  map[CapabilityTier]string
}

func NewRegistry(registryFile string, catalog *Catalog, logger *slog.Logger) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	r := &Registry{
		registryFile: registryFile,
		catalog:      catalog,
		logger:       logger,
		manifests:    make(map[string]*ModelManifest),
		activeModel:  make(map[CapabilityTier]string),
	}
	r.load()
	return r
}

type registryData struct {
	Models      map[string]ModelManifest `json:"models"`
	ActiveModel map[string]string        `json:"active_model"`
	UpdatedAt   time.Time                `json:"updated_at"`
}

func (r *Registry) load() {
	data, err := os.ReadFile(r.registryFile)
	if err != nil {
		if !os.IsNotExist(err) {
			r.logger.Warn("failed to read registry file", "error", err)
		}
		return
	}

	var rd registryData
	if err := json.Unmarshal(data, &rd); err != nil {
		r.logger.Warn("failed to parse registry file", "error", err)
		return
	}

	for id, m := range rd.Models {
		m := m
		r.manifests[id] = &m
	}
	for tier, modelID := range rd.ActiveModel {
		r.activeModel[CapabilityTier(tier)] = modelID
	}
	r.logger.Debug("registry loaded", "model_count", len(r.manifests), "active_count", len(r.activeModel))
}

func (r *Registry) save() error {
	rd := registryData{
		Models:      make(map[string]ModelManifest),
		ActiveModel: make(map[string]string),
		UpdatedAt:   time.Now().UTC(),
	}

	for id, m := range r.manifests {
		if m != nil {
			rd.Models[id] = *m
		}
	}
	for tier, modelID := range r.activeModel {
		rd.ActiveModel[string(tier)] = modelID
	}

	tmpPath := r.registryFile + ".tmp"
	data, err := json.MarshalIndent(rd, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp registry: %w", err)
	}

	if err := os.Rename(tmpPath, r.registryFile); err != nil {
		return fmt.Errorf("atomic registry rename: %w", err)
	}
	return nil
}

func (r *Registry) Save() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.save()
}

func (r *Registry) Register(m *ModelManifest) error {
	if m == nil {
		return fmt.Errorf("manifest cannot be nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.manifests[m.ID]
	if ok && existing != nil {
		for _, f := range m.Failures {
			existing.Failures = append(existing.Failures, f)
		}
		existing.Status = m.Status
		existing.Health = m.Health
		existing.UpdatedAt = m.UpdatedAt
		existing.FilePath = m.FilePath
		existing.RuntimeName = m.RuntimeName
	} else {
		r.manifests[m.ID] = m
	}

	return r.save()
}

func (r *Registry) Get(modelID string) (*ModelManifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	m, ok := r.manifests[modelID]
	if !ok {
		return nil, NewError("registry", modelID, "not_found", ErrModelNotFound)
	}
	return m, nil
}

func (r *Registry) GetByName(name string) (*ModelManifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, m := range r.manifests {
		if m != nil && m.Name == name {
			return m, nil
		}
	}
	return nil, NewError("registry", name, "name_not_found", ErrModelNotFound)
}

func (r *Registry) GetByTier(tier CapabilityTier) (*ModelManifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var best *ModelManifest
	for _, m := range r.manifests {
		if m == nil {
			continue
		}
		if m.CapabilityTier == tier && m.IsUsable() {
			if best == nil || m.Capabilities.ArchitectureScore > best.Capabilities.ArchitectureScore {
				best = m
			}
		}
	}
	if best == nil {
		return nil, NewError("registry", string(tier), "tier_not_found", ErrModelNotFound)
	}
	return best, nil
}

func (r *Registry) GetActive(tier CapabilityTier) (*ModelManifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	modelID, ok := r.activeModel[tier]
	if !ok {
		return nil, NewError("registry", string(tier), "no_active_model", ErrModelNotFound)
	}

	m, ok := r.manifests[modelID]
	if !ok || m == nil {
		return nil, NewError("registry", modelID, "active_but_missing", ErrModelNotFound)
	}
	return m, nil
}

func (r *Registry) Activate(tier CapabilityTier, modelID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.manifests[modelID]
	if !ok || m == nil {
		return NewError("registry", modelID, "activate", ErrModelNotFound)
	}

	if !m.IsUsable() {
		return NewError("registry", modelID, "activate_unhealthy", ErrHealthCheckFailed)
	}

	oldActive, hadOld := r.activeModel[tier]
	r.manifests[modelID].Status = StatusActive

	if hadOld && oldActive != modelID {
		if old, ok := r.manifests[oldActive]; ok && old != nil && old.Status == StatusActive {
			old.Status = StatusHealthy
		}
	}

	r.activeModel[tier] = modelID

	r.logger.Info("model activated",
		"tier", string(tier),
		"model_id", modelID,
		"previous_active", oldActive,
	)

	return r.save()
}

func (r *Registry) List() ([]*ModelManifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ModelManifest, 0, len(r.manifests))
	for _, m := range r.manifests {
		if m != nil {
			result = append(result, m)
		}
	}
	return result, nil
}

func (r *Registry) ListByTier(tier CapabilityTier) ([]*ModelManifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ModelManifest, 0)
	for _, m := range r.manifests {
		if m != nil && m.CapabilityTier == tier {
			result = append(result, m)
		}
	}
	return result, nil
}

func (r *Registry) Remove(modelID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.manifests[modelID]
	if !ok || m == nil {
		return nil
	}

	if m.Status == StatusActive {
		for tier, id := range r.activeModel {
			if id == modelID {
				delete(r.activeModel, tier)
			}
		}
	}

	delete(r.manifests, modelID)
	return r.save()
}

func (r *Registry) UpdateHealth(modelID string, health HealthStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.manifests[modelID]
	if !ok || m == nil {
		return NewError("registry", modelID, "health_update", ErrModelNotFound)
	}

	m.Health = health
	m.UpdatedAt = time.Now().UTC()

	if !health.Healthy {
		m.Status = StatusFailed
	} else if m.Status == StatusActive {
		m.Status = StatusActive
	} else {
		m.Status = StatusHealthy
	}

	return r.save()
}

func (r *Registry) RecordFailure(modelID string, stage InstallStage, reason string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.manifests[modelID]
	if !ok || m == nil {
		m = &ModelManifest{
			ID:        modelID,
			CreatedAt: time.Now().UTC(),
		}
		r.manifests[modelID] = m
	}

	m.Failures = append(m.Failures, FailureRecord{
		Timestamp:   time.Now().UTC(),
		Stage:       stage,
		Reason:      reason,
		ErrorDetail: err.Error(),
	})
	m.Status = StatusFailed
	m.UpdatedAt = time.Now().UTC()

	r.logger.Error("model failure recorded",
		"model_id", modelID,
		"stage", string(stage),
		"reason", reason,
		"error", err.Error(),
	)

	_ = r.save()
}

func (r *Registry) Repair(modelID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.manifests[modelID]
	if !ok || m == nil {
		return NewError("registry", modelID, "repair", ErrModelNotFound)
	}

	m.Failures = nil
	m.Status = StatusInstalled
	m.UpdatedAt = time.Now().UTC()
	return r.save()
}

func (r *Registry) ClearFailures(modelID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.manifests[modelID]
	if !ok || m == nil {
		return NewError("registry", modelID, "clear_failures", ErrModelNotFound)
	}

	m.Failures = nil
	m.UpdatedAt = time.Now().UTC()

	if m.Health.Healthy {
		m.Status = StatusHealthy
	}

	return r.save()
}
