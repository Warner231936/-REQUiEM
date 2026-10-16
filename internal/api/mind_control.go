package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

type MindController struct {
	store  *storage.MongoStore
	logger *slog.Logger
}

func NewMindController(store *storage.MongoStore, logger *slog.Logger) *MindController {
	if logger == nil {
		logger = slog.Default()
	}
	return &MindController{
		store:  store,
		logger: logger,
	}
}

func (mc *MindController) Register(mux *http.ServeMux) {
	mc.RegisterAsProtected(mux)
}

func (mc *MindController) RegisterAsProtected(mux *http.ServeMux) {
	mux.HandleFunc("/api/mind/beliefs/approve", mc.handleApproveBelief)
	mux.HandleFunc("/api/mind/beliefs/reject", mc.handleRejectBelief)
	mux.HandleFunc("/api/mind/beliefs/edit", mc.handleEditBelief)
	mux.HandleFunc("/api/mind/beliefs/archive", mc.handleArchiveBelief)
	mux.HandleFunc("/api/mind/beliefs/restore", mc.handleRestoreBelief)
	mux.HandleFunc("/api/mind/beliefs/merge", mc.handleMergeBeliefs)
	mux.HandleFunc("/api/mind/beliefs/invalidate", mc.handleInvalidateBelief)
	mux.HandleFunc("/api/mind/contradictions/resolve", mc.handleResolveContradiction)
	mux.HandleFunc("/api/mind/opinions/approve", mc.handleApproveOpinion)
	mux.HandleFunc("/api/mind/opinions/reject", mc.handleRejectOpinion)
	mux.HandleFunc("/api/mind/snapshots", mc.handleCreateSnapshot)
}

func (mc *MindController) handleApproveBelief(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		BeliefID string `json:"belief_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	mc.logger.Info("approving belief", "belief_id", req.BeliefID)

	beliefs, err := mc.store.Mind().GetBeliefs(r.Context(), models.MindQuery{Limit: 1000})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve beliefs")
		return
	}

	for _, b := range beliefs {
		if b.ID == req.BeliefID {
			b.Status = models.BeliefStatusActive
			b.Confidence = min(b.Confidence+0.1, 1.0)
			if err := mc.store.Mind().UpdateBelief(r.Context(), b); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update belief")
				return
			}
			writeJSON(w, http.StatusOK, b)
			return
		}
	}

	writeError(w, http.StatusNotFound, "belief not found")
}

func (mc *MindController) handleRejectBelief(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		BeliefID string `json:"belief_id"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	mc.logger.Info("rejecting belief", "belief_id", req.BeliefID, "reason", req.Reason)

	beliefs, err := mc.store.Mind().GetBeliefs(r.Context(), models.MindQuery{Limit: 1000})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve beliefs")
		return
	}

	for _, b := range beliefs {
		if b.ID == req.BeliefID {
			b.Status = models.BeliefStatusArchived
			if err := mc.store.Mind().UpdateBelief(r.Context(), b); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update belief")
				return
			}
			writeJSON(w, http.StatusOK, b)
			return
		}
	}

	writeError(w, http.StatusNotFound, "belief not found")
}

func (mc *MindController) handleEditBelief(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		BeliefID   string  `json:"belief_id"`
		Content    string  `json:"content"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	mc.logger.Info("editing belief", "belief_id", req.BeliefID)

	beliefs, err := mc.store.Mind().GetBeliefs(r.Context(), models.MindQuery{Limit: 1000})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve beliefs")
		return
	}

	for _, b := range beliefs {
		if b.ID == req.BeliefID {
			if req.Content != "" {
				b.Content = req.Content
			}
			if req.Confidence > 0 {
				b.Confidence = models.Confidence(req.Confidence)
			}
			if err := mc.store.Mind().UpdateBelief(r.Context(), b); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update belief")
				return
			}
			writeJSON(w, http.StatusOK, b)
			return
		}
	}

	writeError(w, http.StatusNotFound, "belief not found")
}

func (mc *MindController) handleArchiveBelief(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		BeliefID string `json:"belief_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	mc.logger.Info("archiving belief", "belief_id", req.BeliefID)

	beliefs, err := mc.store.Mind().GetBeliefs(r.Context(), models.MindQuery{Limit: 1000})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve beliefs")
		return
	}

	for _, b := range beliefs {
		if b.ID == req.BeliefID {
			b.Status = models.BeliefStatusArchived
			if err := mc.store.Mind().UpdateBelief(r.Context(), b); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update belief")
				return
			}
			writeJSON(w, http.StatusOK, b)
			return
		}
	}

	writeError(w, http.StatusNotFound, "belief not found")
}

