package modelmgr

import (
	"fmt"
	"log/slog"

	"github.com/requiem/cognitive/internal/modelmgr/hardware"
)

type SourceType string

const (
	SourceOllamaPull SourceType = "ollama_pull"
	SourceFileGGUF   SourceType = "file_gguf"
)

type Candidate struct {
	ID             string
	Name           string
	SourceType     SourceType
	OllamaModel    string
	GGUFURL        string
	GGUFModelfile  string
	QuantizedName  string
	ParameterCount int64
	FileSizeBytes  int64
	Sha256         string
	Quantization   string
	ContextLength  int
	VRAMReqMB      int
	RAMReqMB       int
	Tier           CapabilityTier
	Provider       string
	Capabilities   CapabilityMetadata
	RequiresGPU    bool
	Notes          string
}

type Catalog struct {
	candidates []Candidate
	logger     *slog.Logger
}

func NewCatalog(logger *slog.Logger) *Catalog {
	if logger == nil {
		logger = slog.Default()
	}
	c := &Catalog{
		candidates: buildDefaultCandidates(),
		logger:     logger,
	}
	c.logger.Debug("catalog initialized", "candidate_count", len(c.candidates))
	return c
}

func (c *Catalog) All() []Candidate {
	return c.candidates
}

func (c *Catalog) ByTier(tier CapabilityTier) []Candidate {
	result := make([]Candidate, 0)
	for _, cand := range c.candidates {
		if cand.Tier == tier {
			result = append(result, cand)
		}
	}
	return result
}

func (c *Catalog) Get(id string) (*Candidate, error) {
	for _, cand := range c.candidates {
		if cand.ID == id {
			return &cand, nil
		}
	}
	return nil, fmt.Errorf("candidate %q not found in catalog", id)
}

func (c *Catalog) Add(candidate Candidate) {
	c.candidates = append(c.candidates, candidate)
}

func (c *Catalog) BestForTier(tier CapabilityTier, hw *hardware.HardwareSpec, utilTarget float64) (*Candidate, error) {
	candidates := c.ByTier(tier)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidates for tier %q", tier)
	}

	budget := computeBudget(hw, utilTarget)

	var best *Candidate
	for i := range candidates {
		cand := &candidates[i]

		if !fitsBudget(cand, budget) {
			continue
		}

		if best == nil {
			best = cand
			continue
		}

		if cand.Capabilities.ArchitectureScore > best.Capabilities.ArchitectureScore {
			best = cand
		}
	}

	if best == nil {
		c.logger.Warn("no candidates fit budget, falling back to smallest non-GPU",
			"tier", string(tier),
			"vram_budget", budget.VRAMMB,
			"ram_budget", budget.RAMMB,
		)
		// Fallback: pick smallest non-GPU candidate
		for i := range candidates {
			if !candidates[i].RequiresGPU {
				best = &candidates[i]
				break
			}
		}
		// If all require GPU, pick the one with lowest VRAM requirement
		if best == nil {
			best = &candidates[0]
			for i := range candidates {
				if candidates[i].VRAMReqMB < best.VRAMReqMB {
					best = &candidates[i]
				}
			}
		}
	}

	return best, nil
}

type resourceBudget struct {
	VRAMMB int
	RAMMB  int
	DiskMB int
}

func computeBudget(hw *hardware.HardwareSpec, utilTarget float64) resourceBudget {
	budget := resourceBudget{
		DiskMB: hw.DiskMB,
	}

	var totalVRAM int
	for _, gpu := range hw.GPUs {
		totalVRAM += gpu.VRAMMB
	}

	if totalVRAM > 0 {
		budget.VRAMMB = int(float64(totalVRAM) * utilTarget)
	}

	if hw.RAMMB > 0 {
		budget.RAMMB = int(float64(hw.RAMMB) * utilTarget)
	}

	if budget.RAMMB == 0 && hw.RAMMB > 0 {
		budget.RAMMB = int(float64(hw.RAMMB) * utilTarget)
	}

	overheadMB := 4000
	if budget.DiskMB > overheadMB {
		budget.DiskMB -= overheadMB
	}

	return budget
}

func fitsBudget(cand *Candidate, budget resourceBudget) bool {
	if budget.VRAMMB > 0 && cand.VRAMReqMB > 0 && cand.VRAMReqMB > budget.VRAMMB {
		return false
	}
	if budget.RAMMB > 0 && cand.RAMReqMB > 0 && cand.RAMReqMB > budget.RAMMB {
		return false
	}
	if budget.DiskMB > 0 && cand.FileSizeBytes > 0 {
		fileSizeMB := int(cand.FileSizeBytes / (1024 * 1024))
		if fileSizeMB > budget.DiskMB {
			return false
		}
	}
	return true
}
