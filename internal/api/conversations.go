package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

type ConversationController struct {
	store  *storage.MongoStore
	logger *slog.Logger
}

func NewConversationController(store *storage.MongoStore, logger *slog.Logger) *ConversationController {
	if logger == nil {
		logger = slog.Default()
	}
	return &ConversationController{
		store:  store,
		logger: logger,
	}
}

func (cc *ConversationController) Register(mux *http.ServeMux) {
	cc.RegisterAsProtected(mux)
}

func (cc *ConversationController) RegisterAsProtected(mux *http.ServeMux) {
	mux.HandleFunc("/api/conversations", cc.handleConversations)
	mux.HandleFunc("/api/conversations/", cc.handleConversationByID)
}

func (cc *ConversationController) handleConversations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cc.listConversations(w, r)
	case http.MethodPost:
		cc.createConversation(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (cc *ConversationController) handleConversationByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	parts := strings.SplitN(path, "/", 2)

	if len(parts) < 1 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "conversation ID required")
		return
	}

	convID := parts[0]

	switch r.Method {
	case http.MethodGet:
		cc.getConversation(w, r, convID)
	case http.MethodDelete:
		cc.deleteConversation(w, r, convID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (cc *ConversationController) listConversations(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	conversations, err := cc.store.Conversation().ListConversations(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list conversations")
		return
	}

	writeJSON(w, http.StatusOK, conversations)
}

func (cc *ConversationController) createConversation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	convID := models.NewID()
	now := time.Now().UTC()

	conversation := models.ConversationMessage{
		ID:             convID,
		ConversationID: convID,
		Title:          req.Title,
		Role:           "system",
		Content:        "Conversation created.",
		Timestamp:      now,
	}

	if err := cc.store.Conversation().CreateConversation(r.Context(), conversation); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create conversation")
		return
	}

	writeJSON(w, http.StatusCreated, models.AnyMap{
		"conversation_id": convID,
		"title":           req.Title,
	})
}

func (cc *ConversationController) getConversation(w http.ResponseWriter, r *http.Request, convID string) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	messages, err := cc.store.Conversation().GetMessagesPaginated(r.Context(), convID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get conversation")
		return
	}

	writeJSON(w, http.StatusOK, models.AnyMap{
		"conversation_id": convID,
		"messages":        messages,
		"count":           len(messages),
	})
}

func (cc *ConversationController) deleteConversation(w http.ResponseWriter, r *http.Request, convID string) {
	memories, err := cc.store.Memory().GetByConversation(r.Context(), convID, 1000)
	if err == nil {
		for _, mem := range memories {
			cc.store.Memory().Delete(r.Context(), mem.ID)
		}
	}

	result, err := cc.store.Conversation().DeleteAll(r.Context(), convID)
	if err != nil {
		cc.logger.Warn("failed to delete conversation messages", "error", err)
	}

	writeJSON(w, http.StatusOK, models.AnyMap{
		"conversation_id":  convID,
		"deleted":          true,
		"messages_deleted": result,
	})
}
