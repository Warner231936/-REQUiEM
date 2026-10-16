package mind

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

type Snapshot struct {
	BaseRecord           models.BaseRecord `json:"base_record" bson:"base_record"`
	BeliefCount          int               `json:"belief_count" bson:"belief_count"`
	OpinionCount         int               `json:"opinion_count" bson:"opinion_count"`
	PreferenceCount      int               `json:"preference_count" bson:"preference_count"`
	GoalCount            int               `json:"goal_count" bson:"goal_count"`
	HypothesisCount      int               `json:"hypothesis_count" bson:"hypothesis_count"`
	UncertaintyCount     int               `json:"uncertainty_count" bson:"uncertainty_count"`
	SelfObservationCount int               `json:"self_observation_count" bson:"self_observation_count"`
	ContradictionCount   int               `json:"contradiction_count" bson:"contradiction_count"`
	MemoryCount          int               `json:"memory_count" bson:"memory_count"`
	AvgConfidence        float64           `json:"avg_confidence" bson:"avg_confidence"`
}

func (s *Snapshot) GetID() string {
	return s.BaseRecord.ID
}

type SnapshotService struct {
	store  *storage.MongoStore
	logger *slog.Logger
}

func NewSnapshotService(store *storage.MongoStore, logger *slog.Logger) *SnapshotService {
	if logger == nil {
		logger = slog.Default()
	}
	return &SnapshotService{
		store:  store,
		logger: logger,
	}
}

