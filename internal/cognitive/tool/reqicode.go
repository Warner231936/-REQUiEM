package tool

import (
	"context"
	"fmt"
	"time"

	"github.com/requiem/cognitive/internal/cognitive/integration"
	"github.com/requiem/cognitive/pkg/models"
)

const reqicodeDefaultTimeout = 120

type ReqiCodeTool struct {
	client *integration.ReQiCodeClient
}

func NewReqiCodeTool(baseURL string) *ReqiCodeTool {
	return &ReqiCodeTool{
		client: integration.NewReQiCodeClient(baseURL, reqicodeDefaultTimeout),
	}
}

func (t *ReqiCodeTool) Name() string {
	return "reqicode"
}

func (t *ReqiCodeTool) Description() string {
	return `Submit structured engineering tasks to the ReqiCode daemon for autonomous implementation, verification, and evidence return. The daemon creates a project workspace, runs a spiral engineering cycle (requirements, architecture, code generation, build, test), and returns structured results including files changed, test outcomes, build status, and chain-verified integrity metadata. Use this when a conversation requires implementing, modifying, or verifying code in a separate project workspace, or when structured engineering evidence is needed from the ReqiCode SpiraCodeMaker engine. The tool returns a run_id on submission; poll GET /api/runs/<id> to check completion and retrieve results.`
}

func (t *ReqiCodeTool) ParameterSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Operation to perform: 'submit' to start a new engineering task, 'get_result' to retrieve results by run_id, 'cancel' to cancel a running task",
				"enum":        []string{"submit", "get_result", "cancel"},
			},
			"task_id": map[string]any{
				"type":        "string",
				"description": "Unique identifier for the engineering task (required for submit)",
			},
			"objective": map[string]any{
				"type":        "string",
				"description": "What the engineering task should accomplish (required for submit)",
			},
			"rationale": map[string]any{
				"type":        "string",
				"description": "Why this task is being undertaken, context for the daemon",
			},
			"repository": map[string]any{
				"type":        "string",
				"description": "Repository identifier or URL this task targets",
			},
			"workspace_path": map[string]any{
				"type":        "string",
				"description": "Filesystem path for the project workspace",
			},
			"relevant_files": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Files relevant to the task",
			},
			"contextual_evidence": map[string]any{
				"type":        "string",
				"description": "Prior code or error context to inform the task",
			},
			"acceptance_criteria": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "List of criteria the result must satisfy",
			},
			"test_requirements": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"unit_tests":     map[string]any{"type": "boolean"},
					"integration":    map[string]any{"type": "boolean"},
					"coverage":       map[string]any{"type": "boolean"},
					"benchmark":      map[string]any{"type": "boolean"},
					"race_detection": map[string]any{"type": "boolean"},
				},
			},
			"budget": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"max_tokens":    map[string]any{"type": "integer"},
					"time_limit_sec": map[string]any{"type": "integer"},
					"max_retries":   map[string]any{"type": "integer"},
				},
			},
			"permissions": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"allow_fs_write":     map[string]any{"type": "boolean"},
					"allow_network":      map[string]any{"type": "boolean"},
					"allowed_paths":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"restricted_commands": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
			},
			"run_id": map[string]any{
				"type":        "string",
				"description": "Run ID from a previous submit call (required for get_result or cancel)",
			},
		},
		"required": []string{"action"},
	}
}

func (t *ReqiCodeTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	action := getStringArg(args, "action")
	if action == "" {
		return "", fmt.Errorf("action argument is required")
	}

	switch action {
	case "submit":
		return t.submitTask(ctx, args)
	case "get_result":
		return t.getResult(ctx, args)
	case "cancel":
		return t.cancelTask(ctx, args)
	default:
		return "", fmt.Errorf("unknown action: %s (expected 'submit', 'get_result', or 'cancel')", action)
	}
}

func (t *ReqiCodeTool) submitTask(ctx context.Context, args map[string]any) (string, error) {
	task := models.TaskRequest{
		TaskID:             getStringArg(args, "task_id"),
		Objective:          getStringArg(args, "objective"),
		Rationale:          getStringArg(args, "rationale"),
		Repository:         getStringArg(args, "repository"),
		WorkspacePath:      getStringArg(args, "workspace_path"),
		ContextualEvidence: getStringArg(args, "contextual_evidence"),
		AcceptanceCriteria: getStringArgSlice(args, "acceptance_criteria"),
		RelevantFiles:      getStringArgSlice(args, "relevant_files"),
	}

	if task.Objective == "" {
		return "", fmt.Errorf("objective is required for submit action")
	}
	if task.TaskID == "" {
		task.TaskID = fmt.Sprintf("task-%d", time.Now().UnixNano())
	}

	if v, ok := args["test_requirements"].(map[string]any); ok && len(v) > 0 {
		task.TestRequirements = parseTestRequirements(v)
	}
	if v, ok := args["budget"].(map[string]any); ok && len(v) > 0 {
		task.Budget = parseTaskBudget(v)
	}
	if v, ok := args["permissions"].(map[string]any); ok && len(v) > 0 {
		task.Permissions = parseTaskPermissions(v)
	}

	runID, err := t.client.SubmitTask(ctx, task)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`Engineering task submitted successfully.
- Task ID: %s
- Run ID: %s

To check results, call reqicode with action='get_result' and run_id='%s'.`,
		task.TaskID, runID, runID), nil
}

