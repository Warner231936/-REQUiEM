package modelmgr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const ManifestFilename = "manifest.json"

type ManifestStore struct {
	baseDir string
}

func NewManifestStore(baseDir string) *ManifestStore {
	return &ManifestStore{baseDir: baseDir}
}

func (s *ManifestStore) PathFor(modelID string) string {
	safeID := sanitizeModelID(modelID)
	return filepath.Join(s.baseDir, safeID, ManifestFilename)
}

func (s *ManifestStore) Load(modelID string) (*ModelManifest, error) {
	path := s.PathFor(modelID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("manifest not found for model %q", modelID)
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m ModelManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

func (s *ManifestStore) Save(m *ModelManifest) error {
	dir := filepath.Dir(s.PathFor(m.ID))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create manifest dir: %w", err)
	}

	tmpPath := s.PathFor(m.ID) + ".tmp"
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp manifest: %w", err)
	}

	if err := os.Rename(tmpPath, s.PathFor(m.ID)); err != nil {
		return fmt.Errorf("atomic manifest rename: %w", err)
	}
	return nil
}

func (s *ManifestStore) List() ([]*ModelManifest, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read manifest dir: %w", err)
	}

	var manifests []*ModelManifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(s.baseDir, entry.Name(), ManifestFilename)
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var m ModelManifest
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		manifests = append(manifests, &m)
	}
	return manifests, nil
}

func (s *ManifestStore) Delete(modelID string) error {
	path := s.PathFor(modelID)
	return os.Remove(path)
}

func sanitizeModelID(id string) string {
	safe := ""
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			safe += string(r)
		} else {
			safe += "_"
		}
	}
	if safe == "" {
		safe = "model"
	}
	return safe
}
