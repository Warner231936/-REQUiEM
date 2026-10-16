package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/internal/validation"
	"github.com/requiem/cognitive/pkg/models"
)

type Stage6Consolidation struct {
	store            storage.Store
	embeddingService *llm.EmbeddingService
	sanitizer        *validation.OutputSanitizer
	logger           *slog.Logger
	verificationPolicy *VerificationPolicy
}

func NewStage6Consolidation(store storage.Store, embeddingService *llm.EmbeddingService, logger *slog.Logger) *Stage6Consolidation {
	if logger == nil {
		logger = slog.Default()
	}
	return &Stage6Consolidation{
		store:            store,
		embeddingService: embeddingService,
		sanitizer:        validation.NewOutputSanitizer(),
		logger:           logger,
	}
}

func (s *Stage6Consolidation) SetVerificationPolicy(policy *VerificationPolicy) {
	s.verificationPolicy = policy
}

func (s *Stage6Consolidation) Name() string {
	return "consolidation"
}

func calculateImportance(
	content string,
	confidence models.Confidence,
	evidenceCount int,
	isExplicitUser bool,
	relatedToGoals bool,
	technicalSignificance bool,
) models.Importance {
	score := 0.0

	if confidence > 0 {
		score += float64(confidence) * 0.3
	}
	if isExplicitUser {
		score += 0.2
	}
	if relatedToGoals {
		score += 0.15
	}
	if technicalSignificance {
		score += 0.15
	}
	if evidenceCount > 0 {
		evidenceScore := float64(evidenceCount) * 0.05
		if evidenceScore > 0.1 {
			evidenceScore = 0.1
		}
		score += evidenceScore
	}

	if score < 0 {
		score = 0
	}
	if score > 1.0 {
		score = 1.0
	}

	return models.Importance(score)
}

func isTechnicalContent(content string) bool {
	technicalKeywords := []string{
		"architecture", "system", "database", "api", "code", "programming",
		"memory", "model", "weights", "storage", "cache", "transaction",
		"index", "query", "schema", "server", "client", "protocol",
	}
	lowerContent := strings.ToLower(content)
	for _, kw := range technicalKeywords {
		if strings.Contains(lowerContent, kw) {
			return true
		}
	}
	return false
}

func isRelatedToGoals(content string, beliefs []models.Belief) bool {
	for _, b := range beliefs {
		if strings.Contains(strings.ToLower(b.Subject), strings.ToLower(content)) {
			return true
		}
	}
	return false
}

func (s *Stage6Consolidation) Execute(ctx context.Context, input StageInput) (StageOutput, error) {
	startTime := time.Now()

	if input.PipelineInput.UserID != "" {
		ctx = storage.WithUserID(ctx, input.PipelineInput.UserID)
	}

	reflection := input.Stage3Output
	if reflection == nil {
		return NewStageOutput(5, true, time.Since(startTime).Milliseconds(), nil), nil
	}

	output := &models.ConsolidationOutput{
		ConversationID: reflection.ConversationID,
		TurnID:         reflection.TurnID,
		ConsolidatedAt: time.Now().UTC(),
		Actions:        []models.ConsolidationAction{},
	}

	sourceEventIDs := []string{input.TurnID}

	var txErr error
	err := s.store.WithRetry(ctx, func(ctx context.Context) error {
		txCtx, err := s.store.StartTransaction(ctx)
		if err != nil {
			s.logger.Warn("transactions not available, using non-transactional mode", "error", err)
			return s.executeConsolidation(txCtx, reflection, output, sourceEventIDs)
		}

		defer func() {
			if output.Success {
				if commitErr := s.store.CommitTransaction(txCtx); commitErr != nil {
					txErr = commitErr
				}
			} else {
				s.store.AbortTransaction(txCtx)
			}
		}()

		return s.executeConsolidation(txCtx, reflection, output, sourceEventIDs)
	})

	if err != nil {
		txErr = err
	}

	if txErr != nil {
		s.logger.Error("transaction error during consolidation", "error", txErr)
	}

	allSuccess := true
	for _, action := range output.Actions {
		if !action.Success {
			allSuccess = false
			s.logger.Warn("consolidation action failed",
				"entity_type", action.EntityType,
				"entity_id", action.EntityID,
				"error", action.Error,
			)
		}
	}
	output.Success = allSuccess

	s.logger.Info("consolidation completed",
		"actions", len(output.Actions),
		"success", allSuccess,
		"duration_ms", time.Since(startTime).Milliseconds(),
	)

	return NewStageOutput(5, true, time.Since(startTime).Milliseconds(), output), nil
}

