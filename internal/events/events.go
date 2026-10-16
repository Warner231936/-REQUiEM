package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

type EventType string

const (
	EventPipelineStarted        EventType = "pipeline.started"
	EventPipelineCompleted      EventType = "pipeline.completed"
	EventStageStarted           EventType = "stage.started"
	EventStageCompleted         EventType = "stage.completed"
	EventStageFailed            EventType = "stage.failed"
	EventMemoryCreated          EventType = "memory.created"
	EventMemoryUpdated          EventType = "memory.updated"
	EventBeliefProposed         EventType = "belief.proposed"
	EventBeliefChanged          EventType = "belief.changed"
	EventOpinionProposed        EventType = "opinion.proposed"
	EventOpinionCommitted       EventType = "opinion.committed"
	EventContradictionDetected  EventType = "contradiction.detected"
	EventConsolidationCompleted EventType = "consolidation.completed"
	EventDriftDetected          EventType = "drift.detected"

	EventBeliefCreated           EventType = "belief.created"
	EventBeliefUpdated           EventType = "belief.updated"
	EventBeliefConfidenceChanged EventType = "belief.confidence_changed"
	EventBeliefRejected          EventType = "belief.rejected"
	EventBeliefPromoted          EventType = "belief.promoted"
	EventEvidenceAdded           EventType = "evidence.added"
	EventHypothesisCreated       EventType = "hypothesis.created"
	EventHypothesisCompetition   EventType = "hypothesis.competition_started"
	EventHypothesisWon           EventType = "hypothesis.won"
	EventHypothesisLost          EventType = "hypothesis.lost"
	EventMindSnapshot            EventType = "mind.snapshot"
)

type Event struct {
	Type           EventType     `json:"type"`
	Stage          int           `json:"stage,omitempty"`
	ConversationID string        `json:"conversation_id,omitempty"`
	TurnID         string        `json:"turn_id,omitempty"`
	DurationMs     int64         `json:"duration_ms,omitempty"`
	Model          string        `json:"model,omitempty"`
	Timestamp      time.Time     `json:"timestamp"`
	Details        models.AnyMap `json:"details,omitempty"`
}

type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string]*Subscriber
	logger      *slog.Logger
}

type Subscriber struct {
	ID     string
	Events chan Event
	Filter EventType
}

func NewEventBus(logger *slog.Logger) *EventBus {
	if logger == nil {
		logger = slog.Default()
	}
	return &EventBus{
		subscribers: make(map[string]*Subscriber),
		logger:      logger,
	}
}

func (eb *EventBus) Subscribe(id string, filter EventType) *Subscriber {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	sub := &Subscriber{
		ID:     id,
		Events: make(chan Event, 100),
		Filter: filter,
	}
	eb.subscribers[id] = sub
	return sub
}

func (eb *EventBus) Unsubscribe(id string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if sub, ok := eb.subscribers[id]; ok {
		close(sub.Events)
		delete(eb.subscribers, id)
	}
}

func (eb *EventBus) Publish(event Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	event.Timestamp = time.Now().UTC()

	for _, sub := range eb.subscribers {
		if sub.Filter == "" || sub.Filter == event.Type {
			select {
			case sub.Events <- event:
			default:
				eb.logger.Warn("subscriber event channel full, dropping event", "subscriber", sub.ID)
			}
		}
	}
}

func (eb *EventBus) PublishStageEvent(stageNum int, convID, turnID, model string, durationMs int64, success bool) {
	eventType := EventStageCompleted
	if !success {
		eventType = EventStageFailed
	}

	eb.Publish(Event{
		Type:           eventType,
		Stage:          stageNum,
		ConversationID: convID,
		TurnID:         turnID,
		DurationMs:     durationMs,
		Model:          model,
	})
}

func (eb *EventBus) PublishPipelineEvent(convID, turnID string, durationMs int64, success bool) {
	eventType := EventPipelineCompleted
	if !success {
		eventType = EventStageFailed
	}

	eb.Publish(Event{
		Type:           eventType,
		ConversationID: convID,
		TurnID:         turnID,
		DurationMs:     durationMs,
	})
}

func (eb *EventBus) PublishMemoryEvent(eventType EventType, convID, turnID, memoryID string, details models.AnyMap) {
	eb.Publish(Event{
		Type:           eventType,
		ConversationID: convID,
		TurnID:         turnID,
		Details:        details,
	})
}

func (eb *EventBus) PublishBeliefEvent(eventType EventType, convID, turnID, beliefID string, details models.AnyMap) {
	eb.Publish(Event{
		Type:           eventType,
		ConversationID: convID,
		TurnID:         turnID,
		Details:        details,
	})
}

func (eb *EventBus) PublishBeliefEventWithData(eventType EventType, convID, turnID, beliefID string, details models.AnyMap) {
	eb.Publish(Event{
		Type:           eventType,
		ConversationID: convID,
		TurnID:         turnID,
		Details:        details,
	})
}

func (eb *EventBus) PublishContradictionEvent(convID, turnID, contradictionID string, details models.AnyMap) {
	eb.Publish(Event{
		Type:           EventContradictionDetected,
		ConversationID: convID,
		TurnID:         turnID,
		Details:        details,
	})
}