func (s *SnapshotService) CreateSnapshot(ctx context.Context) (*Snapshot, error) {
	snapshot := &Snapshot{
		BaseRecord: models.NewBaseRecord(uuid.New().String()),
	}

	beliefs, err := s.store.Mind().GetBeliefs(ctx, models.MindQuery{Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("get beliefs for snapshot: %w", err)
	}
	snapshot.BeliefCount = len(beliefs)

	var totalConfidence float64
	for _, b := range beliefs {
		totalConfidence += float64(b.Confidence)
	}
	if len(beliefs) > 0 {
		snapshot.AvgConfidence = totalConfidence / float64(len(beliefs))
	}

	opinions, err := s.store.Mind().GetOpinions(ctx, models.MindQuery{Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("get opinions for snapshot: %w", err)
	}
	snapshot.OpinionCount = len(opinions)

	preferences, err := s.store.Mind().GetPreferences(ctx, models.MindQuery{Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("get preferences for snapshot: %w", err)
	}
	snapshot.PreferenceCount = len(preferences)

	goals, err := s.store.Mind().GetGoals(ctx, models.MindQuery{Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("get goals for snapshot: %w", err)
	}
	snapshot.GoalCount = len(goals)

	hypotheses, err := s.store.Mind().GetHypotheses(ctx, models.MindQuery{Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("get hypotheses for snapshot: %w", err)
	}
	snapshot.HypothesisCount = len(hypotheses)

	uncertainties, err := s.store.Mind().GetUncertainties(ctx, models.MindQuery{Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("get uncertainties for snapshot: %w", err)
	}
	snapshot.UncertaintyCount = len(uncertainties)

	selfObs, err := s.store.Mind().GetSelfObservations(ctx, models.MindQuery{Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("get self observations for snapshot: %w", err)
	}
	snapshot.SelfObservationCount = len(selfObs)

	contradictions, err := s.store.Mind().GetContradictions(ctx, models.MindQuery{Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("get contradictions for snapshot: %w", err)
	}
	snapshot.ContradictionCount = len(contradictions)

	memories, err := s.store.Memory().Search(ctx, models.MemoryQuery{Limit: 10000})
	if err != nil {
		return nil, fmt.Errorf("get memories for snapshot: %w", err)
	}
	snapshot.MemoryCount = len(memories)

	s.logger.Info("mind snapshot created",
		"id", snapshot.GetID(),
		"beliefs", snapshot.BeliefCount,
		"opinions", snapshot.OpinionCount,
		"avg_confidence", snapshot.AvgConfidence,
	)

	return snapshot, nil
}

type SnapshotComparison struct {
	SnapshotA       *Snapshot `json:"snapshot_a"`
	SnapshotB       *Snapshot `json:"snapshot_b"`
	BeliefDelta     int       `json:"belief_delta"`
	OpinionDelta    int       `json:"opinion_delta"`
	ConfidenceDelta float64   `json:"confidence_delta"`
	NewBeliefs      []string  `json:"new_beliefs"`
	RemovedBeliefs  []string  `json:"removed_beliefs"`
}

func CompareSnapshots(a, b *Snapshot) *SnapshotComparison {
	comparison := &SnapshotComparison{
		SnapshotA:       a,
		SnapshotB:       b,
		BeliefDelta:     b.BeliefCount - a.BeliefCount,
		OpinionDelta:    b.OpinionCount - a.OpinionCount,
		ConfidenceDelta: b.AvgConfidence - a.AvgConfidence,
	}

	return comparison
}

type DriftDetector struct {
	store                   *storage.MongoStore
	logger                  *slog.Logger
	confidenceJumpThreshold float64
	beliefChangeThreshold   int
	memoryFloodThreshold    int
}

func NewDriftDetector(store *storage.MongoStore, logger *slog.Logger) *DriftDetector {
	if logger == nil {
		logger = slog.Default()
	}
	return &DriftDetector{
		store:                   store,
		logger:                  logger,
		confidenceJumpThreshold: 0.3,
		beliefChangeThreshold:   5,
		memoryFloodThreshold:    100,
	}
}

type DriftReport struct {
	Detected  bool         `json:"detected"`
	Timestamp time.Time    `json:"timestamp"`
	Issues    []DriftIssue `json:"issues"`
	Severity  string       `json:"severity"`
}

type DriftIssue struct {
	Type        string        `json:"type"`
	Description string        `json:"description"`
	Severity    string        `json:"severity"`
	EntityID    string        `json:"entity_id,omitempty"`
	Details     models.AnyMap `json:"details,omitempty"`
}

func (d *DriftDetector) DetectDrift(ctx context.Context) (*DriftReport, error) {
	report := &DriftReport{
		Detected:  false,
		Timestamp: time.Now().UTC(),
		Issues:    []DriftIssue{},
	}

	issues, err := d.checkRapidBeliefChanges(ctx)
	if err != nil {
		d.logger.Warn("rapid belief change check failed", "error", err)
	}
	report.Issues = append(report.Issues, issues...)

	issues, err = d.checkConfidenceJumps(ctx)
	if err != nil {
		d.logger.Warn("confidence jump check failed", "error", err)
	}
	report.Issues = append(report.Issues, issues...)

	issues, err = d.checkMemoryFlooding(ctx)
	if err != nil {
		d.logger.Warn("memory flooding check failed", "error", err)
	}
	report.Issues = append(report.Issues, issues...)

	issues, err = d.checkContradictoryPreferences(ctx)
	if err != nil {
		d.logger.Warn("contradictory preferences check failed", "error", err)
	}
	report.Issues = append(report.Issues, issues...)

	issues, err = d.checkDuplicateBeliefs(ctx)
	if err != nil {
		d.logger.Warn("duplicate beliefs check failed", "error", err)
	}
	report.Issues = append(report.Issues, issues...)

	issues, err = d.checkUnstableOpinions(ctx)
	if err != nil {
		d.logger.Warn("unstable opinions check failed", "error", err)
	}
	report.Issues = append(report.Issues, issues...)

	issues, err = d.checkPromptInjectionMemories(ctx)
	if err != nil {
		d.logger.Warn("prompt injection memory check failed", "error", err)
	}
	report.Issues = append(report.Issues, issues...)

	if len(report.Issues) > 0 {
		report.Detected = true
		report.Severity = d.calculateSeverity(report.Issues)
	}

	return report, nil
}

func (d *DriftDetector) checkRapidBeliefChanges(ctx context.Context) ([]DriftIssue, error) {
	var issues []DriftIssue

	history, err := d.store.Audit().GetHistory(ctx, "belief", "")
	if err != nil {
		return nil, err
	}

	changeCount := make(map[string]int)
	cutoff := time.Now().UTC().Add(-1 * time.Hour)

	for _, record := range history {
		ts, err := time.Parse(time.RFC3339, record.Timestamp)
		if err != nil {
			continue
		}
		if ts.After(cutoff) {
			changeCount[record.EntityID]++
		}
	}

	for entityID, count := range changeCount {
		if count > d.beliefChangeThreshold {
			issues = append(issues, DriftIssue{
				Type:        "rapid_belief_change",
				Description: fmt.Sprintf("Belief %s changed %d times in the last hour", entityID, count),
				Severity:    "high",
				EntityID:    entityID,
				Details:     models.AnyMap{"change_count": count},
			})
		}
	}

	return issues, nil
}

func (d *DriftDetector) checkConfidenceJumps(ctx context.Context) ([]DriftIssue, error) {
	var issues []DriftIssue

	history, err := d.store.Audit().GetHistory(ctx, "belief", "")
	if err != nil {
		return nil, err
	}

	for _, record := range history {
		prevState, ok := record.PreviousState["confidence"]
		if !ok {
			continue
		}
		newState, ok := record.NewState["confidence"]
		if !ok {
			continue
		}

		prevConf, ok := prevState.(float64)
		if !ok {
			continue
		}
		newConf, ok := newState.(float64)
		if !ok {
			continue
		}

		jump := newConf - prevConf
		if jump < 0 {
			jump = -jump
		}

		if jump > d.confidenceJumpThreshold {
			issues = append(issues, DriftIssue{
				Type:        "confidence_jump",
				Description: fmt.Sprintf("Belief %s confidence jumped by %.2f", record.EntityID, jump),
				Severity:    "medium",
				EntityID:    record.EntityID,
				Details:     models.AnyMap{"jump": jump, "previous": prevConf, "new": newConf},
			})
		}
	}

	return issues, nil
}

func (d *DriftDetector) checkMemoryFlooding(ctx context.Context) ([]DriftIssue, error) {
	var issues []DriftIssue

	recentCount := 0
	cutoff := time.Now().UTC().Add(-1 * time.Hour)

	memories, err := d.store.Memory().Search(ctx, models.MemoryQuery{Limit: 10000})
	if err != nil {
		return nil, err
	}

	for _, mem := range memories {
		if mem.Memory.CreatedAt.After(cutoff) {
			recentCount++
		}
	}

	if recentCount > d.memoryFloodThreshold {
		issues = append(issues, DriftIssue{
			Type:        "memory_flooding",
			Description: fmt.Sprintf("%d memories created in the last hour", recentCount),
			Severity:    "high",
			Details:     models.AnyMap{"count": recentCount, "threshold": d.memoryFloodThreshold},
		})
	}

	return issues, nil
}

func (d *DriftDetector) checkContradictoryPreferences(ctx context.Context) ([]DriftIssue, error) {
	var issues []DriftIssue

	contradictions, err := d.store.Mind().GetContradictions(ctx, models.MindQuery{
		Limit: 100,
	})
	if err != nil {
		return nil, err
	}

	if len(contradictions) > 10 {
		issues = append(issues, DriftIssue{
			Type:        "unresolved_contradictions",
			Description: fmt.Sprintf("%d unresolved contradictions detected", len(contradictions)),
			Severity:    "medium",
			Details:     models.AnyMap{"count": len(contradictions)},
		})
	}

	return issues, nil
}

func (d *DriftDetector) checkDuplicateBeliefs(ctx context.Context) ([]DriftIssue, error) {
	var issues []DriftIssue

	beliefs, err := d.store.Mind().GetBeliefs(ctx, models.MindQuery{Limit: 1000})
	if err != nil {
		return nil, err
	}

	seen := make(map[string]string)
	for _, b := range beliefs {
		if prevID, ok := seen[b.Content]; ok {
			issues = append(issues, DriftIssue{
				Type:        "duplicate_belief",
				Description: fmt.Sprintf("Duplicate belief content found: %s and %s", prevID, b.ID),
				Severity:    "low",
				EntityID:    b.ID,
				Details:     models.AnyMap{"original_id": prevID, "duplicate_id": b.ID},
			})
		}
		seen[b.Content] = b.ID
	}

	return issues, nil
}

func (d *DriftDetector) calculateSeverity(issues []DriftIssue) string {
	highCount := 0
	mediumCount := 0

	for _, issue := range issues {
		switch issue.Severity {
		case "high":
			highCount++
		case "medium":
			mediumCount++
		}
	}

	if highCount > 0 {
		return "high"
	}
	if mediumCount > 2 {
		return "medium"
	}
	return "low"
}

func (d *DriftDetector) checkUnstableOpinions(ctx context.Context) ([]DriftIssue, error) {
	var issues []DriftIssue

	history, err := d.store.Audit().GetHistory(ctx, "opinion", "")
	if err != nil {
		return nil, err
	}

	changeCount := make(map[string]int)
	cutoff := time.Now().UTC().Add(-1 * time.Hour)

	for _, record := range history {
		ts, err := time.Parse(time.RFC3339, record.Timestamp)
		if err != nil {
			continue
		}
		if ts.After(cutoff) {
			changeCount[record.EntityID]++
		}
	}

	for entityID, count := range changeCount {
		if count > 3 {
			issues = append(issues, DriftIssue{
				Type:        "unstable_opinion",
				Description: fmt.Sprintf("Opinion %s changed %d times in the last hour", entityID, count),
				Severity:    "medium",
				EntityID:    entityID,
				Details:     models.AnyMap{"change_count": count},
			})
		}
	}

	return issues, nil
}

func (d *DriftDetector) checkPromptInjectionMemories(ctx context.Context) ([]DriftIssue, error) {
	var issues []DriftIssue

	memories, err := d.store.Memory().Search(ctx, models.MemoryQuery{Limit: 10000})
	if err != nil {
		return nil, err
	}

	injectionPatterns := []string{
		"(?i)system\\s+instruction",
		"(?i)ignore\\s+previous",
		"(?i)you\\s+are\\s+now",
		"(?i)new\\s+persona",
		"(?i)override\\s+system",
	}

	for _, mem := range memories {
		for _, pattern := range injectionPatterns {
			matched, err := regexp.MatchString(pattern, mem.Memory.Content)
			if err != nil {
				continue
			}
			if matched {
				issues = append(issues, DriftIssue{
					Type:        "prompt_injection_memory",
					Description: fmt.Sprintf("Memory %s may contain prompt injection", mem.Memory.ID),
					Severity:    "high",
					EntityID:    mem.Memory.ID,
				})
				break
			}
		}
	}

	return issues, nil
}