func (s *Stage6Consolidation) executeConsolidation(
	ctx context.Context,
	reflection *models.ReflectionOutput,
	output *models.ConsolidationOutput,
	sourceEventIDs []string,
) error {
	for _, fact := range reflection.LearnedFacts {
		action := s.consolidateLearnedFact(ctx, fact, sourceEventIDs)
		output.Actions = append(output.Actions, action)
	}

	for _, beliefUpdate := range reflection.BeliefUpdates {
		action := s.consolidateBeliefUpdate(ctx, beliefUpdate, sourceEventIDs)
		output.Actions = append(output.Actions, action)
	}

	for _, newBelief := range reflection.NewBeliefs {
		s.logger.Debug("consolidating new belief", "content", newBelief.Content, "confidence", newBelief.Confidence, "subject", newBelief.Subject)
		action := s.consolidateNewBelief(ctx, newBelief, sourceEventIDs)
		output.Actions = append(output.Actions, action)
		if action.Success {
			s.detectContradictions(ctx, newBelief.Content, sourceEventIDs)
		}
	}

	for _, opinionUpdate := range reflection.OpinionUpdates {
		action := s.consolidateOpinionUpdate(ctx, opinionUpdate, sourceEventIDs)
		output.Actions = append(output.Actions, action)
	}

	for _, newOpinion := range reflection.NewOpinions {
		action := s.consolidateNewOpinion(ctx, newOpinion, sourceEventIDs)
		output.Actions = append(output.Actions, action)
	}

	for _, prefUpdate := range reflection.PreferenceUpdates {
		action := s.consolidatePreferenceUpdate(ctx, prefUpdate, sourceEventIDs)
		output.Actions = append(output.Actions, action)
	}

	for _, goalUpdate := range reflection.GoalUpdates {
		action := s.consolidateGoalUpdate(ctx, goalUpdate, sourceEventIDs)
		output.Actions = append(output.Actions, action)
	}

	for _, hypothesis := range reflection.Hypotheses {
		s.logger.Debug("consolidating hypothesis", "hypothesis", hypothesis.Hypothesis, "confidence", hypothesis.Confidence)
		action := s.consolidateHypothesis(ctx, hypothesis, sourceEventIDs)
		output.Actions = append(output.Actions, action)
	}

	for _, uncertainty := range reflection.Uncertainties {
		s.logger.Debug("consolidating uncertainty", "question", uncertainty.Question, "confidence", uncertainty.Confidence)
		action := s.consolidateUncertainty(ctx, uncertainty, sourceEventIDs)
		output.Actions = append(output.Actions, action)
	}

	for _, selfObs := range reflection.SelfObservations {
		action := s.consolidateSelfObservation(ctx, selfObs, sourceEventIDs)
		output.Actions = append(output.Actions, action)
	}

	for _, contradiction := range reflection.Contradictions {
		action := s.consolidateContradiction(ctx, contradiction, sourceEventIDs)
		output.Actions = append(output.Actions, action)
	}

	for _, memCandidate := range reflection.MemoryCandidates {
		s.logger.Debug("consolidating memory candidate", "content", memCandidate.Content, "confidence", memCandidate.Confidence, "type", memCandidate.Type)
		action := s.consolidateMemoryCandidate(ctx, memCandidate, sourceEventIDs)
		output.Actions = append(output.Actions, action)
	}

	return nil
}

