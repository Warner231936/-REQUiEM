package tool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/requiem/cognitive/pkg/models"
)

func newTestReqiCodeTool(t *testing.T, handler http.HandlerFunc) *ReqiCodeTool {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewReqiCodeTool(server.URL)
}

func TestReqiCodeTool_Name(t *testing.T) {
	tool := NewReqiCodeTool("http://localhost:9999")
	if name := tool.Name(); name != "reqicode" {
		t.Errorf("expected name 'reqicode', got %s", name)
	}
}

func TestReqiCodeTool_Description(t *testing.T) {
	tool := NewReqiCodeTool("http://localhost:9999")
	desc := tool.Description()
	if desc == "" {
		t.Error("description should not be empty")
	}
	if !strings.Contains(desc, "ReqiCode") {
		t.Error("description should mention ReqiCode")
	}
}

func TestReqiCodeTool_ParameterSchema(t *testing.T) {
	tool := NewReqiCodeTool("http://localhost:9999")
	schema := tool.ParameterSchema()
	if schema["type"] != "object" {
		t.Error("expected schema type 'object'")
	}
	if _, ok := schema["properties"]; !ok {
		t.Error("expected 'properties' in schema")
	}
	req, ok := schema["required"].([]string)
	if !ok || len(req) != 1 || req[0] != "action" {
		t.Errorf("expected required ['action'], got %v", schema["required"])
	}
}

func TestReqiCodeTool_Execute_InvalidAction(t *testing.T) {
	tool := NewReqiCodeTool("http://localhost:9999")

	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "invalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
	if !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("expected unknown action error, got: %v", err)
	}
}

func TestReqiCodeTool_Execute_MissingAction(t *testing.T) {
	tool := NewReqiCodeTool("http://localhost:9999")

	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing action")
	}
	if !strings.Contains(err.Error(), "action argument is required") {
		t.Errorf("expected missing action error, got: %v", err)
	}
}

func TestReqiCodeTool_SubmitTask_Success(t *testing.T) {
	submitted := false
	tool := newTestReqiCodeTool(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/engineering" {
			t.Errorf("expected path /api/engineering, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var req models.TaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.Objective != "Fix the bug" {
			t.Errorf("expected objective 'Fix the bug', got %s", req.Objective)
		}

		submitted = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "accepted",
			"run_id":  "run-abc123",
			"task_id": req.TaskID,
		})
	})

	result, err := tool.Execute(context.Background(), map[string]any{
		"action":          "submit",
		"objective":       "Fix the bug",
		"task_id":         "task-001",
		"contextual_evidence": "nil pointer dereference in handler.go",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !submitted {
		t.Error("expected submit request to be made")
	}

	if !strings.Contains(result, "run-abc123") {
		t.Errorf("expected run ID in result, got: %s", result)
	}
	if !strings.Contains(result, "task-001") {
		t.Errorf("expected task ID in result, got: %s", result)
	}
}

func TestReqiCodeTool_SubmitTask_AutoGenerateTaskID(t *testing.T) {
	tool := newTestReqiCodeTool(t, func(w http.ResponseWriter, r *http.Request) {
		var req models.TaskRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.TaskID == "" {
			t.Error("TaskID should be auto-generated if empty")
		}
		if !strings.HasPrefix(req.TaskID, "task-") {
			t.Errorf("expected task ID prefix 'task-', got %s", req.TaskID)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "accepted",
			"run_id":  "run-test",
			"task_id": req.TaskID,
		})
	})

	_, err := tool.Execute(context.Background(), map[string]any{
		"action":    "submit",
		"objective": "Test auto task ID",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

func TestReqiCodeTool_SubmitTask_MissingObjective(t *testing.T) {
	tool := NewReqiCodeTool("http://localhost:9999")

	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "submit",
	})
	if err == nil {
		t.Fatal("expected error for missing objective")
	}
	if !strings.Contains(err.Error(), "objective is required") {
		t.Errorf("expected objective required error, got: %v", err)
	}
}

func TestReqiCodeTool_SubmitTask_ServerError(t *testing.T) {
	tool := newTestReqiCodeTool(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "internal server error",
		})
	})

	_, err := tool.Execute(context.Background(), map[string]any{
		"action":    "submit",
		"objective": "Test server error",
	})
	if err == nil {
		t.Fatal("expected error for server error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 status in error, got: %v", err)
	}
}

