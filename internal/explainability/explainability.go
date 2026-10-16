package explainability

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

type Explanation struct {
	BeliefID          string    `json:"belief_id"`
	Title             string    `json:"title"`
	CurrentConfidence float64   `json:"current_confidence"`
	CurrentStatus     string    `json:"current_status"`
	CreatedAt         time.Time `json:"created_at"`
	LastUpdated       time.Time `json:"last_updated"`

	EvidenceTrail       []EvidenceItem   `json:"evidence_trail"`
	CompetingHypotheses []HypothesisInfo `json:"competing_hypotheses"`
	DecisionTree        *DecisionNode    `json:"decision_tree"`
	Explanation         string           `json:"explanation"`
	Summary             string           `json:"summary"`

	Timeline          []EventItem       `json:"timeline"`
	ConfidenceHistory []ConfidencePoint `json:"confidence_history"`
}

type EvidenceItem struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Content     string    `json:"content"`
	Weight      float64   `json:"weight"`
	Source      string    `json:"source"`
	Timestamp   time.Time `json:"timestamp"`
	Description string    `json:"description"`
}

type HypothesisInfo struct {
	ID         string         `json:"id"`
	Statement  string         `json:"statement"`
	Confidence float64        `json:"confidence"`
	Status     string         `json:"status"`
	Sources    []EvidenceItem `json:"sources"`
}

type DecisionNode struct {
	Decision   string          `json:"decision"`
	Condition  string          `json:"condition"`
	Outcome    string          `json:"outcome"`
	Confidence float64         `json:"confidence"`
	Children   []*DecisionNode `json:"children,omitempty"`
	Reasoning  string          `json:"reasoning"`
	Timestamp  time.Time       `json:"timestamp"`
}

type EventItem struct {
	Type        string        `json:"type"`
	Timestamp   time.Time     `json:"timestamp"`
	Description string        `json:"description"`
	Details     models.AnyMap `json:"details"`
}

type ConfidencePoint struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
	Cause string    `json:"cause"`
}

type ExplainabilityService struct {
	store storage.Store
}

func NewExplainabilityService(store storage.Store) *ExplainabilityService {
	return &ExplainabilityService{
		store: store,
	}
}

func (s *ExplainabilityService) ExplainBelief(ctx context.Context, beliefID string) (*Explanation, error) {
	belief, err := s.getBeliefByID(ctx, beliefID)
	if err != nil {
		return nil, fmt.Errorf("get belief: %w", err)
	}

	auditRecords, err := s.store.Audit().GetHistory(ctx, "belief", beliefID)
	if err != nil {
		auditRecords = []storage.AuditRecord{}
	}

	provenanceSteps, err := s.store.Audit().GetProvenance(ctx, beliefID)
	if err != nil {
		provenanceSteps = []storage.ProvenanceStep{}
	}

	explanation := &Explanation{
		BeliefID:          belief.ID,
		Title:             belief.Subject,
		CurrentConfidence: float64(belief.Confidence),
		CurrentStatus:     string(belief.Status),
		CreatedAt:         belief.CreatedAt,
		LastUpdated:       belief.UpdatedAt,
	}

	explanation.EvidenceTrail = s.buildEvidenceTrail(auditRecords)
	explanation.CompetingHypotheses = s.buildCompetingHypotheses(ctx, &belief, auditRecords)
	explanation.DecisionTree = s.buildDecisionTree(auditRecords, provenanceSteps)
	explanation.Timeline = s.buildTimeline(auditRecords)
	explanation.ConfidenceHistory = s.buildConfidenceHistory(auditRecords)

	explanation.Summary = s.generateSummary(explanation)
	explanation.Explanation = s.generateExplanationText(explanation)

	return explanation, nil
}

func (s *ExplainabilityService) getBeliefByID(ctx context.Context, beliefID string) (models.Belief, error) {
	beliefs, err := s.store.Mind().GetBeliefs(ctx, models.MindQuery{
		IDs: []string{beliefID},
	})
	if err == nil && len(beliefs) > 0 {
		return beliefs[0], nil
	}

	allBeliefs, err := s.store.Mind().GetBeliefs(ctx, models.MindQuery{Limit: 1000})
	if err != nil {
		return models.Belief{}, fmt.Errorf("failed to query beliefs: %w", err)
	}
	for _, b := range allBeliefs {
		if b.ID == beliefID || b.Subject == beliefID {
			return b, nil
		}
	}
	return models.Belief{}, fmt.Errorf("belief not found: %s", beliefID)
}