func (s *Stage6Consolidation) consolidateLearnedFact(ctx context.Context, fact models.LearnedFact, sourceEventIDs []string) models.ConsolidationAction {
	action := models.ConsolidationAction{
		Type:       "CREATE",
		EntityType: "memory",
		Reason:     fact.Reason,
	}

	if fact.Content == "" {
		action.Success = false
		action.Error = "empty content"
		return action
	}

	if fact.Confidence < 0.15 {
		action.Success = false
		action.Error = "confidence too low"
		return action
	}

	memory := models.Memory{
		BaseRecord: models.NewBaseRecord(uuid.New().String()),
		UserID:     storage.GetUserIDFromContext(ctx),
		Type:       models.MemoryTypeSemantic,
		Content:    s.sanitizer.SanitizeString(fact.Content),
		Importance: calculateImportance(fact.Content, fact.Confidence, len(fact.Evidence), false, false, isTechnicalContent(fact.Content)),
		Confidence: fact.Confidence,
		Source:     models.Provenance{SourceEventIDs: sourceEventIDs, SourceType: string(models.SourceModelInferred)},
		Tags:       []string{"learned_fact"},
	}

	if s.embeddingService != nil {
		if embedding, err := s.embeddingService.Embed(ctx, fact.Content); err == nil {
			memory.Embedding = embedding
		}
	}

	if err := s.store.Memory().Create(ctx, memory); err != nil {
		action.Success = false
		action.Error = err.Error()
		return action
	}

	action.EntityID = memory.ID
	action.Success = true

	s.recordAudit(ctx, "memory", memory.ID, "CREATE", nil, memory, fact.Reason, sourceEventIDs)
	s.recordClaimForFact(ctx, fact, memory.ID, sourceEventIDs)

	return action
}

func (s *Stage6Consolidation) consolidateBeliefUpdate(ctx context.Context, update models.BeliefUpdate, sourceEventIDs []string) models.ConsolidationAction {
	action := models.ConsolidationAction{
		Type:       "UPDATE",
		EntityType: "belief",
		EntityID:   update.BeliefID,
		Reason:     update.Reason,
	}

	if update.BeliefID == "" {
		action.Success = false
		action.Error = "missing belief_id"
		return action
	}

	existing, err := s.getBeliefByContent(ctx, update.BeliefID)
	if err != nil {
		action.Success = false
		action.Error = fmt.Sprintf("belief not found: %s", update.BeliefID)
		return action
	}

	previousState := map[string]any{
		"confidence": existing.Confidence,
		"status":     existing.Status,
	}

	existing.Confidence = update.NewConfidence
	existing.SourceCount++
	existing.LastConfirmedAt = &models.Timestamp{Time: time.Now().UTC()}
	existing.UpdatedAt = time.Now().UTC()

	if update.NewConfidence < 0.2 {
		existing.Status = models.BeliefStatusChallenged
	}

	if err := s.store.Mind().UpdateBelief(ctx, existing); err != nil {
		action.Success = false
		action.Error = err.Error()
		return action
	}

	action.Success = true
	action.Details = models.AnyMap{
		"statement":   existing.Content,
		"title":       existing.Content,
		"confidence":  float64(existing.Confidence),
		"importance":  float64(existing.Importance),
		"category":    "belief",
		"status":      string(existing.Status),
		"evidence":    existing.Evidence,
		"updated_at":  existing.UpdatedAt,
		"source_type": string(existing.SourceType),
		"subject":     existing.Subject,
	}
	s.recordAudit(ctx, "belief", existing.ID, "UPDATE", previousState, existing, update.Reason, sourceEventIDs)

	return action
}

