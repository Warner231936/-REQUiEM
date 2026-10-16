package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Metrics struct {
	mu sync.RWMutex

	StageDurations map[int][]int64
	StageCounts    map[int]int64
	StageErrors    map[int]int64
	TokenCounts    map[string]int64
	MemoryCounts   map[string]int64
	PipelineCount  int64
	PipelineErrors int64
	TotalDuration  int64
	ActiveRequests int64
}

func NewMetrics() *Metrics {
	return &Metrics{
		StageDurations: make(map[int][]int64),
		StageCounts:    make(map[int]int64),
		StageErrors:    make(map[int]int64),
		TokenCounts:    make(map[string]int64),
		MemoryCounts:   make(map[string]int64),
	}
}

func (m *Metrics) RecordStageDuration(stage int, durationMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StageDurations[stage] = append(m.StageDurations[stage], durationMs)
	m.StageCounts[stage]++
}

func (m *Metrics) RecordStageError(stage int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StageErrors[stage]++
}

func (m *Metrics) RecordTokens(stage string, count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TokenCounts[stage] += int64(count)
}

func (m *Metrics) RecordMemory(operation string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MemoryCounts[operation]++
}

func (m *Metrics) RecordPipeline(durationMs int64, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PipelineCount++
	m.TotalDuration += durationMs
	if !success {
		m.PipelineErrors++
	}
}

func (m *Metrics) RecordActiveRequest(delta int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActiveRequests += delta
}

func (m *Metrics) GetSnapshot() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stageAvgDurations := make(map[string]float64)
	for stage, durations := range m.StageDurations {
		if len(durations) > 0 {
			var sum int64
			for _, d := range durations {
				sum += d
			}
			stageAvgDurations["stage_"+strconv.Itoa(stage)] = float64(sum) / float64(len(durations))
		}
	}

	return map[string]any{
		"pipeline_count":      m.PipelineCount,
		"pipeline_errors":     m.PipelineErrors,
		"total_duration_ms":   m.TotalDuration,
		"active_requests":     m.ActiveRequests,
		"stage_counts":        m.StageCounts,
		"stage_errors":        m.StageErrors,
		"stage_avg_durations": stageAvgDurations,
		"token_counts":        m.TokenCounts,
		"memory_counts":       m.MemoryCounts,
		"timestamp":           time.Now().UTC().Format(time.RFC3339),
	}
}

func (m *Metrics) MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snapshot := m.GetSnapshot()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snapshot)
	}
}

func (m *Metrics) PrometheusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Write(m.PrometheusFormat())
	}
}

func (m *Metrics) RequestCountMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.RecordActiveRequest(1)
		defer m.RecordActiveRequest(-1)
		next.ServeHTTP(w, r)
	})
}

func (m *Metrics) PrometheusFormat() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("# HELP requiem_pipeline_total Total number of pipeline executions\n")
	sb.WriteString("# TYPE requiem_pipeline_total counter\n")
	sb.WriteString(fmt.Sprintf("requiem_pipeline_total %d\n", m.PipelineCount))

	sb.WriteString("# HELP requiem_pipeline_errors Total number of pipeline errors\n")
	sb.WriteString("# TYPE requiem_pipeline_errors counter\n")
	sb.WriteString(fmt.Sprintf("requiem_pipeline_errors %d\n", m.PipelineErrors))

	sb.WriteString("# HELP requiem_pipeline_duration_ms Total pipeline duration in milliseconds\n")
	sb.WriteString("# TYPE requiem_pipeline_duration_ms counter\n")
	sb.WriteString(fmt.Sprintf("requiem_pipeline_duration_ms %d\n", m.TotalDuration))

	sb.WriteString("# HELP requiem_active_requests Number of active requests\n")
	sb.WriteString("# TYPE requiem_active_requests gauge\n")
	sb.WriteString(fmt.Sprintf("requiem_active_requests %d\n", m.ActiveRequests))

	for stage, count := range m.StageCounts {
		sb.WriteString(fmt.Sprintf("requiem_stage_count{stage=\"%d\"} %d\n", stage, count))
	}
	for stage, errs := range m.StageErrors {
		sb.WriteString(fmt.Sprintf("requiem_stage_errors{stage=\"%d\"} %d\n", stage, errs))
	}
	for stage, durations := range m.StageDurations {
		if len(durations) > 0 {
			var sum int64
			for _, d := range durations {
				sum += d
			}
			avg := float64(sum) / float64(len(durations))
			sb.WriteString(fmt.Sprintf("requiem_stage_avg_duration_ms{stage=\"%d\"} %.2f\n", stage, avg))
		}
	}
	for k, v := range m.TokenCounts {
		sb.WriteString(fmt.Sprintf("requiem_tokens{stage=\"%s\"} %d\n", k, v))
	}
	for k, v := range m.MemoryCounts {
		sb.WriteString(fmt.Sprintf("requiem_memory_operations{operation=\"%s\"} %d\n", k, v))
	}

	sb.WriteString(fmt.Sprintf("requiem_metrics_timestamp %d\n", time.Now().Unix()))

	return []byte(sb.String())
}