func (eb *EventBus) PublishDriftEvent(convID, turnID string, details models.AnyMap) {
	eb.Publish(Event{
		Type:           EventDriftDetected,
		ConversationID: convID,
		TurnID:         turnID,
		Details:        details,
	})
}

func (eb *EventBus) PublishHypothesisEvent(eventType EventType, hypothesisID string, details models.AnyMap) {
	eb.Publish(Event{
		Type:    eventType,
		Details: details,
	})
}

type SSEHandler struct {
	eventBus  *EventBus
	logger    *slog.Logger
	origins   []string
	mindStore storage.MindStore
}

func NewSSEHandler(eventBus *EventBus, logger *slog.Logger, allowedOrigins []string) *SSEHandler {
	if logger == nil {
		logger = slog.Default()
	}
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"http://localhost:3000", "http://127.0.0.1:3000"}
	}
	return &SSEHandler{
		eventBus: eventBus,
		logger:   logger,
		origins:  allowedOrigins,
	}
}

func (h *SSEHandler) WithMindStore(store storage.MindStore) *SSEHandler {
	h.mindStore = store
	return h
}

func (h *SSEHandler) AllowedOrigins() []string {
	return h.origins
}

func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	allowedOrigins := h.AllowedOrigins()
	origin := r.Header.Get("Origin")
	if origin != "" {
		for _, allowed := range allowedOrigins {
			if allowed == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				break
			}
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	subID := r.URL.Query().Get("id")
	if subID == "" {
		subID = "sse-" + time.Now().Format("20060102150405")
	}

	filter := EventType(r.URL.Query().Get("filter"))

	sub := h.eventBus.Subscribe(subID, filter)
	defer h.eventBus.Unsubscribe(subID)

	ctx := r.Context()

	h.logger.Info("SSE client connected", "subscriber", subID, "filter", filter)

	if h.mindStore != nil && filter == "" {
		h.sendInitialSnapshot(w, flusher, subID)
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-sub.Events:
			if !ok {
				return
			}

			payload := models.AnyMap{"type": string(event.Type)}
			if event.Details != nil {
				for k, v := range event.Details {
					if k != "type" {
						payload[k] = v
					}
				}
			}

			data, err := json.Marshal(payload)
			if err != nil {
				continue
			}

			fmt.Fprintf(w, "event: %s\n", event.Type)
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()

		case <-ticker.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()

		case <-ctx.Done():
			h.logger.Info("SSE client disconnected", "subscriber", subID)
			return
		}
	}
}

func (h *SSEHandler) sendInitialSnapshot(w http.ResponseWriter, flusher http.Flusher, subID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	nodes := []models.AnyMap{}
	beliefCount := 0
	hypothesisCount := 0

	beliefs, err := h.mindStore.GetBeliefs(ctx, models.MindQuery{Limit: 1000})
	if err != nil {
		h.logger.Warn("failed to fetch beliefs for SSE snapshot", "error", err, "subscriber", subID)
	} else {
		beliefCount = len(beliefs)
		for _, b := range beliefs {
			nodes = append(nodes, models.AnyMap{
				"id":          b.ID,
				"statement":   b.Content,
				"title":       b.Content,
				"confidence":  float64(b.Confidence),
				"importance":  float64(b.Importance),
				"category":    "belief",
				"status":      string(b.Status),
				"evidence":    b.Evidence,
				"created_at":  b.CreatedAt,
				"updated_at":  b.UpdatedAt,
				"source_type": string(b.SourceType),
				"subject":     b.Subject,
			})
		}
	}

	hypotheses, err := h.mindStore.GetHypotheses(ctx, models.MindQuery{Limit: 1000})
	if err != nil {
		h.logger.Warn("failed to fetch hypotheses for SSE snapshot", "error", err, "subscriber", subID)
	} else {
		hypothesisCount = len(hypotheses)
		for _, hyp := range hypotheses {
			nodes = append(nodes, models.AnyMap{
				"id":         hyp.ID,
				"statement":  hyp.Hypothesis,
				"title":      hyp.Hypothesis,
				"confidence": float64(hyp.Confidence),
				"category":   "hypothesis",
				"status":     "hypothesis",
				"evidence":   hyp.SupportingEvidence,
				"created_at": hyp.CreatedAt,
				"updated_at": hyp.UpdatedAt,
				"subject":    hyp.Subject,
			})
		}
	}

	if len(nodes) == 0 {
		h.logger.Info("no beliefs or hypotheses in snapshot, sending empty snapshot",
			"subscriber", subID)
	}

	snapshotData := models.AnyMap{
		"type":  string(EventMindSnapshot),
		"nodes": nodes,
		"counts": models.AnyMap{
			"beliefs":    beliefCount,
			"hypotheses": hypothesisCount,
		},
	}

	data, err := json.Marshal(snapshotData)
	if err != nil {
		return
	}

	fmt.Fprintf(w, "event: %s\n", EventMindSnapshot)
	fmt.Fprintf(w, "data: %s\n\n", string(data))
	flusher.Flush()

	h.logger.Info("sent initial mind snapshot to SSE client",
		"subscriber", subID,
		"beliefs", beliefCount,
		"hypotheses", hypothesisCount,
	)
}