func (s *Stage6Consolidation) consolidateNewBelief(ctx context.Context, newBelief models.NewBelief, sourceEventIDs []string) models.ConsolidationAction {
	action := models.ConsolidationAction{
		Type:       "CREATE",
		EntityType: "belief",
		Reason:     newBelief.Reason,
	}

	if newBelief.Content == "" {
		action.Success = false
		action.Error = "empty content"
		return action
	}

	if newBelief.Confidence < 0.2 {
		action.Success = false
		action.Error = "confidence too low for new belief"
		return action
	}

	if len(newBelief.Evidence) == 0 {
		newBelief.Evidence = []string{"inferred from conversation"}
	}

	existing, err := s.store.Mind().GetBeliefs(ctx, models.MindQuery{
		Subjects: []string{newBelief.Subject},
		Limit:    5,
	})
	if err == nil && len(existing) > 0 {
		for _, e := range existing {
			if e.Content == newBelief.Content {
				action.Success = false
				action.Error = "duplicate belief"
				return action
			}
		}
	}

	belief := models.Belief{
		BaseRecord:  models.NewBaseRecord(uuid.New().String()),
		Subject:     s.sanitizer.SanitizeFieldName(newBelief.Subject),
		Content:     s.sanitizer.SanitizeString(newBelief.Content),
		Confidence:  newBelief.Confidence,
		Importance:  calculateImportance(newBelief.Content, newBelief.Confidence, len(newBelief.Evidence), false, isRelatedToGoals(newBelief.Subject, existing), isTechnicalContent(newBelief.Content)),
		Status:      models.BeliefStatusActive,
		Evidence:    []models.Evidence{{Type: "reflection", Content: s.sanitizer.SanitizeString(newBelief.Reason)}},
		SourceCount: 1,
		SourceType:  models.SourceModelInferred,
	}

	if err := s.store.Mind().CreateBelief(ctx, belief); err != nil {
		action.Success = false
		action.Error = err.Error()
		return action
	}

	action.EntityID = belief.ID
	action.Success = true
	action.Details = models.AnyMap{
		"statement":   belief.Content,
		"title":       belief.Content,
		"confidence":  float64(belief.Confidence),
		"importance":  float64(belief.Importance),
		"category":    "belief",
		"status":      string(belief.Status),
		"evidence":    belief.Evidence,
		"created_at":  belief.CreatedAt,
		"updated_at":  belief.UpdatedAt,
		"source_type": string(belief.SourceType),
		"subject":     belief.Subject,
	}
	s.recordAudit(ctx, "belief", belief.ID, "CREATE", nil, belief, newBelief.Reason, sourceEventIDs)
	if isTechnicalContent(belief.Content) {
		s.recordClaimForFact(ctx, models.LearnedFact{
			ProposedMutation: models.ProposedMutation{
				Reason:     newBelief.Reason,
				Confidence: newBelief.Confidence,
				Evidence:   newBelief.Evidence,
			},
			Content: belief.Content,
		}, belief.ID, sourceEventIDs)
	}

	return action
}

func (s *Stage6Consolidation) consolidateOpinionUpdate(ctx context.Context, update models.OpinionUpdate, sourceEventIDs []string) models.ConsolidationAction {
	action := models.ConsolidationAction{
		Type:       "UPDATE",
		EntityType: "opinion",
		EntityID:   update.OpinionID,
		Reason:     update.Reason,
	}

	existing, err := s.store.Mind().GetOpinions(ctx, models.MindQuery{Limit: 100})
	if err != nil {
		action.Success = false
		action.Error = err.Error()
		return action
	}

	var opinion *models.Opinion
	for i := range existing {
		if existing[i].ID == update.OpinionID {
			opinion = &existing[i]
			break
		}
	}

	if opinion == nil {
		action.Success = false
		action.Error = "opinion not found"
		return action
	}

	previousState := map[string]any{"strength": opinion.Strength}
	opinion.Strength = update.NewStrength
	opinion.UpdatedAt = time.Now().UTC()

	if err := s.store.Mind().UpdateOpinion(ctx, *opinion); err != nil {
		action.Success = false
		action.Error = err.Error()
		return action
	}

	action.Success = true
	s.recordAudit(ctx, "opinion", opinion.ID, "UPDATE", previousState, *opinion, update.Reason, sourceEventIDs)

	return action
}