func (mc *MindController) handleRestoreBelief(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		BeliefID string `json:"belief_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	mc.logger.Info("restoring belief", "belief_id", req.BeliefID)

	beliefs, err := mc.store.Mind().GetBeliefs(r.Context(), models.MindQuery{Limit: 1000})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve beliefs")
		return
	}

	for _, b := range beliefs {
		if b.ID == req.BeliefID {
			b.Status = models.BeliefStatusActive
			if err := mc.store.Mind().UpdateBelief(r.Context(), b); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update belief")
				return
			}
			writeJSON(w, http.StatusOK, b)
			return
		}
	}

	writeError(w, http.StatusNotFound, "belief not found")
}

func (mc *MindController) handleMergeBeliefs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		BeliefIDA string `json:"belief_id_a"`
		BeliefIDB string `json:"belief_id_b"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	mc.logger.Info("merging beliefs", "belief_id_a", req.BeliefIDA, "belief_id_b", req.BeliefIDB)

	beliefs, err := mc.store.Mind().GetBeliefs(r.Context(), models.MindQuery{Limit: 1000})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve beliefs")
		return
	}

	var beliefA, beliefB *models.Belief
	for i := range beliefs {
		if beliefs[i].ID == req.BeliefIDA {
			beliefA = &beliefs[i]
		}
		if beliefs[i].ID == req.BeliefIDB {
			beliefB = &beliefs[i]
		}
	}

	if beliefA == nil || beliefB == nil {
		writeError(w, http.StatusNotFound, "one or both beliefs not found")
		return
	}

	merged := models.Belief{
		BaseRecord:  models.NewBaseRecord(uuid.New().String()),
		Subject:     beliefA.Subject,
		Content:     beliefA.Content + " | " + beliefB.Content,
		Confidence:  models.Confidence((float64(beliefA.Confidence) + float64(beliefB.Confidence)) / 2),
		Importance:  models.Importance(max(float64(beliefA.Importance), float64(beliefB.Importance))),
		Status:      models.BeliefStatusActive,
		SourceCount: beliefA.SourceCount + beliefB.SourceCount,
		SourceType:  models.SourceModelInferred,
	}

	if err := mc.store.Mind().CreateBelief(r.Context(), merged); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create merged belief")
		return
	}

	beliefA.Status = models.BeliefStatusSuperseded
	beliefB.Status = models.BeliefStatusSuperseded
	mc.store.Mind().UpdateBelief(r.Context(), *beliefA)
	mc.store.Mind().UpdateBelief(r.Context(), *beliefB)

	writeJSON(w, http.StatusOK, merged)
}

func (mc *MindController) handleInvalidateBelief(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		BeliefID string `json:"belief_id"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	mc.logger.Info("invalidating belief", "belief_id", req.BeliefID, "reason", req.Reason)

	beliefs, err := mc.store.Mind().GetBeliefs(r.Context(), models.MindQuery{Limit: 1000})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve beliefs")
		return
	}

	for _, b := range beliefs {
		if b.ID == req.BeliefID {
			b.Status = models.BeliefStatusSuperseded
			b.Confidence = 0.0
			if err := mc.store.Mind().UpdateBelief(r.Context(), b); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update belief")
				return
			}
			writeJSON(w, http.StatusOK, b)
			return
		}
	}

	writeError(w, http.StatusNotFound, "belief not found")
}

func (mc *MindController) handleResolveContradiction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		ContradictionID string `json:"contradiction_id"`
		Resolution      string `json:"resolution"`
		PreferredBelief string `json:"preferred_belief"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	mc.logger.Info("resolving contradiction", "contradiction_id", req.ContradictionID)

	contradictions, err := mc.store.Mind().GetContradictions(r.Context(), models.MindQuery{Limit: 1000})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve contradictions")
		return
	}

	for _, c := range contradictions {
		if c.ID == req.ContradictionID {
			c.Status = models.ContradictionStatusContextualized
			c.Resolution = req.Resolution
			if req.PreferredBelief == "a" {
				c.Status = models.ContradictionStatusAPreferred
			} else if req.PreferredBelief == "b" {
				c.Status = models.ContradictionStatusBPreferred
			}
			if err := mc.store.Mind().UpdateContradiction(r.Context(), c); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update contradiction")
				return
			}
			writeJSON(w, http.StatusOK, c)
			return
		}
	}

	writeError(w, http.StatusNotFound, "contradiction not found")
}

func (mc *MindController) handleApproveOpinion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		OpinionID string `json:"opinion_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	opinions, err := mc.store.Mind().GetOpinions(r.Context(), models.MindQuery{Limit: 1000})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve opinions")
		return
	}

	for _, o := range opinions {
		if o.ID == req.OpinionID {
			o.Status = models.StatusActive
			o.Strength = min(o.Strength+0.1, 1.0)
			if err := mc.store.Mind().UpdateOpinion(r.Context(), o); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update opinion")
				return
			}
			writeJSON(w, http.StatusOK, o)
			return
		}
	}

	writeError(w, http.StatusNotFound, "opinion not found")
}

func (mc *MindController) handleRejectOpinion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		OpinionID string `json:"opinion_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	opinions, err := mc.store.Mind().GetOpinions(r.Context(), models.MindQuery{Limit: 1000})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve opinions")
		return
	}

	for _, o := range opinions {
		if o.ID == req.OpinionID {
			o.Status = models.StatusArchived
			if err := mc.store.Mind().UpdateOpinion(r.Context(), o); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update opinion")
				return
			}
			writeJSON(w, http.StatusOK, o)
			return
		}
	}

	writeError(w, http.StatusNotFound, "opinion not found")
}

func (mc *MindController) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "snapshot endpoint - use SnapshotService directly"})
}

func min(a, b models.Confidence) models.Confidence {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func extractIDFromPath(path string, prefix string) string {
	return strings.TrimPrefix(path, prefix)
}
