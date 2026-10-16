package mind

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

type DecayConfig struct {
	Enabled            bool          `json:"enabled"`
	HalfLifeHours      float64       `json:"half_life_hours"`
	MinConfidence      float64       `json:"min_confidence"`
	CheckInterval      time.Duration `json:"check_interval"`
	EpisodicMultiplier float64       `json:"episodic_multiplier"`
	SemanticMultiplier float64       `json:"semantic_multiplier"`
}

func DefaultDecayConfig() DecayConfig {
	return DecayConfig{
		Enabled:            true,
		HalfLifeHours:      168,
		MinConfidence:      0.1,
		CheckInterval:      1 * time.Hour,
		EpisodicMultiplier: 1.0,
		SemanticMultiplier: 0.5,
	}
}

type DecayService struct {
	store  *storage.MongoStore
	config DecayConfig
	logger *slog.Logger
}

func NewDecayService(store *storage.MongoStore, config DecayConfig, logger *slog.Logger) *DecayService {
	if logger == nil {
		logger = slog.Default()
	}
	return &DecayService{
		store:  store,
		config: config,
		logger: logger,
	}
}

func (s *DecayService) DecayConfidence(ctx context.Context) error {
	if !s.config.Enabled {
		return nil
	}

	memories, err := s.store.Memory().Search(ctx, models.MemoryQuery{Limit: 10000})
	if err != nil {
		return fmt.Errorf("get memories for decay: %w", err)
	}

	now := time.Now().UTC()
	decayedCount := 0

	for _, mem := range memories {
		mult := s.config.EpisodicMultiplier
		if mem.Memory.Type == models.MemoryTypeSemantic {
			mult = s.config.SemanticMultiplier
		}

		hoursOld := now.Sub(mem.Memory.CreatedAt).Hours()
		decayFactor := calculateDecayFactor(hoursOld, s.config.HalfLifeHours, mult)

		newConfidence := float64(mem.Memory.Confidence) * decayFactor
		if newConfidence < s.config.MinConfidence {
			newConfidence = s.config.MinConfidence
		}

		if newConfidence != float64(mem.Memory.Confidence) {
			mem.Memory.Confidence = models.Confidence(newConfidence)
			if err := s.store.Memory().Update(ctx, mem.Memory); err != nil {
				s.logger.Warn("failed to update memory confidence", "id", mem.Memory.ID, "error", err)
				continue
			}
			decayedCount++
		}
	}

	if decayedCount > 0 {
		s.logger.Info("memory decay applied", "decayed_count", decayedCount)
	}

	return nil
}

func calculateDecayFactor(hoursOld, halfLifeHours, multiplier float64) float64 {
	if hoursOld <= 0 {
		return 1.0
	}
	halfLife := halfLifeHours / multiplier
	return math.Pow(0.5, hoursOld/halfLife)
}

func pow(base, exp float64) float64 {
	return math.Pow(base, exp)
}

type HypothesisPromotionRule struct {
	MinConfidence     float64 `json:"min_confidence"`
	MinEvidenceCount  int     `json:"min_evidence_count"`
	RequireAuditTrail bool    `json:"require_audit_trail"`
}

func DefaultHypothesisPromotionRule() HypothesisPromotionRule {
	return HypothesisPromotionRule{
		MinConfidence:     0.75,
		MinEvidenceCount:  3,
		RequireAuditTrail: true,
	}
}

type HypothesisService struct {
	store  *storage.MongoStore
	rule   HypothesisPromotionRule
	logger *slog.Logger
}

func NewHypothesisService(store *storage.MongoStore, rule HypothesisPromotionRule, logger *slog.Logger) *HypothesisService {
	if logger == nil {
		logger = slog.Default()
	}
	return &HypothesisService{
		store:  store,
		rule:   rule,
		logger: logger,
	}
}

func (s *HypothesisService) CanPromote(ctx context.Context, hypothesis models.Hypothesis) bool {
	if hypothesis.Status != models.HypothesisStatusSupported {
		return false
	}

	if float64(hypothesis.Confidence) < s.rule.MinConfidence {
		return false
	}

	totalEvidence := len(hypothesis.SupportingEvidence)
	if totalEvidence < s.rule.MinEvidenceCount {
		return false
	}

	return true
}