func (s *Stage6Consolidation) consolidateNewOpinion(ctx context.Context, newOpinion models.NewOpinion, sourceEventIDs []string) models.ConsolidationAction {
	action := models.ConsolidationAction{
		Type:       "CREATE",
		EntityType: "opinion",
		Reason:     newOpinion.Reason,
	}

	if newOpinion.Content == "" {
		action.Success = false
		action.Error = "empty content"
		return action
	}

	if newOpinion.Confidence < 0.5 {
		action.Success = false
		action.Error = "confidence too low for new opinion"
		return action
	}

	opinion := models.Opinion{
		BaseRecord: models.NewBaseRecord(uuid.New().String()),
		Subject:    newOpinion.Subject,
		Content:    newOpinion.Content,
		Strength:   newOpinion.Confidence,
		Confidence: newOpinion.Confidence,
		Status:     models.StatusActive,
		SourceType: models.SourceModelInferred,
	}

	if err := s.store.Mind().CreateOpinion(ctx, opinion); err != nil {
		action.Success = false
		action.Error = err.Error()
		return action
	}

	action.EntityID = opinion.ID
	action.Success = true
	s.recordAudit(ctx, "opinion", opinion.ID, "CREATE", nil, opinion, newOpinion.Reason, sourceEventIDs)

	return action
}

func (s *Stage6Consolidation) consolidatePreferenceUpdate(ctx context.Context, update models.PreferenceUpdate, sourceEventIDs []string) models.ConsolidationAction {
	action := models.ConsolidationAction{
		Type:       "UPDATE",
		EntityType: "preference",
		EntityID:   update.PreferenceID,
		Reason:     update.Reason,
	}

	existing, err := s.store.Mind().GetPreferences(ctx, models.MindQuery{Limit: 100})
	if err != nil {
		action.Success = false
		action.Error = err.Error()
		return action
	}

	var pref *models.Preference
	for i := range existing {
		if existing[i].ID == update.PreferenceID {
			pref = &existing[i]
			break
		}
	}

	if pref == nil {
		action.Success = false
		action.Error = "preference not found"
		return action
	}

	previousState := map[string]any{"strength": pref.Strength}
	pref.Strength = update.NewStrength
	pref.UpdatedAt = time.Now().UTC()

	if err := s.store.Mind().UpdatePreference(ctx, *pref); err != nil {
		action.Success = false
		action.Error = err.Error()
		return action
	}

	action.Success = true
	s.recordAudit(ctx, "preference", pref.ID, "UPDATE", previousState, *pref, update.Reason, sourceEventIDs)

	return action
}

func (s *Stage6Consolidation) consolidateGoalUpdate(ctx context.Context, update models.GoalUpdate, sourceEventIDs []string) models.ConsolidationAction {
	action := models.ConsolidationAction{
		Type:       "UPDATE",
		EntityType: "goal",
		EntityID:   update.GoalID,
		Reason:     update.Reason,
	}

	existing, err := s.store.Mind().GetGoals(ctx, models.MindQuery{Limit: 100})
	if err != nil {
		action.Success = false
		action.Error = err.Error()
		return action
	}

	var goal *models.Goal
	for i := range existing {
		if existing[i].ID == update.GoalID {
			goal = &existing[i]
			break
		}
	}

	if goal == nil {
		action.Success = false
		action.Error = "goal not found"
		return action
	}

	previousState := map[string]any{"status": goal.Status, "progress": goal.Progress}
	goal.Status = update.NewStatus
	goal.Progress = update.NewProgress
	goal.UpdatedAt = time.Now().UTC()

	if err := s.store.Mind().UpdateGoal(ctx, *goal); err != nil {
		action.Success = false
		action.Error = err.Error()
		return action
	}

	action.Success = true
	s.recordAudit(ctx, "goal", goal.ID, "UPDATE", previousState, *goal, update.Reason, sourceEventIDs)

	return action
}

