package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

type SearchMemoriesTool struct {
	store            storage.Store
	embeddingService *llm.EmbeddingService
}

func NewSearchMemoriesTool(store storage.Store, embeddingService *llm.EmbeddingService) *SearchMemoriesTool {
	return &SearchMemoriesTool{
		store:            store,
		embeddingService: embeddingService,
	}
}

func (t *SearchMemoriesTool) Name() string {
	return "search_memories"
}

func (t *SearchMemoriesTool) Description() string {
	return "Search the memory store for relevant memories. Use this when you need to recall specific information, facts, or past interactions that might be relevant to the current conversation. Returns matching memories ranked by relevance."
}

func (t *SearchMemoriesTool) ParameterSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query to find relevant memories",
			},
			"type": map[string]any{
				"type":        "string",
				"description": "Memory type to filter by: 'episodic' or 'semantic'. If omitted, searches all types",
				"enum":        []string{"episodic", "semantic"},
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results to return (default 10, max 50)",
				"minimum":     1,
				"maximum":     50,
			},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Optional tags to filter by",
			},
		},
		"required": []string{"query"},
	}
}

func (t *SearchMemoriesTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	query := getStringArg(args, "query")
	if query == "" {
		return "", fmt.Errorf("query argument is required")
	}

	memQuery := models.MemoryQuery{
		SearchText: query,
		Limit:      10,
	}

	if memType, ok := args["type"].(string); ok && memType != "" {
		memQuery.Type = models.MemoryType(memType)
	}

	if limitVal, ok := args["limit"].(float64); ok && limitVal > 0 {
		limit := int(limitVal)
		if limit > 50 {
			limit = 50
		}
		memQuery.Limit = limit
	}

	if tagsVal, ok := args["tags"].([]any); ok {
		tags := make([]string, 0, len(tagsVal))
		for _, tag := range tagsVal {
			if s, ok := tag.(string); ok {
				tags = append(tags, s)
			}
		}
		memQuery.Tags = tags
	}

	if t.store == nil {
		return "", fmt.Errorf("storage not available")
	}

	results, err := t.store.Memory().Search(ctx, memQuery)
	if err != nil {
		return fmt.Sprintf("Error searching memories: %s", err.Error()), fmt.Errorf("memory search failed: %w", err)
	}

	if len(results) == 0 {
		return "No memories found matching the query.", nil
	}

	type memResult struct {
		Content   string   `json:"content"`
		Type      string   `json:"type"`
		Relevance float64  `json:"relevance"`
		Tags      []string `json:"tags,omitempty"`
	}

	output := make([]memResult, 0, len(results))
	for _, r := range results {
		output = append(output, memResult{
			Content:   r.Memory.Content,
			Type:      string(r.Memory.Type),
			Relevance: r.Relevance,
			Tags:      r.Memory.Tags,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal memory results: %w", err)
	}

	return string(data), nil
}

type ListBeliefsTool struct {
	store storage.Store
}

func NewListBeliefsTool(store storage.Store) *ListBeliefsTool {
	return &ListBeliefsTool{store: store}
}

func (t *ListBeliefsTool) Name() string {
	return "list_beliefs"
}

func (t *ListBeliefsTool) Description() string {
	return "List current beliefs from the mind store. Useful for understanding what the system believes to be true. Can filter by subject or confidence threshold."
}

func (t *ListBeliefsTool) ParameterSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subject": map[string]any{
				"type":        "string",
				"description": "Filter beliefs by subject (e.g., 'architecture', 'user_preferences')",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of beliefs to return",
				"minimum":     1,
				"maximum":     100,
				"default":     20,
			},
			"status": map[string]any{
				"type":        "string",
				"description": "Filter by belief status",
				"enum":        []string{"active", "challenged", "archived", "superseded"},
			},
		},
		"required": []string{},
	}
}

