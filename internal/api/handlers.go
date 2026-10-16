package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/requiem/cognitive/internal/cognitive/pipeline"
	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

type Handler struct {
	pipeline         *pipeline.Pipeline
	storage          *storage.MongoStore
	embeddingService *llm.EmbeddingService
	logger           *slog.Logger
}

func NewHandler(p *pipeline.Pipeline, store *storage.MongoStore, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		pipeline: p,
		storage:  store,
		logger:   logger,
	}
}

func NewHandlerWithEmbeddings(p *pipeline.Pipeline, store *storage.MongoStore, embeddingService *llm.EmbeddingService, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		pipeline:         p,
		storage:          store,
		embeddingService: embeddingService,
		logger:           logger,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/chat", h.handleChat)
	mux.HandleFunc("/api/mind", h.handleMind)
	mux.HandleFunc("/api/mind/beliefs", h.handleBeliefs)
	mux.HandleFunc("/api/mind/opinions", h.handleOpinions)
	mux.HandleFunc("/api/mind/preferences", h.handlePreferences)
	mux.HandleFunc("/api/mind/values", h.handleValues)
	mux.HandleFunc("/api/mind/goals", h.handleGoals)
	mux.HandleFunc("/api/mind/hypotheses", h.handleHypotheses)
	mux.HandleFunc("/api/mind/uncertainties", h.handleUncertainties)
	mux.HandleFunc("/api/mind/self", h.handleSelfObservations)
	mux.HandleFunc("/api/mind/contradictions", h.handleContradictions)
	mux.HandleFunc("/api/mind/history", h.handleMindHistory)
	mux.HandleFunc("/api/memory", h.handleMemory)
	mux.HandleFunc("/api/memory/", h.handleMemoryByID)
	mux.HandleFunc("/api/cognitive/", h.handleCognitiveDebug)
	mux.HandleFunc("/api/embeddings/backfill", h.handleEmbeddingsBackfill)
}

func (h *Handler) RegisterPublic(mux *http.ServeMux) {
	mux.HandleFunc("/api/chat", h.handleChat)
	mux.HandleFunc("/api/cognitive/", h.handleCognitiveDebug)
}

func (h *Handler) RegisterProtected(mux *http.ServeMux) {
	mux.HandleFunc("/api/mind", h.handleMind)
	mux.HandleFunc("/api/mind/beliefs", h.handleBeliefs)
	mux.HandleFunc("/api/mind/opinions", h.handleOpinions)
	mux.HandleFunc("/api/mind/preferences", h.handlePreferences)
	mux.HandleFunc("/api/mind/values", h.handleValues)
	mux.HandleFunc("/api/mind/goals", h.handleGoals)
	mux.HandleFunc("/api/mind/hypotheses", h.handleHypotheses)
	mux.HandleFunc("/api/mind/uncertainties", h.handleUncertainties)
	mux.HandleFunc("/api/mind/self", h.handleSelfObservations)
	mux.HandleFunc("/api/mind/contradictions", h.handleContradictions)
	mux.HandleFunc("/api/mind/history", h.handleMindHistory)
	mux.HandleFunc("/api/memory", h.handleMemory)
	mux.HandleFunc("/api/memory/", h.handleMemoryByID)
	mux.HandleFunc("/api/embeddings/backfill", h.handleEmbeddingsBackfill)
}

func (h *Handler) HandleChat(w http.ResponseWriter, r *http.Request) {
	h.handleChat(w, r)
}

func (h *Handler) HandleMind(w http.ResponseWriter, r *http.Request) {
	h.handleMind(w, r)
}

func (h *Handler) HandleBeliefs(w http.ResponseWriter, r *http.Request) {
	h.handleBeliefs(w, r)
}

func (h *Handler) HandleOpinions(w http.ResponseWriter, r *http.Request) {
	h.handleOpinions(w, r)
}

func (h *Handler) HandlePreferences(w http.ResponseWriter, r *http.Request) {
	h.handlePreferences(w, r)
}

func (h *Handler) HandleValues(w http.ResponseWriter, r *http.Request) {
	h.handleValues(w, r)
}

func (h *Handler) HandleGoals(w http.ResponseWriter, r *http.Request) {
	h.handleGoals(w, r)
}