func (s *Stage6Consolidation) consolidateHypothesis(ctx context.Context, hypothesis models.NewHypothesis, sourceEventIDs []string) models.ConsolidationAction {
	action := models.ConsolidationAction{
		Type:       "CREATE",
		EntityType: "hypothesis",
		Reason:     hypothesis.Reason,
	}

	if hypothesis.Hypothesis == "" {
		action.Success = false
		action.Error = "empty hypothesis"
		return action
	}

	h := models.Hypothesis{
		BaseRecord: models.NewBaseRecord(uuid.New().String()),
		Subject:    hypothesis.Subject,
		Hypothesis: hypothesis.Hypothesis,
		Confidence: hypothesis.Confidence,
		Status:     models.HypothesisStatusUntested,
	}

	if err := s.store.Mind().CreateHypothesis(ctx, h); err != nil {
		action.Success = false
		action.Error = err.Error()
		return action
	}

	action.EntityID = h.ID
	action.Success = true
	action.Details = models.AnyMap{
		"id":          h.ID,
		"hypothesis":  h.Hypothesis,
		"title":       h.Hypothesis,
		"confidence":  float64(h.Confidence),
		"category":    "hypothesis",
		"status":      "hypothesis",
		"created_at":  h.CreatedAt,
		"updated_at":  h.UpdatedAt,
		"subject":     h.Subject,
		"entity_type": "hypothesis",
	}
	s.recordAudit(ctx, "hypothesis", h.ID, "CREATE", nil, h, hypothesis.Reason, sourceEventIDs)

	return action
}

func (s *Stage6Consolidation) consolidateUncertainty(ctx context.Context, uncertainty models.NewUncertainty, sourceEventIDs []string) models.ConsolidationAction {
	action := models.ConsolidationAction{
		Type:       "CREATE",
		EntityType: "uncertainty",
		Reason:     uncertainty.Reason,
	}

	if uncertainty.Question == "" {
		action.Success = false
		action.Error = "empty question"
		return action
	}

	u := models.Uncertainty{
		BaseRecord: models.NewBaseRecord(uuid.New().String()),
		Question:   uncertainty.Question,
		Status:     models.UncertaintyStatusUncertain,
	}

	if err := s.store.Mind().CreateUncertainty(ctx, u); err != nil {
		action.Success = false
		action.Error = err.Error()
		return action
	}

	action.EntityID = u.ID
	action.Success = true
	action.Details = models.AnyMap{
		"id":          u.ID,
		"question":    u.Question,
		"title":       u.Question,
		"category":    "uncertainty",
		"status":      string(u.Status),
		"created_at":  u.CreatedAt,
		"updated_at":  u.UpdatedAt,
		"entity_type": "uncertainty",
	}
	s.recordAudit(ctx, "uncertainty", u.ID, "CREATE", nil, u, uncertainty.Reason, sourceEventIDs)

	return action
}

func (s *Stage6Consolidation) consolidateSelfObservation(ctx context.Context, selfObs models.NewSelfObservation, sourceEventIDs []string) models.ConsolidationAction {
	action := models.ConsolidationAction{
		Type:       "CREATE",
		EntityType: "self_observation",
		Reason:     selfObs.Reason,
	}

	if selfObs.Observation == "" {
		action.Success = false
		action.Error = "empty observation"
		return action
	}

	so := models.SelfObservation{
		BaseRecord:    models.NewBaseRecord(uuid.New().String()),
		Subject:       selfObs.Subject,
		Observation:   selfObs.Observation,
		Confidence:    selfObs.Confidence,
		EvidenceCount: 1,
		Status:        models.StatusActive,
	}

	if err := s.store.Mind().CreateSelfObservation(ctx, so); err != nil {
		action.Success = false
		action.Error = err.Error()
		return action
	}

	action.EntityID = so.ID
	action.Success = true
	s.recordAudit(ctx, "self_observation", so.ID, "CREATE", nil, so, selfObs.Reason, sourceEventIDs)

	return action
}

func (s *Stage6Consolidation) consolidateContradiction(ctx context.Context, contradiction models.NewContradiction, sourceEventIDs []string) models.ConsolidationAction {
	action := models.ConsolidationAction{
		Type:       "CREATE",
		EntityType: "contradiction",
		Reason:     contradiction.Reason,
	}

	if contradiction.BeliefA == "" || contradiction.BeliefB == "" {
		action.Success = false
		action.Error = "missing belief references"
		return action
	}

	c := models.Contradiction{
		BaseRecord: models.NewBaseRecord(uuid.New().String()),
		BeliefA:    contradiction.BeliefA,
		BeliefB:    contradiction.BeliefB,
		Status:     models.ContradictionStatusUnresolved,
	}

	if err := s.store.Mind().CreateContradiction(ctx, c); err != nil {
		action.Success = false
		action.Error = err.Error()
		return action
	}

	action.EntityID = c.ID
	action.Success = true
	s.recordAudit(ctx, "contradiction", c.ID, "CREATE", nil, c, contradiction.Reason, sourceEventIDs)

	return action
}