func (s *ExplainabilityService) buildEvidenceTrail(records []storage.AuditRecord) []EvidenceItem {
	var evidence []EvidenceItem
	for _, record := range records {
		if record.Action == "CREATE" || record.Action == "UPDATE" || strings.Contains(record.Reason, "evidence") {
			ev := EvidenceItem{
				ID:        record.EntityID,
				Type:      record.Action,
				Content:   s.extractContent(record.NewState),
				Weight:    0.5,
				Source:    record.Reason,
				Timestamp: time.Now(),
			}
			if ts, err := time.Parse(time.RFC3339, record.Timestamp); err == nil {
				ev.Timestamp = ts
			}
			evidence = append(evidence, ev)
		}
	}

	sort.Slice(evidence, func(i, j int) bool {
		return evidence[i].Timestamp.Before(evidence[j].Timestamp)
	})

	return evidence
}

func (s *ExplainabilityService) buildCompetingHypotheses(ctx context.Context, belief *models.Belief, records []storage.AuditRecord) []HypothesisInfo {
	var hypotheses []HypothesisInfo

	contradictions, err := s.store.Mind().GetContradictions(ctx, models.MindQuery{
		Limit: 100,
	})
	if err == nil {
		for _, c := range contradictions {
			if c.BeliefA == belief.Subject || c.BeliefB == belief.Subject {
				otherBelief := c.BeliefA
				if otherBelief == belief.Subject {
					otherBelief = c.BeliefB
				}
				hyp := HypothesisInfo{
					ID:         c.ID,
					Statement:  otherBelief,
					Confidence: float64(c.ConfidenceA),
					Status:     string(c.Status),
				}
				hypotheses = append(hypotheses, hyp)
			}
		}
	}

	return hypotheses
}

func (s *ExplainabilityService) buildDecisionTree(records []storage.AuditRecord, steps []storage.ProvenanceStep) *DecisionNode {
	if len(records) == 0 && len(steps) == 0 {
		return &DecisionNode{
			Decision:   "No decision history available",
			Condition:  "N/A",
			Outcome:    "Belief established without recorded history",
			Confidence: 0.5,
			Reasoning:  "This belief was created or updated outside the standard audit path.",
			Timestamp:  time.Now(),
		}
	}

	root := &DecisionNode{
		Decision:   "Belief evolution tracked",
		Condition:  "Evidence accumulation and conflict resolution",
		Outcome:    "Current belief state",
		Confidence: 0.0,
		Reasoning:  "Built from audit history and provenance records",
		Timestamp:  time.Now(),
	}

	for _, record := range records {
		change := s.extractConfidenceChange(record)
		if change != 0 {
			child := &DecisionNode{
				Decision:  fmt.Sprintf("Confidence change: %+.2f", change),
				Condition: record.Reason,
				Outcome: fmt.Sprintf("Confidence: %.0f%% -> %.0f%%",
					s.extractConfidence(record.PreviousState)*100,
					s.extractConfidence(record.NewState)*100),
				Confidence: s.extractConfidence(record.NewState),
				Reasoning:  fmt.Sprintf("Action: %s, Entity: %s", record.Action, record.EntityID),
				Timestamp:  s.parseTimestamp(record.Timestamp),
			}

			if len(record.SourceEventIDs) > 0 {
				child.Reasoning += fmt.Sprintf(", Sources: %v", record.SourceEventIDs)
			}

			root.Children = append(root.Children, child)
		}
	}

	for _, step := range steps {
		child := &DecisionNode{
			Decision:  "Provenance step",
			Condition: step.Step,
			Outcome:   step.Action,
			Reasoning: step.Reason,
			Timestamp: s.parseTimestamp(step.Timestamp),
		}
		root.Children = append(root.Children, child)
	}

	return root
}

func (s *ExplainabilityService) buildTimeline(records []storage.AuditRecord) []EventItem {
	var timeline []EventItem
	for _, record := range records {
		event := EventItem{
			Type:        record.Action,
			Description: record.Reason,
			Details:     record.NewState,
			Timestamp:   s.parseTimestamp(record.Timestamp),
		}
		timeline = append(timeline, event)
	}

	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Timestamp.Before(timeline[j].Timestamp)
	})

	return timeline
}

func (s *ExplainabilityService) buildConfidenceHistory(records []storage.AuditRecord) []ConfidencePoint {
	var history []ConfidencePoint
	for _, record := range records {
		conf := s.extractConfidence(record.NewState)
		if conf >= 0 {
			history = append(history, ConfidencePoint{
				Time:  s.parseTimestamp(record.Timestamp),
				Value: conf,
				Cause: record.Reason,
			})
		}
	}

	sort.Slice(history, func(i, j int) bool {
		return history[i].Time.Before(history[j].Time)
	})

	return history
}

