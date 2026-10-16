package modelmgr

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type CapabilityTier string

const (
	TierFast    CapabilityTier = "fast"
	TierGeneral CapabilityTier = "general"
	TierHigh    CapabilityTier = "high"
)

func ParseCapabilityTier(s string) (CapabilityTier, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "fast", "lightweight", "lite":
		return TierFast, nil
	case "general", "primary", "standard":
		return TierGeneral, nil
	case "high", "high_end", "reasoning", "top":
		return TierHigh, nil
	default:
		return "", fmt.Errorf("unknown capability tier: %s", s)
	}
}

type ModelStatus string

const (
	StatusAbsent      ModelStatus = "absent"
	StatusDownloading ModelStatus = "downloading"
	StatusVerifying   ModelStatus = "verifying"
	StatusImporting   ModelStatus = "importing"
	StatusTesting     ModelStatus = "testing"
	StatusInstalled   ModelStatus = "installed"
	StatusHealthy     ModelStatus = "healthy"
	StatusActive      ModelStatus = "active"
	StatusFailed      ModelStatus = "failed"
	StatusCorrupt     ModelStatus = "corrupt"
	StatusArchived    ModelStatus = "archived"
)

type InstallStage string

const (
	StageDownload   InstallStage = "download"
	StageVerify     InstallStage = "verify"
	StageImport     InstallStage = "import"
	StageHealthTest InstallStage = "health_test"
	StageActivation InstallStage = "activation"
)

type RequestReason string

const (
	ReasonInsufficient         RequestReason = "insufficient"
	ReasonNewRequirement       RequestReason = "new_requirement"
	ReasonBetterModelAvailable RequestReason = "better_model_available"
	ReasonRecovery             RequestReason = "recovery"
	ReasonCorruption           RequestReason = "corruption"
)

type ModelRequest struct {
	Action     string        `json:"action"`
	Capability string        `json:"capability"`
	Tier       string        `json:"tier"`
	Reason     string        `json:"reason"`
	MaxVRAMMB  int           `json:"max_vram_mb,omitempty"`
	MaxRAMMB   int           `json:"max_ram_mb,omitempty"`
	MinParamsB float64       `json:"min_params_b,omitempty"`
	MaxParamsB float64       `json:"max_params_b,omitempty"`
	Priority   string        `json:"priority,omitempty"`
	TTL        time.Duration `json:"ttl,omitempty"`
}

type AcquisitionResult struct {
	ModelID  string         `json:"model_id"`
	Name     string         `json:"name"`
	Tier     CapabilityTier `json:"tier"`
	Status   ModelStatus    `json:"status"`
	Active   bool           `json:"active"`
	Message  string         `json:"message"`
	Duration time.Duration  `json:"duration"`
}

type CapabilityMetadata struct {
	CodingAbility     int     `json:"coding_ability,omitempty"`
	ReasoningAbility  int     `json:"reasoning_ability,omitempty"`
	InferenceSpeed    int     `json:"inference_speed,omitempty"`
	ArchitectureScore float64 `json:"architecture_score,omitempty"`
	Description       string  `json:"description,omitempty"`
}

type FailureRecord struct {
	Timestamp   time.Time    `json:"timestamp"`
	Stage       InstallStage `json:"stage"`
	Reason      string       `json:"reason"`
	ErrorDetail string       `json:"error_detail"`
	StackTrace  string       `json:"stack_trace,omitempty"`
}

type HealthStatus struct {
	Healthy      bool      `json:"healthy"`
	LastTestAt   time.Time `json:"last_test_at,omitempty"`
	LastError    string    `json:"last_error,omitempty"`
	LatencyMs    int64     `json:"latency_ms,omitempty"`
	OutputSample string    `json:"output_sample,omitempty"`
	Confidence   float64   `json:"confidence,omitempty"`
}

type ModelManifest struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	Provider          string             `json:"provider"`
	Source            string             `json:"source"`
	Format            string             `json:"format"`
	Quantization      string             `json:"quantization"`
	ParameterCount    int64              `json:"parameter_count"`
	FileSize          int64              `json:"file_size"`
	Sha256            string             `json:"sha256"`
	ContextLength     int                `json:"context_length"`
	CapabilityTier    CapabilityTier     `json:"capability_tier"`
	Runtime           string             `json:"runtime"`
	RuntimeName       string             `json:"runtime_name"`
	Status            ModelStatus        `json:"status"`
	Health            HealthStatus       `json:"health"`
	Failures          []FailureRecord    `json:"failures,omitempty"`
	Capabilities      CapabilityMetadata `json:"capabilities,omitempty"`
	CreatedAt         time.Time          `json:"created"`
	UpdatedAt         time.Time          `json:"updated"`
	VRAMRequirementMB int                `json:"vram_requirement_mb,omitempty"`
	RAMRequirementMB  int                `json:"ram_requirement_mb,omitempty"`
	FilePath          string             `json:"file_path,omitempty"`
	ModelfileContent  string             `json:"modelfile_content,omitempty"`
	Version           string             `json:"version,omitempty"`
}