func (s *Stage6Consolidation) consolidateMemoryCandidate(ctx context.Context, candidate models.MemoryCandidate, sourceEventIDs []string) models.ConsolidationAction {
	action := models.ConsolidationAction{
		Type:       "CREATE",
		EntityType: "memory",
		Reason:     candidate.Reason,
	}

	if candidate.Content == "" {
		action.Success = false
		action.Error = "empty content"
		return action
	}

	if candidate.Confidence < 0.15 {
		action.Success = false
		action.Error = "confidence too low"
		return action
	}

	memoryType := candidate.Type
	if memoryType == "" {
		memoryType = models.MemoryTypeEpisodic
	}

	memory := models.Memory{
		BaseRecord: models.NewBaseRecord(uuid.New().String()),
		UserID:     storage.GetUserIDFromContext(ctx),
		Type:       memoryType,
		Content:    s.sanitizer.SanitizeString(candidate.Content),
		Importance: calculateImportance(candidate.Content, candidate.Confidence, len(candidate.Evidence), false, false, isTechnicalContent(candidate.Content)),
		Confidence: candidate.Confidence,
		Source:     models.Provenance{SourceEventIDs: sourceEventIDs, SourceType: string(models.SourceModelInferred)},
		Tags:       candidate.Tags,
	}

	if s.embeddingService != nil {
		if embedding, err := s.embeddingService.Embed(ctx, candidate.Content); err == nil {
			memory.Embedding = embedding
		}
	}

	if err := s.store.Memory().Create(ctx, memory); err != nil {
		action.Success = false
		action.Error = err.Error()
		return action
	}

	action.EntityID = memory.ID
	action.Success = true
	s.recordAudit(ctx, "memory", memory.ID, "CREATE", nil, memory, candidate.Reason, sourceEventIDs)

	return action
}

func (s *Stage6Consolidation) getBeliefByContent(ctx context.Context, id string) (models.Belief, error) {
	beliefs, err := s.store.Mind().GetBeliefs(ctx, models.MindQuery{Limit: 100})
	if err != nil {
		return models.Belief{}, err
	}

	for _, b := range beliefs {
		if b.ID == id || b.Subject == id {
			return b, nil
		}
	}

	return models.Belief{}, fmt.Errorf("belief not found: %s", id)
}

func (s *Stage6Consolidation) recordAudit(
	ctx context.Context,
	entityType, entityID, action string,
	previousState, newState any,
	reason string,
	sourceEventIDs []string,
) {
	auditRecord := storage.AuditRecord{
		EntityType:     entityType,
		EntityID:       entityID,
		Action:         action,
		PreviousState:  models.AnyMap{},
		NewState:       models.AnyMap{},
		Reason:         reason,
		SourceEventIDs: sourceEventIDs,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}

	if previousState != nil {
		if b, err := marshalToMap(previousState); err == nil {
			auditRecord.PreviousState = b
		}
	}
	if newState != nil {
		if b, err := marshalToMap(newState); err == nil {
			auditRecord.NewState = b
		}
	}

	if err := s.store.Audit().RecordHistory(ctx, auditRecord); err != nil {
		s.logger.Warn("failed to record audit", "error", err)
	}
}