func (s *HypothesisService) PromoteToBelief(ctx context.Context, hypothesisID string) (string, error) {
	hypotheses, err := s.store.Mind().GetHypotheses(ctx, models.MindQuery{Limit: 1000})
	if err != nil {
		return "", fmt.Errorf("get hypotheses: %w", err)
	}

	var hypothesis *models.Hypothesis
	for i := range hypotheses {
		if hypotheses[i].ID == hypothesisID {
			hypothesis = &hypotheses[i]
			break
		}
	}

	if hypothesis == nil {
		return "", fmt.Errorf("hypothesis not found: %s", hypothesisID)
	}

	if !s.CanPromote(ctx, *hypothesis) {
		return "", fmt.Errorf("hypothesis does not meet promotion criteria")
	}

	belief := models.Belief{
		BaseRecord:  hypothesis.BaseRecord,
		Subject:     hypothesis.Subject,
		Content:     fmt.Sprintf("Previously hypothesized: %s", hypothesis.Hypothesis),
		Confidence:  models.Confidence(float64(hypothesis.Confidence) * 0.9),
		Importance:  models.Importance(0.7),
		Status:      models.BeliefStatusActive,
		Evidence:    hypothesis.SupportingEvidence,
		SourceCount: len(hypothesis.SupportingEvidence),
		SourceType:  models.SourceModelInferred,
	}

	if err := s.store.Mind().CreateBelief(ctx, belief); err != nil {
		return "", fmt.Errorf("create belief: %w", err)
	}

	hypothesis.Status = models.HypothesisStatusConfirmed
	if err := s.store.Mind().UpdateHypothesis(ctx, *hypothesis); err != nil {
		s.logger.Warn("failed to update hypothesis status", "error", err)
	}

	auditRecord := storage.AuditRecord{
		EntityType:     "hypothesis",
		EntityID:       hypothesisID,
		Action:         "PROMOTED_TO_BELIEF",
		Reason:         fmt.Sprintf("Promoted to belief %s", belief.ID),
		SourceEventIDs: []string{hypothesisID},
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	s.store.Audit().RecordHistory(ctx, auditRecord)

	s.logger.Info("hypothesis promoted to belief",
		"hypothesis_id", hypothesisID,
		"belief_id", belief.ID,
	)

	return belief.ID, nil
}

type ValueModificationRule struct {
	MinConfidence           float64 `json:"min_confidence"`
	MinSourceCount          int     `json:"min_source_count"`
	RequireStrongerEvidence bool    `json:"require_stronger_evidence"`
}

func DefaultValueModificationRule() ValueModificationRule {
	return ValueModificationRule{
		MinConfidence:           0.8,
		MinSourceCount:          5,
		RequireStrongerEvidence: true,
	}
}

type ValueService struct {
	store  *storage.MongoStore
	rule   ValueModificationRule
	logger *slog.Logger
}

func NewValueService(store *storage.MongoStore, rule ValueModificationRule, logger *slog.Logger) *ValueService {
	if logger == nil {
		logger = slog.Default()
	}
	return &ValueService{
		store:  store,
		rule:   rule,
		logger: logger,
	}
}

func (s *ValueService) CanModifyValue(ctx context.Context, value models.Value, newConfidence float64) bool {
	if s.rule.RequireStrongerEvidence {
		if newConfidence < s.rule.MinConfidence {
			return false
		}
	}

	return true
}

func (s *ValueService) UpdateValue(ctx context.Context, valueID string, newContent string, newImportance float64, reason string) error {
	values, err := s.store.Mind().GetValues(ctx, models.MindQuery{Limit: 1000})
	if err != nil {
		return fmt.Errorf("get values: %w", err)
	}

	var target *models.Value
	for i := range values {
		if values[i].ID == valueID {
			target = &values[i]
			break
		}
	}

	if target == nil {
		return fmt.Errorf("value not found: %s", valueID)
	}

	if !s.CanModifyValue(ctx, *target, float64(target.Importance)) {
		return fmt.Errorf("value modification does not meet evidence threshold")
	}

	target.Content = newContent
	target.Importance = models.Importance(newImportance)

	if err := s.store.Mind().CreateValue(ctx, *target); err != nil {
		return fmt.Errorf("update value: %w", err)
	}

	auditRecord := storage.AuditRecord{
		EntityType:     "value",
		EntityID:       valueID,
		Action:         "UPDATE",
		Reason:         reason,
		SourceEventIDs: []string{valueID},
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	s.store.Audit().RecordHistory(ctx, auditRecord)

	return nil
}