func (s *ExplainabilityService) generateSummary(exp *Explanation) string {
	direction := "stable"
	if len(exp.ConfidenceHistory) >= 2 {
		first := exp.ConfidenceHistory[0].Value
		last := exp.ConfidenceHistory[len(exp.ConfidenceHistory)-1].Value
		if last > first+0.1 {
			direction = "strengthening"
		} else if last < first-0.1 {
			direction = "weakening"
		}
	}

	evidenceCount := len(exp.EvidenceTrail)
	competitorCount := len(exp.CompetingHypotheses)
	changeCount := len(exp.ConfidenceHistory)

	summary := fmt.Sprintf(
		"Belief '%s' is currently at %.0f%% confidence (%s). ",
		exp.Title, exp.CurrentConfidence*100, direction,
	)

	if changeCount > 0 {
		summary += fmt.Sprintf("It has undergone %d recorded changes, ", changeCount)
	}

	if evidenceCount > 0 {
		summary += fmt.Sprintf("accumulated %d evidence items, ", evidenceCount)
	}

	if competitorCount > 0 {
		summary += fmt.Sprintf("with %d competing alternatives identified, ", competitorCount)
	}

	summary += "making this a " + s.confidenceLevelDescriptor(exp.CurrentConfidence) + " belief."

	return summary
}

func (s *ExplainabilityService) generateExplanationText(exp *Explanation) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("BELIEF: %s\n\n", exp.Title))
	sb.WriteString(fmt.Sprintf("Current Status: %s\n", exp.CurrentStatus))
	sb.WriteString(fmt.Sprintf("Confidence: %.0f%%\n\n", exp.CurrentConfidence*100))

	if len(exp.ConfidenceHistory) > 0 {
		sb.WriteString("CONFIDENCE TRAJECTORY:\n")
		for i, point := range exp.ConfidenceHistory {
			if i == 0 {
				sb.WriteString(fmt.Sprintf("  Initially set at %.0f%%\n", point.Value*100))
			} else {
				prev := exp.ConfidenceHistory[i-1].Value
				delta := (point.Value - prev) * 100
				sign := "+"
				if delta < 0 {
					sign = ""
				}
				sb.WriteString(fmt.Sprintf("  Changed to %.0f%% (%s%.0f%%) — %s\n",
					point.Value*100, sign, delta, point.Cause))
			}
		}
		sb.WriteString("\n")
	}

	if len(exp.EvidenceTrail) > 0 {
		sb.WriteString("EVIDENCE TRAIL:\n")
		for i, ev := range exp.EvidenceTrail {
			sb.WriteString(fmt.Sprintf("  %d. [%s] %s\n", i+1,
				ev.Type, ev.Description))
		}
		sb.WriteString("\n")
	}

	if len(exp.CompetingHypotheses) > 0 {
		sb.WriteString("COMPETING HYPOTHESES:\n")
		for _, h := range exp.CompetingHypotheses {
			sb.WriteString(fmt.Sprintf("  • %s (%.0f%% confidence)\n", h.Statement, h.Confidence*100))
		}
		sb.WriteString("\n")
	}

	if exp.DecisionTree != nil {
		sb.WriteString("DECISION TREE:\n")
		s.walkDecisionTree(&sb, exp.DecisionTree, 0)
		sb.WriteString("\n")
	}

	return sb.String()
}

func (s *ExplainabilityService) walkDecisionTree(sb *strings.Builder, node *DecisionNode, depth int) {
	indent := strings.Repeat("  ", depth)
	sb.WriteString(fmt.Sprintf("%s├─ Decision: %s\n", indent, node.Decision))
	sb.WriteString(fmt.Sprintf("%s│  Condition: %s\n", indent, node.Condition))
	sb.WriteString(fmt.Sprintf("%s│  Outcome: %s\n", indent, node.Outcome))
	sb.WriteString(fmt.Sprintf("%s│  Confidence: %.0f%%\n", indent, node.Confidence*100))
	sb.WriteString(fmt.Sprintf("%s│  Reasoning: %s\n\n", indent, node.Reasoning))

	for _, child := range node.Children {
		s.walkDecisionTree(sb, child, depth+1)
	}
}

func (s *ExplainabilityService) confidenceLevelDescriptor(confidence float64) string {
	switch {
	case confidence >= 0.9:
		return "high-confidence established"
	case confidence >= 0.7:
		return "well-established"
	case confidence >= 0.5:
		return "moderately supported"
	case confidence >= 0.3:
		return "weakly supported"
	default:
		return "highly uncertain"
	}
}

func (s *ExplainabilityService) extractContent(state models.AnyMap) string {
	for _, key := range []string{"content", "statement", "subject", "text"} {
		if val, ok := state[key]; ok {
			if str, ok := val.(string); ok && str != "" {
				return str
			}
		}
	}
	return "No content available"
}

func (s *ExplainabilityService) extractConfidence(state models.AnyMap) float64 {
	if val, ok := state["confidence"]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case string:
			var result float64
			fmt.Sscanf(v, "%f", &result)
			return result
		}
	}
	return -1
}

func (s *ExplainabilityService) extractConfidenceChange(record storage.AuditRecord) float64 {
	oldConf := s.extractConfidence(record.PreviousState)
	newConf := s.extractConfidence(record.NewState)
	if oldConf >= 0 && newConf >= 0 {
		return newConf - oldConf
	}
	return 0
}

func (s *ExplainabilityService) parseTimestamp(ts string) time.Time {
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t
	}
	return time.Now()
}