func (s *Stage6Consolidation) recordClaimForFact(ctx context.Context, fact models.LearnedFact, entityID string, sourceEventIDs []string) {
	if fact.Confidence < 0.3 {
		return
	}

	if !isTechnicalContent(fact.Content) {
		return
	}

	claim := models.Claim{
		BaseRecord: models.NewBaseRecord(uuid.New().String()),
		Subject:    fact.Content[:min(len(fact.Content), 100)],
		Content:    s.sanitizer.SanitizeString(fact.Content),
		Status:     models.ClaimProposed,
		Confidence: fact.Confidence,
		EvidenceIDs: fact.Evidence,
		Provenance: models.Provenance{
			SourceEventIDs: sourceEventIDs,
			SourceType:     string(models.SourceModelInferred),
		},
	}

	if err := s.store.Claim().CreateClaim(ctx, claim); err != nil {
		s.logger.Warn("failed to record claim for learned fact", "error", err)
		return
	}

	s.logger.Debug("recorded claim for learned fact",
		"claim_id", claim.ID,
		"entity_id", entityID,
		"confidence", claim.Confidence,
	)

	if s.verificationPolicy != nil {
		eval := s.verificationPolicy.EvaluateClaim(claim)
		if eval.ShouldVerify {
			s.logger.Info("claim requires verification",
				"claim_id", claim.ID,
				"subject", claim.Subject,
				"confidence", claim.Confidence,
				"reason", eval.Reason,
			)
			evRecord := models.EvidenceRecord{
				BaseRecord:         models.NewBaseRecord(models.NewID()),
				ClaimID:            claim.ID,
				Subject:            claim.Subject,
				EvidenceSource:     models.EvidenceSourcePolicy,
				ClaimContent:       eval.Reason,
				ObservedResult:     "claim requires further verification",
				VerificationMethod: string(models.MethodAbstain),
				Validity:           models.ValidityPending,
				Confidence:         0.5,
				Provenance: models.Provenance{
					SourceEventIDs: sourceEventIDs,
					SourceType:     "pipeline_policy",
				},
			}
			if err := s.store.Evidence().CreateRecord(ctx, evRecord); err != nil {
				s.logger.Warn("failed to record evidence for claim requiring verification",
					"claim_id", claim.ID, "error", err)
			}
		}
	}
}

func marshalToMap(v any) (models.AnyMap, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var m models.AnyMap
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Stage6Consolidation) detectContradictions(ctx context.Context, newBeliefContent string, sourceEventIDs []string) {
	existingBeliefs, err := s.store.Mind().GetBeliefs(ctx, models.MindQuery{Limit: 100})
	if err != nil {
		return
	}

	sanitizedNew := s.sanitizer.SanitizeString(newBeliefContent)
	for _, existing := range existingBeliefs {
		sanitizedExisting := s.sanitizer.SanitizeString(existing.Content)
		if isContradictory(sanitizedNew, sanitizedExisting) {
			contradiction := models.Contradiction{
				BaseRecord:  models.NewBaseRecord(uuid.New().String()),
				BeliefA:     s.sanitizer.SanitizeString(newBeliefContent),
				BeliefB:     s.sanitizer.SanitizeString(existing.Content),
				ConfidenceA: 0.5,
				ConfidenceB: existing.Confidence,
				Status:      models.ContradictionStatusUnresolved,
			}
			s.store.Mind().CreateContradiction(ctx, contradiction)
			s.logger.Info("automatic contradiction detected",
				"belief_a", s.sanitizer.SanitizeForLog(newBeliefContent),
				"belief_b", s.sanitizer.SanitizeForLog(existing.Content),
			)
		}
	}
}

func isContradictory(a, b string) bool {
	negationPatterns := []string{
		"not ", "no ", "never ", "cannot ", "can't ", "won't ", "don't ", "doesn't ",
		"isn't ", "aren't ", "wasn't ", "weren't ", "shouldn't ", "wouldn't ",
	}

	aHasNegation := false
	bHasNegation := false
	for _, pattern := range negationPatterns {
		if strings.Contains(a, pattern) {
			aHasNegation = true
		}
		if strings.Contains(b, pattern) {
			bHasNegation = true
		}
	}

	if aHasNegation != bHasNegation {
		aWords := strings.Fields(a)
		bWords := strings.Fields(b)
		commonWords := 0
		for _, aw := range aWords {
			for _, bw := range bWords {
				if aw == bw && len(aw) > 3 {
					commonWords++
				}
			}
		}
		if commonWords >= 3 {
			return true
		}
	}

	return false
}
