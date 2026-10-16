package modelmgr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type APIHandler struct {
	manager *ModelManager
}

func NewAPIHandler(manager *ModelManager) *APIHandler {
	return &APIHandler{manager: manager}
}

func (h *APIHandler) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/models", h.handleListModels)
	mux.HandleFunc(prefix+"/models/get", h.handleGetModel)
	mux.HandleFunc(prefix+"/models/active", h.handleGetActive)
	mux.HandleFunc(prefix+"/models/activate", h.handleActivate)
	mux.HandleFunc(prefix+"/models/test", h.handleTestModel)
	mux.HandleFunc(prefix+"/models/remove", h.handleRemoveModel)
	mux.HandleFunc(prefix+"/models/repair", h.handleRepairModel)
	mux.HandleFunc(prefix+"/models/update", h.handleUpdateModel)
	mux.HandleFunc(prefix+"/models/recover", h.handleRecover)
	mux.HandleFunc(prefix+"/models/request", h.handleRequestModel)
	mux.HandleFunc(prefix+"/models/bootstrap", h.handleBootstrap)
	mux.HandleFunc(prefix+"/hardware", h.handleHardware)
}

func (h *APIHandler) handleListModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	models, err := h.manager.ListModels()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"models": models,
		"count":  len(models),
	})
}

func (h *APIHandler) handleGetModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	modelID := r.URL.Query().Get("id")
	if modelID == "" {
		modelID = r.URL.Query().Get("name")
	}
	if modelID == "" {
		http.Error(w, "model id or name required", http.StatusBadRequest)
		return
	}

	model, err := h.manager.GetModel(modelID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSONResponse(w, http.StatusOK, model)
}

func (h *APIHandler) handleGetActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tier := r.URL.Query().Get("tier")
	tierValue, _ := ParseCapabilityTier(tier)

	model, err := h.manager.GetActiveModel(tierValue)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSONResponse(w, http.StatusOK, model)
}

func (h *APIHandler) handleActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Tier    string `json:"tier"`
		ModelID string `json:"model_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	tier, err := ParseCapabilityTier(req.Tier)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.manager.Activate(r.Context(), tier, req.ModelID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "activated", "model_id": req.ModelID, "tier": req.Tier})
}

func (h *APIHandler) handleTestModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	modelID := r.URL.Query().Get("id")
	if modelID == "" {
		http.Error(w, "model id required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	health, err := h.manager.TestModel(ctx, modelID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, http.StatusOK, health)
}

func (h *APIHandler) handleRemoveModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	modelID := r.URL.Query().Get("id")
	if modelID == "" {
		http.Error(w, "model id required", http.StatusBadRequest)
		return
	}

	if err := h.manager.RemoveModel(r.Context(), modelID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "removed", "model_id": modelID})
}

func (h *APIHandler) handleRepairModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	modelID := r.URL.Query().Get("id")
	if modelID == "" {
		http.Error(w, "model id required", http.StatusBadRequest)
		return
	}

	if err := h.manager.RepairModel(r.Context(), modelID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "repaired", "model_id": modelID})
}

func (h *APIHandler) handleUpdateModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	modelID := r.URL.Query().Get("id")
	if modelID == "" {
		http.Error(w, "model id required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()

	if err := h.manager.UpdateModel(ctx, modelID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "updated", "model_id": modelID})
}

func (h *APIHandler) handleRecover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()

	report, err := h.manager.RunRecovery(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, http.StatusOK, report)
}

func (h *APIHandler) handleRequestModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()

	result, err := h.manager.RequestModel(ctx, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, http.StatusOK, result)
}

func (h *APIHandler) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()

	cfg := DefaultBootstrapConfig()
	results, err := h.manager.Bootstrap(ctx, cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, http.StatusOK, results)
}

func (h *APIHandler) handleHardware(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spec, err := h.manager.DetectHardware(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, http.StatusOK, spec)
}

func writeJSONResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

var _ = fmt.Sprintf