func (m *ModelManifest) MarshalJSON() ([]byte, error) {
	type alias struct {
		ID                string             `json:"id"`
		Name              string             `json:"name"`
		Provider          string             `json:"provider"`
		Source            string             `json:"source"`
		Format            string             `json:"format"`
		Quantization      string             `json:"quantization"`
		ParameterCount    int64              `json:"parameter_count"`
		FileSize          int64              `json:"file_size"`
		Sha256            string             `json:"sha256"`
		ContextLength     int                `json:"context_length"`
		CapabilityTier    CapabilityTier     `json:"capability_tier"`
		Runtime           string             `json:"runtime"`
		RuntimeName       string             `json:"runtime_name"`
		Status            ModelStatus        `json:"status"`
		Health            HealthStatus       `json:"health"`
		Failures          []FailureRecord    `json:"failures,omitempty"`
		Capabilities      CapabilityMetadata `json:"capabilities,omitempty"`
		CreatedAt         time.Time          `json:"created"`
		UpdatedAt         time.Time          `json:"updated"`
		VRAMRequirementMB int                `json:"vram_requirement_mb,omitempty"`
		RAMRequirementMB  int                `json:"ram_requirement_mb,omitempty"`
		FilePath          string             `json:"file_path,omitempty"`
		ModelfileContent  string             `json:"modelfile_content,omitempty"`
		Version           string             `json:"version,omitempty"`
	}
	a := alias{
		ID:                m.ID,
		Name:              m.Name,
		Provider:          m.Provider,
		Source:            m.Source,
		Format:            m.Format,
		Quantization:      m.Quantization,
		ParameterCount:    m.ParameterCount,
		FileSize:          m.FileSize,
		Sha256:            m.Sha256,
		ContextLength:     m.ContextLength,
		CapabilityTier:    m.CapabilityTier,
		Runtime:           m.Runtime,
		RuntimeName:       m.RuntimeName,
		Status:            m.Status,
		Health:            m.Health,
		Failures:          m.Failures,
		Capabilities:      m.Capabilities,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
		VRAMRequirementMB: m.VRAMRequirementMB,
		RAMRequirementMB:  m.RAMRequirementMB,
		FilePath:          m.FilePath,
		ModelfileContent:  m.ModelfileContent,
		Version:           m.Version,
	}
	return json.Marshal(a)
}

func (m *ModelManifest) UnmarshalJSON(data []byte) error {
	type alias struct {
		ID                string             `json:"id"`
		Name              string             `json:"name"`
		Provider          string             `json:"provider"`
		Source            string             `json:"source"`
		Format            string             `json:"format"`
		Quantization      string             `json:"quantization"`
		ParameterCount    int64              `json:"parameter_count"`
		FileSize          int64              `json:"file_size"`
		Sha256            string             `json:"sha256"`
		ContextLength     int                `json:"context_length"`
		CapabilityTier    CapabilityTier     `json:"capability_tier"`
		Runtime           string             `json:"runtime"`
		RuntimeName       string             `json:"runtime_name"`
		Status            ModelStatus        `json:"status"`
		Health            HealthStatus       `json:"health"`
		Failures          []FailureRecord    `json:"failures,omitempty"`
		Capabilities      CapabilityMetadata `json:"capabilities,omitempty"`
		CreatedAt         time.Time          `json:"created"`
		UpdatedAt         time.Time          `json:"updated"`
		VRAMRequirementMB int                `json:"vram_requirement_mb,omitempty"`
		RAMRequirementMB  int                `json:"ram_requirement_mb,omitempty"`
		FilePath          string             `json:"file_path,omitempty"`
		ModelfileContent  string             `json:"modelfile_content,omitempty"`
		Version           string             `json:"version,omitempty"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	m.ID = a.ID
	m.Name = a.Name
	m.Provider = a.Provider
	m.Source = a.Source
	m.Format = a.Format
	m.Quantization = a.Quantization
	m.ParameterCount = a.ParameterCount
	m.FileSize = a.FileSize
	m.Sha256 = a.Sha256
	m.ContextLength = a.ContextLength
	m.CapabilityTier = a.CapabilityTier
	m.Runtime = a.Runtime
	m.RuntimeName = a.RuntimeName
	m.Status = a.Status
	m.Health = a.Health
	m.Failures = a.Failures
	m.Capabilities = a.Capabilities
	m.CreatedAt = a.CreatedAt
	m.UpdatedAt = a.UpdatedAt
	m.VRAMRequirementMB = a.VRAMRequirementMB
	m.RAMRequirementMB = a.RAMRequirementMB
	m.FilePath = a.FilePath
	m.ModelfileContent = a.ModelfileContent
	m.Version = a.Version
	return nil
}

func (m *ModelManifest) Touch() {
	m.UpdatedAt = time.Now().UTC()
}

func (m *ModelManifest) HasFailed() bool {
	return m.Status == StatusFailed || m.Status == StatusCorrupt
}

func (m *ModelManifest) IsUsable() bool {
	return m.Status == StatusHealthy || m.Status == StatusActive || m.Status == StatusInstalled
}