func TestReqiCodeTool_GetResult_Success(t *testing.T) {
	tool := newTestReqiCodeTool(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/runs/run-456" {
			t.Errorf("expected path /api/runs/run-456, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":       "run-456",
			"task_id":  "task-001",
			"status":   "COMPLETED",
			"result": map[string]any{
				"status":              "passed",
				"verification_status": "passed",
				"duration_ms":         1250,
				"files_changed": []map[string]any{
					{"path": "main.go", "operation": "modified"},
				},
				"test_results": []map[string]any{
					{"command": "go test ./...", "status": "passed"},
				},
			},
		})
	})

	result, err := tool.Execute(context.Background(), map[string]any{
		"action": "get_result",
		"run_id": "run-456",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "run-456") {
		t.Errorf("expected run ID in result, got: %s", result)
	}
	if !strings.Contains(result, "COMPLETED") {
		t.Errorf("expected status COMPLETED in result, got: %s", result)
	}
	if !strings.Contains(result, "main.go") {
		t.Errorf("expected file info in result, got: %s", result)
	}
	if !strings.Contains(result, "passed") {
		t.Errorf("expected test status in result, got: %s", result)
	}
}

func TestReqiCodeTool_GetResult_MissingRunID(t *testing.T) {
	tool := NewReqiCodeTool("http://localhost:9999")

	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "get_result",
	})
	if err == nil {
		t.Fatal("expected error for missing run_id")
	}
	if !strings.Contains(err.Error(), "run_id is required") {
		t.Errorf("expected run_id required error, got: %v", err)
	}
}

func TestReqiCodeTool_GetResult_NotFound(t *testing.T) {
	tool := newTestReqiCodeTool(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "run not found",
		})
	})

	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "get_result",
		"run_id": "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for not found")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 status in error, got: %v", err)
	}
}

