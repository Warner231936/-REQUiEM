package hardware

import (
	"time"
)

type HardwareSpec struct {
	CPU             CPUInfo    `json:"cpu"`
	RAMMB           int        `json:"ram_mb"`
	GPUs            []GPUInfo  `json:"gpus"`
	DiskMB          int        `json:"disk_mb"`
	OS              string     `json:"os"`
	Arch            string     `json:"arch"`
	Ollama          OllamaInfo `json:"ollama"`
	RuntimeBackends []string   `json:"runtime_backends,omitempty"`
	DetectedAt      time.Time  `json:"detected_at"`
}

type CPUInfo struct {
	Model   string `json:"model"`
	Cores   int    `json:"cores"`
	Threads int    `json:"threads"`
}

type GPUInfo struct {
	Model   string `json:"model"`
	VRAMMB  int    `json:"vram_mb"`
	Backend string `json:"backend"`
}

type OllamaInfo struct {
	Available  bool              `json:"available"`
	Version    string            `json:"version,omitempty"`
	Running    bool              `json:"running"`
	ModelsPath string            `json:"models_path,omitempty"`
	Installed  []OllamaModelInfo `json:"installed,omitempty"`
}

type OllamaModelInfo struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified,omitempty"`
	Digest   string    `json:"digest,omitempty"`
}