func (t *ListBeliefsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if t.store == nil {
		return "", fmt.Errorf("storage not available")
	}

	query := models.MindQuery{
		Limit: 20,
	}

	if subject, ok := args["subject"].(string); ok && subject != "" {
		query.Subjects = []string{subject}
	}

	if status, ok := args["status"].(string); ok && status != "" {
		query.Status = models.RecordStatus(status)
	}

	if limitVal, ok := args["limit"].(float64); ok && limitVal > 0 {
		query.Limit = int(limitVal)
	}

	beliefs, err := t.store.Mind().GetBeliefs(ctx, query)
	if err != nil {
		return fmt.Sprintf("Error retrieving beliefs: %s", err.Error()), fmt.Errorf("get beliefs failed: %w", err)
	}

	if len(beliefs) == 0 {
		return "No beliefs found.", nil
	}

	type beliefResult struct {
		Subject    string  `json:"subject"`
		Content    string  `json:"content"`
		Confidence float64 `json:"confidence"`
		Status     string  `json:"status"`
	}

	output := make([]beliefResult, 0, len(beliefs))
	for _, b := range beliefs {
		output = append(output, beliefResult{
			Subject:    b.Subject,
			Content:    b.Content,
			Confidence: float64(b.Confidence),
			Status:     string(b.Status),
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal beliefs: %w", err)
	}

	return string(data), nil
}

type ListOpinionsTool struct {
	store storage.Store
}

func NewListOpinionsTool(store storage.Store) *ListOpinionsTool {
	return &ListOpinionsTool{store: store}
}

func (t *ListOpinionsTool) Name() string {
	return "list_opinions"
}

func (t *ListOpinionsTool) Description() string {
	return "List current opinions from the mind store. Opinions are subjective judgments, preferences, or takes that are evidence-linked and contextual."
}

func (t *ListOpinionsTool) ParameterSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subject": map[string]any{
				"type":        "string",
				"description": "Filter opinions by subject",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of opinions to return",
				"minimum":     1,
				"maximum":     100,
				"default":     20,
			},
		},
		"required": []string{},
	}
}

func (t *ListOpinionsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if t.store == nil {
		return "", fmt.Errorf("storage not available")
	}

	query := models.MindQuery{
		Limit: 20,
	}

	if subject, ok := args["subject"].(string); ok && subject != "" {
		query.Subjects = []string{subject}
	}

	if limitVal, ok := args["limit"].(float64); ok && limitVal > 0 {
		query.Limit = int(limitVal)
	}

	opinions, err := t.store.Mind().GetOpinions(ctx, query)
	if err != nil {
		return fmt.Sprintf("Error retrieving opinions: %s", err.Error()), fmt.Errorf("get opinions failed: %w", err)
	}

	if len(opinions) == 0 {
		return "No opinions found.", nil
	}

	type opinionResult struct {
		Subject    string  `json:"subject"`
		Content    string  `json:"content"`
		Strength   float64 `json:"strength"`
		Confidence float64 `json:"confidence"`
		Status     string  `json:"status"`
	}

	output := make([]opinionResult, 0, len(opinions))
	for _, o := range opinions {
		output = append(output, opinionResult{
			Subject:    o.Subject,
			Content:    o.Content,
			Strength:   float64(o.Strength),
			Confidence: float64(o.Confidence),
			Status:     string(o.Status),
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal opinions: %w", err)
	}

	return string(data), nil
}

type ListPreferencesTool struct {
	store storage.Store
}

func NewListPreferencesTool(store storage.Store) *ListPreferencesTool {
	return &ListPreferencesTool{store: store}
}

func (t *ListPreferencesTool) Name() string {
	return "list_preferences"
}

func (t *ListPreferencesTool) Description() string {
	return "List current preferences from the mind store. Preferences are user or system preferences that inform behavior."
}

func (t *ListPreferencesTool) ParameterSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subject": map[string]any{
				"type":        "string",
				"description": "Filter preferences by subject",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of preferences to return",
				"minimum":     1,
				"maximum":     100,
				"default":     20,
			},
		},
		"required": []string{},
	}
}

func (t *ListPreferencesTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if t.store == nil {
		return "", fmt.Errorf("storage not available")
	}

	query := models.MindQuery{
		Limit: 20,
	}

	if subject, ok := args["subject"].(string); ok && subject != "" {
		query.Subjects = []string{subject}
	}

	if limitVal, ok := args["limit"].(float64); ok && limitVal > 0 {
		query.Limit = int(limitVal)
	}

	prefs, err := t.store.Mind().GetPreferences(ctx, query)
	if err != nil {
		return fmt.Sprintf("Error retrieving preferences: %s", err.Error()), fmt.Errorf("get preferences failed: %w", err)
	}

	if len(prefs) == 0 {
		return "No preferences found.", nil
	}

	type prefResult struct {
		Subject  string  `json:"subject"`
		Content  string  `json:"content"`
		Strength float64 `json:"strength"`
		Status   string  `json:"status"`
	}

	output := make([]prefResult, 0, len(prefs))
	for _, p := range prefs {
		output = append(output, prefResult{
			Subject:  p.Subject,
			Content:  p.Content,
			Strength: float64(p.Strength),
			Status:   string(p.Status),
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal preferences: %w", err)
	}

	return string(data), nil
}

type ListGoalsTool struct {
	store storage.Store
}

func NewListGoalsTool(store storage.Store) *ListGoalsTool {
	return &ListGoalsTool{store: store}
}

func (t *ListGoalsTool) Name() string {
	return "list_goals"
}

func (t *ListGoalsTool) Description() string {
	return "List current goals from the mind store. Goals represent objectives that are being tracked."
}

func (t *ListGoalsTool) ParameterSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of goals to return",
				"minimum":     1,
				"maximum":     100,
				"default":     20,
			},
			"status": map[string]any{
				"type":        "string",
				"description": "Filter by goal status",
				"enum":        []string{"active", "paused", "completed", "abandoned", "superseded"},
			},
		},
		"required": []string{},
	}
}