func TestReqiCodeTool_CancelTask_Success(t *testing.T) {
	tool := newTestReqiCodeTool(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/runs/run-789" {
			t.Errorf("expected path /api/runs/run-789, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
	})

	result, err := tool.Execute(context.Background(), map[string]any{
		"action": "cancel",
		"run_id": "run-789",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "run-789") {
		t.Errorf("expected run ID in result, got: %s", result)
	}
	if !strings.Contains(result, "cancellation requested") {
		t.Errorf("expected cancellation message, got: %s", result)
	}
}

func TestReqiCodeTool_CancelTask_MissingRunID(t *testing.T) {
	tool := NewReqiCodeTool("http://localhost:9999")

	_, err := tool.Execute(context.Background(), map[string]any{
		"action": "cancel",
	})
	if err == nil {
		t.Fatal("expected error for missing run_id")
	}
	if !strings.Contains(err.Error(), "run_id is required") {
		t.Errorf("expected run_id required error, got: %v", err)
	}
}

func TestReqiCodeTool_SubmitTask_WithTestRequirements(t *testing.T) {
	tool := newTestReqiCodeTool(t, func(w http.ResponseWriter, r *http.Request) {
		var req models.TaskRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.TestRequirements == nil {
			t.Error("expected test requirements to be parsed")
		}
		if !req.TestRequirements.UnitTests {
			t.Error("expected unit_tests to be true")
		}
		if !req.TestRequirements.Benchmark {
			t.Error("expected benchmark to be true")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "accepted",
			"run_id":  "run-test",
			"task_id": req.TaskID,
		})
	})

	_, err := tool.Execute(context.Background(), map[string]any{
		"action":    "submit",
		"objective": "Test with requirements",
		"test_requirements": map[string]any{
			"unit_tests":  true,
			"integration": true,
			"benchmark":   true,
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

func TestReqiCodeTool_SubmitTask_WithBudget(t *testing.T) {
	tool := newTestReqiCodeTool(t, func(w http.ResponseWriter, r *http.Request) {
		var req models.TaskRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Budget == nil {
			t.Error("expected budget to be parsed")
		}
		if req.Budget.MaxTokens != 4000 {
			t.Errorf("expected max_tokens 4000, got %d", req.Budget.MaxTokens)
		}
		if req.Budget.TimeLimitSec != 300 {
			t.Errorf("expected time_limit_sec 300, got %d", req.Budget.TimeLimitSec)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "accepted",
			"run_id":  "run-test",
			"task_id": req.TaskID,
		})
	})

	_, err := tool.Execute(context.Background(), map[string]any{
		"action":    "submit",
		"objective": "Test with budget",
		"budget": map[string]any{
			"max_tokens":      float64(4000),
			"time_limit_sec":  float64(300),
			"max_retries":     float64(3),
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

func TestReqiCodeTool_SubmitTask_WithPermissions(t *testing.T) {
	tool := newTestReqiCodeTool(t, func(w http.ResponseWriter, r *http.Request) {
		var req models.TaskRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Permissions == nil {
			t.Error("expected permissions to be parsed")
		}
		if !req.Permissions.AllowFileSystemWrite {
			t.Error("expected allow_fs_write to be true")
		}
		if len(req.Permissions.AllowedPaths) != 1 {
			t.Errorf("expected 1 allowed path, got %d", len(req.Permissions.AllowedPaths))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "accepted",
			"run_id":  "run-test",
			"task_id": req.TaskID,
		})
	})

	_, err := tool.Execute(context.Background(), map[string]any{
		"action":    "submit",
		"objective": "Test with permissions",
		"permissions": map[string]any{
			"allow_fs_write":     true,
			"allow_network":      false,
			"allowed_paths":      []any{"/tmp/workspace"},
			"restricted_commands": []any{"rm -rf /"},
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

func TestGetStringArgSlice_Strings(t *testing.T) {
	args := map[string]any{
		"items": []string{"a", "b", "c"},
	}
	result := getStringArgSlice(args, "items")
	if len(result) != 3 {
		t.Errorf("expected 3 items, got %d", len(result))
	}
}

func TestGetStringArgSlice_AnySlice(t *testing.T) {
	args := map[string]any{
		"items": []any{"x", "y", "z"},
	}
	result := getStringArgSlice(args, "items")
	if len(result) != 3 {
		t.Errorf("expected 3 items, got %d", len(result))
	}
	if result[0] != "x" {
		t.Errorf("expected first item 'x', got %s", result[0])
	}
}

func TestGetStringArgSlice_NonSlice(t *testing.T) {
	args := map[string]any{
		"items": "not a slice",
	}
	result := getStringArgSlice(args, "items")
	if len(result) != 0 {
		t.Errorf("expected empty slice for non-slice, got %v", result)
	}
}

func TestParseTestRequirements(t *testing.T) {
	v := map[string]any{
		"unit_tests":     true,
		"integration":    true,
		"coverage":       false,
		"benchmark":      true,
		"race_detection": false,
	}
	tr := parseTestRequirements(v)
	if tr == nil {
		t.Fatal("expected non-nil test requirements")
	}
	if !tr.UnitTests || !tr.Integration || tr.Coverage || !tr.Benchmark || tr.RaceDetection {
		t.Errorf("unexpected test requirements: %+v", tr)
	}
}

func TestParseTaskBudget(t *testing.T) {
	v := map[string]any{
		"max_tokens":    float64(8000),
		"time_limit_sec": float64(600),
		"max_retries":   float64(5),
	}
	b := parseTaskBudget(v)
	if b == nil {
		t.Fatal("expected non-nil budget")
	}
	if b.MaxTokens != 8000 || b.TimeLimitSec != 600 || b.MaxRetries != 5 {
		t.Errorf("unexpected budget: %+v", b)
	}
}

func TestParseTaskPermissions(t *testing.T) {
	v := map[string]any{
		"allow_fs_write":       true,
		"allow_network":        true,
		"allowed_paths":        []any{"/src", "/tests"},
		"restricted_commands":  []any{"sudo", "rm"},
	}
	p := parseTaskPermissions(v)
	if p == nil {
		t.Fatal("expected non-nil permissions")
	}
	if !p.AllowFileSystemWrite || !p.AllowNetwork {
		t.Error("expected both write and network to be allowed")
	}
	if len(p.AllowedPaths) != 2 {
		t.Errorf("expected 2 allowed paths, got %d", len(p.AllowedPaths))
	}
	if len(p.RestrictedCommands) != 2 {
		t.Errorf("expected 2 restricted commands, got %d", len(p.RestrictedCommands))
	}
}

func TestReqiCodeTool_GetResult_ErrorAssessment(t *testing.T) {
	tool := newTestReqiCodeTool(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":       "run-err",
			"task_id":  "task-err",
			"status":   "FAILED",
			"error":    "task failed during execution",
			"result": map[string]any{
				"status":              "failed",
				"verification_status": "failed",
				"duration_ms":         5000,
				"error_summary":       "compilation error in main.go",
			},
		})
	})

	result, err := tool.Execute(context.Background(), map[string]any{
		"action": "get_result",
		"run_id": "run-err",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "FAILED") {
		t.Errorf("expected FAILED status in result, got: %s", result)
	}
	if !strings.Contains(result, "Error Summary") {
		t.Errorf("expected error summary in result, got: %s", result)
	}
}

func TestReqiCodeTool_GetResult_KnownLimitations(t *testing.T) {
	tool := newTestReqiCodeTool(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":       "run-lim",
			"task_id":  "task-lim",
			"status":   "COMPLETED",
			"result": map[string]any{
				"status":              "completed",
				"verification_status": "passed",
				"duration_ms":         800,
				"known_limitations":   "Edge cases not covered",
			},
		})
	})

	result, err := tool.Execute(context.Background(), map[string]any{
		"action": "get_result",
		"run_id": "run-lim",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !strings.Contains(result, "Known Limitations") {
		t.Errorf("expected known limitations in result, got: %s", result)
	}
}