func (t *ReqiCodeTool) getResult(ctx context.Context, args map[string]any) (string, error) {
	runID := getStringArg(args, "run_id")
	if runID == "" {
		return "", fmt.Errorf("run_id is required for get_result action")
	}

	taskResult, err := t.client.GetTaskResult(ctx, runID)
	if err != nil {
		return "", err
	}

	result := fmt.Sprintf(`ReqiCode run result:
- Run ID: %s
- Task ID: %s
- Status: %s
`, taskResult.RunID, taskResult.TaskID, taskResult.Status)

	if taskResult.FilesChanged != nil {
		result += fmt.Sprintf("- Files Changed: %d\n", len(taskResult.FilesChanged))
	}
	if len(taskResult.TestResults) > 0 {
		result += fmt.Sprintf("- Test Results: %d\n", len(taskResult.TestResults))
	}
	if taskResult.VerificationStatus != "" {
		result += fmt.Sprintf("- Verification: %s\n", taskResult.VerificationStatus)
	}
	if taskResult.DurationMs > 0 {
		result += fmt.Sprintf("- Duration: %dms\n", taskResult.DurationMs)
	}
	if taskResult.ErrorSummary != "" {
		result += fmt.Sprintf("- Error Summary: %s\n", taskResult.ErrorSummary)
	}
	if taskResult.KnownLimitations != "" {
		result += fmt.Sprintf("- Known Limitations: %s\n", taskResult.KnownLimitations)
	}

	if len(taskResult.FilesChanged) > 0 {
		result += "\nFiles:\n"
		for _, f := range taskResult.FilesChanged {
			result += fmt.Sprintf("  - %s (%s)\n", f.Path, f.Operation)
		}
	}
	if len(taskResult.TestResults) > 0 {
		result += "\nTests:\n"
		for _, tr := range taskResult.TestResults {
			result += fmt.Sprintf("  - %s: %s\n", tr.Command, tr.Status)
		}
	}

	validity := t.client.AssessEvidence(taskResult)
	result += fmt.Sprintf("\nEvidence Assessment: %s\n", validity)

	return result, nil
}

func (t *ReqiCodeTool) cancelTask(ctx context.Context, args map[string]any) (string, error) {
	runID := getStringArg(args, "run_id")
	if runID == "" {
		return "", fmt.Errorf("run_id is required for cancel action")
	}

	if err := t.client.CancelTask(ctx, runID); err != nil {
		return "", err
	}

	return fmt.Sprintf("Run %s cancellation requested.", runID), nil
}

func getStringArgSlice(args map[string]any, key string) []string {
	if v, ok := args[key].([]string); ok {
		return v
	}
	if v, ok := args[key].([]any); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func parseTestRequirements(v map[string]any) *models.TaskTestRequirements {
	tr := &models.TaskTestRequirements{}
	if val, ok := v["unit_tests"].(bool); ok {
		tr.UnitTests = val
	}
	if val, ok := v["integration"].(bool); ok {
		tr.Integration = val
	}
	if val, ok := v["coverage"].(bool); ok {
		tr.Coverage = val
	}
	if val, ok := v["benchmark"].(bool); ok {
		tr.Benchmark = val
	}
	if val, ok := v["race_detection"].(bool); ok {
		tr.RaceDetection = val
	}
	return tr
}

func parseTaskBudget(v map[string]any) *models.TaskBudget {
	b := &models.TaskBudget{}
	if val, ok := v["max_tokens"].(float64); ok {
		b.MaxTokens = int(val)
	}
	if val, ok := v["time_limit_sec"].(float64); ok {
		b.TimeLimitSec = int(val)
	}
	if val, ok := v["max_retries"].(float64); ok {
		b.MaxRetries = int(val)
	}
	return b
}

func parseTaskPermissions(v map[string]any) *models.TaskPermissions {
	p := &models.TaskPermissions{}
	if val, ok := v["allow_fs_write"].(bool); ok {
		p.AllowFileSystemWrite = val
	}
	if val, ok := v["allow_network"].(bool); ok {
		p.AllowNetwork = val
	}
	if val, ok := v["allowed_paths"].([]any); ok {
		p.AllowedPaths = make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				p.AllowedPaths = append(p.AllowedPaths, s)
			}
		}
	}
	if val, ok := v["restricted_commands"].([]any); ok {
		p.RestrictedCommands = make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				p.RestrictedCommands = append(p.RestrictedCommands, s)
			}
		}
	}
	return p
}