func (t *ListGoalsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if t.store == nil {
		return "", fmt.Errorf("storage not available")
	}

	query := models.MindQuery{
		Limit: 20,
	}

	if status, ok := args["status"].(string); ok && status != "" {
		query.Status = models.RecordStatus(status)
	}

	if limitVal, ok := args["limit"].(float64); ok && limitVal > 0 {
		query.Limit = int(limitVal)
	}

	goals, err := t.store.Mind().GetGoals(ctx, query)
	if err != nil {
		return fmt.Sprintf("Error retrieving goals: %s", err.Error()), fmt.Errorf("get goals failed: %w", err)
	}

	if len(goals) == 0 {
		return "No goals found.", nil
	}

	type goalResult struct {
		Goal     string  `json:"goal"`
		Priority int     `json:"priority"`
		Status   string  `json:"status"`
		Progress float64 `json:"progress"`
	}

	output := make([]goalResult, 0, len(goals))
	for _, g := range goals {
		output = append(output, goalResult{
			Goal:     g.Goal,
			Priority: g.Priority,
			Status:   string(g.Status),
			Progress: g.Progress,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal goals: %w", err)
	}

	return string(data), nil
}

type GetCurrentTimeTool struct{}

func NewGetCurrentTimeTool() *GetCurrentTimeTool {
	return &GetCurrentTimeTool{}
}

func (t *GetCurrentTimeTool) Name() string {
	return "get_current_time"
}

func (t *GetCurrentTimeTool) Description() string {
	return "Get the current date and time. Use this when the conversation requires knowing the current time or date."
}

func (t *GetCurrentTimeTool) ParameterSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
		"required":   []string{},
	}
}

func (t *GetCurrentTimeTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	now := time.Now().UTC()
	return fmt.Sprintf("Current time: %s (UTC)", now.Format(time.RFC3339)), nil
}

type SearchConversationsTool struct {
	store storage.Store
}

func NewSearchConversationsTool(store storage.Store) *SearchConversationsTool {
	return &SearchConversationsTool{store: store}
}

func (t *SearchConversationsTool) Name() string {
	return "search_conversations"
}

func (t *SearchConversationsTool) Description() string {
	return "Search conversation history for a specific conversation. Use this to find past messages or discussions with the user. Requires a conversation_id."
}

func (t *SearchConversationsTool) ParameterSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"conversation_id": map[string]any{
				"type":        "string",
				"description": "The conversation ID to search within",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of messages to return",
				"minimum":     1,
				"maximum":     100,
				"default":     20,
			},
		},
		"required": []string{"conversation_id"},
	}
}

func (t *SearchConversationsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if t.store == nil {
		return "", fmt.Errorf("storage not available")
	}

	convID, ok := args["conversation_id"].(string)
	if !ok || convID == "" {
		return "", fmt.Errorf("conversation_id argument is required")
	}

	limit := 20
	if limitVal, ok := args["limit"].(float64); ok && limitVal > 0 {
		limit = int(limitVal)
		if limit > 100 {
			limit = 100
		}
	}

	messages, err := t.store.Conversation().GetMessages(ctx, convID, limit)
	if err != nil {
		return fmt.Sprintf("Error retrieving conversation: %s", err.Error()), fmt.Errorf("get messages failed: %w", err)
	}

	if len(messages) == 0 {
		return "No messages found for this conversation.", nil
	}

	type msgResult struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	output := make([]msgResult, 0, len(messages))
	for _, m := range messages {
		output = append(output, msgResult{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal conversation: %w", err)
	}

	return string(data), nil
}

func getStringArg(args map[string]any, key string) string {
	if val, ok := args[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func BuildToolListDescription(specs []models.ToolSpec) string {
	if len(specs) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\nAvailable tools (use ONLY when needed):\n")
	for _, spec := range specs {
		sb.WriteString(fmt.Sprintf("  - %s: %s\n", spec.Name, spec.Description))
	}
	sb.WriteString("\nIf you decide a tool is needed, output a JSON object with the tool call.\n")
	sb.WriteString("If no tool is needed, simply respond normally.\n")
	return sb.String()
}
