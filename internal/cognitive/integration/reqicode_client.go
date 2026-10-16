package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/requiem/cognitive/pkg/models"
)

type ReQiCodeClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewReQiCodeClient(baseURL string, timeoutSec int) *ReQiCodeClient {
	to := 120 * time.Second
	if timeoutSec > 0 {
		to = time.Duration(timeoutSec) * time.Second
	}
	return &ReQiCodeClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: to,
		},
	}
}

func (c *ReQiCodeClient) SubmitTask(ctx context.Context, req models.TaskRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal task request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/engineering", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("submit task to reqicode: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("reqicode returned %d: %s", resp.StatusCode, string(respBody))
	}

	var submitResp struct {
		Status string `json:"status"`
		RunID  string `json:"run_id"`
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(respBody, &submitResp); err != nil {
		return "", fmt.Errorf("parse submit response: %w", err)
	}

	return submitResp.RunID, nil
}

func (c *ReQiCodeClient) GetTaskResult(ctx context.Context, runID string) (models.TaskResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/runs/"+runID, nil)
	if err != nil {
		return models.TaskResult{}, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return models.TaskResult{}, fmt.Errorf("get task result from reqicode: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return models.TaskResult{}, fmt.Errorf("reqicode returned %d: %s", resp.StatusCode, string(respBody))
	}

	var runResp struct {
		ID           string `json:"id"`
		TaskID       string `json:"task_id"`
		Status       string `json:"status"`
		Error        string `json:"error,omitempty"`
		OutputPath   string `json:"output_path"`
		Result       *struct {
			Status             string                 `json:"status"`
			Objective          string                 `json:"objective,omitempty"`
			FilesChanged       []models.TaskFileChange `json:"files_changed,omitempty"`
			TestResults        []models.TaskTestResult `json:"test_results,omitempty"`
			BenchmarkResults   []map[string]any       `json:"benchmark_results,omitempty"`
			CommandsExecuted   []models.TaskCommandExec `json:"commands_executed,omitempty"`
			VerificationStatus string                 `json:"verification_status"`
			ErrorSummary       string                 `json:"error_summary,omitempty"`
			KnownLimitations   string                 `json:"known_limitations,omitempty"`
			DurationMs         int64                  `json:"duration_ms"`
		} `json:"result,omitempty"`
	}

	if err := json.Unmarshal(respBody, &runResp); err != nil {
		return models.TaskResult{}, fmt.Errorf("parse run response: %w", err)
	}

	taskResult := models.TaskResult{
		RunID:             runResp.ID,
		TaskID:            runResp.TaskID,
		Status:            models.TaskLifecycleStatus(runResp.Status),
		ErrorSummary:      runResp.Error,
	}

	if runResp.Result != nil {
		taskResult.Objective = runResp.Result.Objective
		taskResult.FilesChanged = runResp.Result.FilesChanged
		taskResult.TestResults = runResp.Result.TestResults
		taskResult.BenchmarkResults = runResp.Result.BenchmarkResults
		taskResult.CommandsExecuted = runResp.Result.CommandsExecuted
		taskResult.VerificationStatus = runResp.Result.VerificationStatus
		taskResult.ErrorSummary = runResp.Result.ErrorSummary
		taskResult.KnownLimitations = runResp.Result.KnownLimitations
		taskResult.DurationMs = runResp.Result.DurationMs
	}

	return taskResult, nil
}

func (c *ReQiCodeClient) CancelTask(ctx context.Context, runID string) error {
	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+"/api/runs/"+runID, nil)
	if err != nil {
		return fmt.Errorf("create cancel request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("cancel task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reqicode returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *ReQiCodeClient) WaitForCompletion(ctx context.Context, runID string, pollInterval time.Duration) (models.TaskResult, error) {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return models.TaskResult{}, ctx.Err()
		case <-ticker.C:
			result, err := c.GetTaskResult(ctx, runID)
			if err != nil {
				return models.TaskResult{}, err
			}

			if result.Status == models.TaskStatusCompleted ||
				result.Status == models.TaskStatusFailed {
				return result, nil
			}
		}
	}
}

func (c *ReQiCodeClient) BuildVerificationPlan(claimID, objective string, criteria []string) models.VerificationPlan {
	plan := models.VerificationPlan{
		ClaimID:              claimID,
		Objective:            objective,
		ClaimedVerifications: criteria,
		Methods: []models.VerificationMethod{
			models.MethodUnitTests,
			models.MethodCompilation,
			models.MethodStaticAnalysis,
		},
		CheckerReliability: 0.85,
		Reversible:         true,
	}
	return plan
}

func (c *ReQiCodeClient) AssessEvidence(result models.TaskResult) models.ValidityStatus {
	switch result.Status {
	case models.TaskStatusCompleted:
		if result.VerificationStatus == "passed" || result.VerificationStatus == "completed" {
			return models.ValidityConfirmed
		}
		if result.VerificationStatus == "failed" {
			return models.ValidityRefuted
		}
		return models.ValidityUnverified
	case models.TaskStatusFailed:
		return models.ValidityRefuted
	default:
		if result.VerificationStatus == "" && len(result.TestResults) == 0 {
			return models.ValidityInsufficient
		}
		return models.ValidityUnverified
	}
}
