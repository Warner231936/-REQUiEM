package modelmgr

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/requiem/cognitive/internal/modelmgr/hardware"
)

type Selector struct {
	catalog *Catalog
	logger  *slog.Logger
}

func NewSelector(catalog *Catalog, logger *slog.Logger) *Selector {
	if logger == nil {
		logger = slog.Default()
	}
	return &Selector{
		catalog: catalog,
		logger:  logger,
	}
}

type ScoringResult struct {
	Candidate  Candidate
	Score      float64
	FitsBudget bool
	Reason     string
}

func (s *Selector) Select(tier CapabilityTier, hw *hardware.HardwareSpec, utilTarget float64) (*Candidate, error) {
	if s.catalog == nil {
		return nil, fmt.Errorf("catalog not initialized")
	}
	if hw == nil {
		return nil, fmt.Errorf("hardware spec not available")
	}

	candidates := s.catalog.ByTier(tier)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidates for tier %q", tier)
	}

	results := s.Score(candidates, hw, utilTarget)

	for _, r := range results {
		s.logger.Debug("candidate scored",
			"id", r.Candidate.ID,
			"score", r.Score,
			"fits", r.FitsBudget,
			"reason", r.Reason,
		)
	}

	for _, r := range results {
		if r.FitsBudget {
			return &r.Candidate, nil
		}
	}

	s.logger.Warn("no candidates fit budget, falling back to smallest non-GPU",
		"tier", string(tier),
	)

	for _, r := range results {
		if !r.Candidate.RequiresGPU {
			return &r.Candidate, nil
		}
	}

	return &results[0].Candidate, nil
}

func (s *Selector) Score(candidates []Candidate, hw *hardware.HardwareSpec, utilTarget float64) []ScoringResult {
	budget := computeBudget(hw, utilTarget)

	results := make([]ScoringResult, 0, len(candidates))
	for _, cand := range candidates {
		fits := fitsBudget(&cand, budget)

		score := cand.Capabilities.ArchitectureScore

		if !fits {
			score *= 0.1
		}

		if cand.Capabilities.InferenceSpeed > 7 {
			score += 0.05
		}
		if cand.Capabilities.CodingAbility > 7 {
			score += 0.03
		}

		reason := ""
		if !fits {
			reason = fmt.Sprintf("exceeds budget: vram=%dMB/%dMB ram=%dMB/%dMB disk=%dMB/%dMB",
				cand.VRAMReqMB, budget.VRAMMB,
				cand.RAMReqMB, budget.RAMMB,
				int(cand.FileSizeBytes/(1024*1024)), budget.DiskMB)
		}

		results = append(results, ScoringResult{
			Candidate:  cand,
			Score:      score,
			FitsBudget: fits,
			Reason:     reason,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].FitsBudget != results[j].FitsBudget {
			return results[i].FitsBudget
		}
		return results[i].Score > results[j].Score
	})

	return results
}

func (s *Selector) SelectForCapability(capability string, hw *hardware.HardwareSpec, utilTarget float64) (*Candidate, error) {
	keyword := strings.ToLower(capability)

	var preferredTiers []CapabilityTier
	switch {
	case strings.Contains(keyword, "fast") || strings.Contains(keyword, "light") || strings.Contains(keyword, "routine"):
		preferredTiers = []CapabilityTier{TierFast}
	case strings.Contains(keyword, "code") || strings.Contains(keyword, "develop") || strings.Contains(keyword, "engineer"):
		preferredTiers = []CapabilityTier{TierHigh, TierGeneral, TierFast}
	case strings.Contains(keyword, "reason") || strings.Contains(keyword, "analy") || strings.Contains(keyword, "architect"):
		preferredTiers = []CapabilityTier{TierHigh, TierGeneral, TierFast}
	case strings.Contains(keyword, "high") || strings.Contains(keyword, "top") || strings.Contains(keyword, "power"):
		preferredTiers = []CapabilityTier{TierHigh, TierGeneral, TierFast}
	default:
		preferredTiers = []CapabilityTier{TierGeneral, TierFast, TierHigh}
	}

	for _, tier := range preferredTiers {
		cand, err := s.Select(tier, hw, utilTarget)
		if err == nil && cand != nil {
			return cand, nil
		}
	}

	return s.Select(TierGeneral, hw, utilTarget)
}