func (h *Handler) HandleHypotheses(w http.ResponseWriter, r *http.Request) {
	h.handleHypotheses(w, r)
}

func (h *Handler) HandleUncertainties(w http.ResponseWriter, r *http.Request) {
	h.handleUncertainties(w, r)
}

func (h *Handler) HandleSelfObservations(w http.ResponseWriter, r *http.Request) {
	h.handleSelfObservations(w, r)
}

func (h *Handler) HandleContradictions(w http.ResponseWriter, r *http.Request) {
	h.handleContradictions(w, r)
}

func (h *Handler) HandleMindHistory(w http.ResponseWriter, r *http.Request) {
	h.handleMindHistory(w, r)
}

func (h *Handler) HandleMemory(w http.ResponseWriter, r *http.Request) {
	h.handleMemory(w, r)
}

func (h *Handler) HandleMemoryByID(w http.ResponseWriter, r *http.Request) {
	h.handleMemoryByID(w, r)
}

func (h *Handler) HandleCognitiveDebug(w http.ResponseWriter, r *http.Request) {
	h.handleCognitiveDebug(w, r)
}

func (h *Handler) HandleEmbeddingsBackfill(w http.ResponseWriter, r *http.Request) {
	h.handleEmbeddingsBackfill(w, r)
}

func (h *Handler) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		ConversationID string        `json:"conversation_id"`
		Message        string        `json:"message"`
		Metadata       models.AnyMap `json:"metadata,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "message required")
		return
	}

	convID := req.ConversationID
	if convID == "" {
		if cookie, err := r.Cookie("requiem_conv_id"); err == nil && cookie.Value != "" {
			convID = cookie.Value
		} else {
			convID = models.NewID()
		}
	}
	turnID := models.NewID()

	http.SetCookie(w, &http.Cookie{
		Name:     "requiem_conv_id",
		Value:    convID,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	ctx := r.Context()

	input := models.PipelineInput{
		ConversationID: convID,
		TurnID:         turnID,
		UserMessage:    req.Message,
		Timestamp:      time.Now().UTC(),
		Metadata:       req.Metadata,
	}

	output, err := h.pipeline.Execute(ctx, input)
	if err != nil {
		h.logger.Error("pipeline execution failed", "error", err)
		writeError(w, http.StatusInternalServerError, "pipeline error")
		return
	}

	userMsg := models.ConversationMessage{
		ID:             models.NewID(),
		ConversationID: convID,
		Role:           "user",
		Content:        req.Message,
		Timestamp:      time.Now().UTC(),
	}
	if err := h.storage.Conversation().AddMessage(ctx, convID, userMsg); err != nil {
		h.logger.Warn("failed to persist user message", "error", err, "conversation_id", convID)
	}

	assistantMsg := models.ConversationMessage{
		ID:             models.NewID(),
		ConversationID: convID,
		Role:           "assistant",
		Content:        output.Response,
		Timestamp:      time.Now().UTC(),
	}
	if err := h.storage.Conversation().AddMessage(ctx, convID, assistantMsg); err != nil {
		h.logger.Warn("failed to persist assistant message", "error", err, "conversation_id", convID)
	}

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		h.persistUserFacts(bgCtx, convID, turnID, req.Message)
	}()

	writeJSON(w, http.StatusOK, models.AnyMap{
		"conversation_id": convID,
		"turn_id":         turnID,
		"response":        output.Response,
		"stage_outputs":   output.StageOutputs,
		"duration_ms":     output.DurationMs,
		"completed_at":    output.CompletedAt,
	})
}

func (h *Handler) persistUserFacts(ctx context.Context, convID, turnID, userMessage string) {
	if h.embeddingService == nil {
		return
	}

	embedding, err := h.embeddingService.Embed(ctx, userMessage)
	if err != nil {
		return
	}

	if len(embedding) == 0 {
		return
	}

	words := len(strings.Fields(userMessage))
	tokens := words * 4 / 3
	importance := 0.5
	if tokens > 50 {
		importance = 0.7
	}
	lowerMsg := strings.ToLower(userMessage)
	if strings.Contains(lowerMsg, "name") ||
		strings.Contains(lowerMsg, "i am") ||
		strings.Contains(lowerMsg, "i love") ||
		strings.Contains(lowerMsg, "i like") ||
		strings.Contains(lowerMsg, "i am called") {
		importance = 0.9
	}

	memory := models.Memory{
		BaseRecord:     models.NewBaseRecord(models.NewID()),
		Type:           models.MemoryTypeEpisodic,
		Content:        userMessage,
		ConversationID: convID,
		TurnID:         turnID,
		Importance:     models.Importance(importance),
		Confidence:     0.9,
		Source:         models.Provenance{SourceEventIDs: []string{turnID}, SourceType: string(models.SourceUserStated)},
		Tags:           []string{"conversation", "user_facts"},
		Embedding:      embedding,
	}

	if err := h.storage.Memory().Create(ctx, memory); err != nil {
		h.logger.Warn("failed to persist user fact memory", "error", err)
	}
}

func (h *Handler) handleMind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()
	result := models.AnyMap{}

	beliefs, err := h.storage.Mind().GetBeliefs(ctx, models.MindQuery{Limit: 50})
	if err == nil {
		if beliefs == nil {
			beliefs = []models.Belief{}
		}
		result["beliefs"] = beliefs
	} else {
		result["beliefs"] = []models.Belief{}
		result["beliefs_error"] = err.Error()
	}

	opinions, err := h.storage.Mind().GetOpinions(ctx, models.MindQuery{Limit: 50})
	if err == nil {
		if opinions == nil {
			opinions = []models.Opinion{}
		}
		result["opinions"] = opinions
	} else {
		result["opinions"] = []models.Opinion{}
		result["opinions_error"] = err.Error()
	}

	preferences, err := h.storage.Mind().GetPreferences(ctx, models.MindQuery{Limit: 50})
	if err == nil {
		if preferences == nil {
			preferences = []models.Preference{}
		}
		result["preferences"] = preferences
	} else {
		result["preferences"] = []models.Preference{}
		result["preferences_error"] = err.Error()
	}

	goals, err := h.storage.Mind().GetGoals(ctx, models.MindQuery{Limit: 50})
	if err == nil {
		if goals == nil {
			goals = []models.Goal{}
		}
		result["goals"] = goals
	} else {
		result["goals"] = []models.Goal{}
		result["goals_error"] = err.Error()
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleBeliefs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	beliefs, err := h.storage.Mind().GetBeliefs(r.Context(), models.MindQuery{Limit: 100})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve beliefs")
		return
	}

	if beliefs == nil {
		beliefs = []models.Belief{}
	}
	writeJSON(w, http.StatusOK, beliefs)
}

func (h *Handler) handleOpinions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	opinions, err := h.storage.Mind().GetOpinions(r.Context(), models.MindQuery{Limit: 100})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve opinions")
		return
	}

	if opinions == nil {
		opinions = []models.Opinion{}
	}
	writeJSON(w, http.StatusOK, opinions)
}

func (h *Handler) handlePreferences(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	preferences, err := h.storage.Mind().GetPreferences(r.Context(), models.MindQuery{Limit: 100})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve preferences")
		return
	}

	if preferences == nil {
		preferences = []models.Preference{}
	}
	writeJSON(w, http.StatusOK, preferences)
}

func (h *Handler) handleValues(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	values, err := h.storage.Mind().GetValues(r.Context(), models.MindQuery{Limit: 100})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve values")
		return
	}

	if values == nil {
		values = []models.Value{}
	}
	writeJSON(w, http.StatusOK, values)
}

func (h *Handler) handleGoals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	goals, err := h.storage.Mind().GetGoals(r.Context(), models.MindQuery{Limit: 100})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve goals")
		return
	}

	if goals == nil {
		goals = []models.Goal{}
	}
	writeJSON(w, http.StatusOK, goals)
}

func (h *Handler) handleHypotheses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	hypotheses, err := h.storage.Mind().GetHypotheses(r.Context(), models.MindQuery{Limit: 100})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve hypotheses")
		return
	}

	if hypotheses == nil {
		hypotheses = []models.Hypothesis{}
	}
	writeJSON(w, http.StatusOK, hypotheses)
}

func (h *Handler) handleUncertainties(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uncertainties, err := h.storage.Mind().GetUncertainties(r.Context(), models.MindQuery{Limit: 100})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve uncertainties")
		return
	}

	if uncertainties == nil {
		uncertainties = []models.Uncertainty{}
	}
	writeJSON(w, http.StatusOK, uncertainties)
}

func (h *Handler) handleSelfObservations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	observations, err := h.storage.Mind().GetSelfObservations(r.Context(), models.MindQuery{Limit: 100})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve self-observations")
		return
	}

	if observations == nil {
		observations = []models.SelfObservation{}
	}
	writeJSON(w, http.StatusOK, observations)
}

func (h *Handler) handleContradictions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	contradictions, err := h.storage.Mind().GetContradictions(r.Context(), models.MindQuery{Limit: 100})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve contradictions")
		return
	}

	if contradictions == nil {
		contradictions = []models.Contradiction{}
	}
	writeJSON(w, http.StatusOK, contradictions)
}

func (h *Handler) handleMindHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	entityID := r.URL.Query().Get("entity_id")
	if entityID == "" {
		writeError(w, http.StatusBadRequest, "entity_id required")
		return
	}

	history, err := h.storage.Audit().GetHistory(r.Context(), "", entityID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve history")
		return
	}

	writeJSON(w, http.StatusOK, history)
}

func (h *Handler) handleMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	query := models.MemoryQuery{
		Limit: 50,
	}

	if memType := r.URL.Query().Get("type"); memType != "" {
		query.Type = models.MemoryType(memType)
	}
	if tags := r.URL.Query().Get("tags"); tags != "" {
		query.Tags = strings.Split(tags, ",")
	}

	results, err := h.storage.Memory().Search(r.Context(), query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search memories")
		return
	}

	writeJSON(w, http.StatusOK, results)
}

func (h *Handler) handleMemoryByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/memory/")
	parts := strings.SplitN(path, "/", 2)

	if len(parts) < 1 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "memory ID required")
		return
	}

	id := parts[0]

	if len(parts) == 2 && parts[1] == "provenance" {
		provenance, err := h.storage.Audit().GetProvenance(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get provenance")
			return
		}
		writeJSON(w, http.StatusOK, provenance)
		return
	}

	memory, err := h.storage.Memory().Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "memory not found")
		return
	}

	writeJSON(w, http.StatusOK, memory)
}

func (h *Handler) handleCognitiveDebug(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	turnID := strings.TrimPrefix(r.URL.Path, "/api/cognitive/")
	if turnID == "" {
		writeError(w, http.StatusBadRequest, "turn_id required")
		return
	}

	run, err := h.storage.PipelineRun().GetRun(r.Context(), turnID)
	if err != nil {
		writeError(w, http.StatusNotFound, "pipeline run not found")
		return
	}

	writeJSON(w, http.StatusOK, run)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, models.AnyMap{"status": "ok"})
}

func (h *Handler) handleEmbeddingsBackfill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.embeddingService == nil {
		writeError(w, http.StatusServiceUnavailable, "embedding service not configured")
		return
	}

	var req struct {
		Limit int `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Limit = 100
	}
	if req.Limit <= 0 {
		req.Limit = 100
	}
	if req.Limit > 1000 {
		req.Limit = 1000
	}

	query := models.MemoryQuery{
		Limit: req.Limit,
	}

	results, err := h.storage.Memory().Search(r.Context(), query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search memories")
		return
	}

	backfilled := 0
	skipped := 0
	errors := 0

	for _, result := range results {
		if len(result.Memory.Embedding) > 0 {
			skipped++
			continue
		}

		embedding, err := h.embeddingService.Embed(r.Context(), result.Memory.Content)
		if err != nil {
			errors++
			h.logger.Warn("failed to embed memory", "memory_id", result.Memory.ID, "error", err)
			continue
		}

		result.Memory.Embedding = embedding
		if err := h.storage.Memory().Update(r.Context(), result.Memory); err != nil {
			errors++
			h.logger.Warn("failed to update memory embedding", "memory_id", result.Memory.ID, "error", err)
			continue
		}

		backfilled++
	}

	writeJSON(w, http.StatusOK, models.AnyMap{
		"backfilled": backfilled,
		"skipped":    skipped,
		"errors":     errors,
		"total":      len(results),
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, models.AnyMap{"error": message})
}
