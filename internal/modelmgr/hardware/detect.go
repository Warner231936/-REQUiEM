package hardware

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Detector interface {
	Detect(ctx context.Context) (*HardwareSpec, error)
}

type DetectorConfig struct {
	OllamaURL string
}

type defaultDetector struct {
	logger    *slog.Logger
	ollamaURL string
}

func NewDetector(logger *slog.Logger, cfg *DetectorConfig) Detector {
	d := &defaultDetector{logger: logger}
	if cfg != nil {
		d.ollamaURL = cfg.OllamaURL
	}
	if d.ollamaURL == "" {
		d.ollamaURL = "http://127.0.0.1:11434"
	}
	return d
}

func (d *defaultDetector) Detect(ctx context.Context) (*HardwareSpec, error) {
	spec := &HardwareSpec{
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		DetectedAt: time.Now().UTC(),
	}

	cpu, err := d.detectCPU(ctx)
	if err != nil {
		d.logger.Warn("cpu detection failed", "error", err)
	}
	spec.CPU = cpu

	ramMB, err := d.detectRAM(ctx)
	if err != nil {
		d.logger.Warn("ram detection failed", "error", err)
	}
	spec.RAMMB = ramMB

	gpus, err := d.detectGPUs(ctx)
	if err != nil {
		d.logger.Warn("gpu detection failed", "error", err)
	}
	spec.GPUs = gpus

	diskMB, err := d.detectDisk(ctx)
	if err != nil {
		d.logger.Warn("disk detection failed", "error", err)
	}
	spec.DiskMB = diskMB

	ollama, err := detectOllama(ctx, d.logger, d.ollamaURL)
	if err != nil {
		d.logger.Debug("ollama detection failed", "error", err)
	}
	spec.Ollama = ollama

	if ollama.Available && ollama.Running {
		spec.RuntimeBackends = append(spec.RuntimeBackends, "ollama")
	}
	if ramMB > 8192 {
		spec.RuntimeBackends = append(spec.RuntimeBackends, "llamacpp")
	}

	d.logger.Info("hardware detected",
		"cpu", cpu.Model,
		"cores", cpu.Cores,
		"ram_mb", ramMB,
		"gpus", len(gpus),
		"disk_mb", diskMB,
		"ollama_available", ollama.Available,
		"runtime_backends", spec.RuntimeBackends,
	)

	return spec, nil
}

func (d *defaultDetector) detectCPU(ctx context.Context) (CPUInfo, error) {
	info := CPUInfo{
		Cores:   runtime.NumCPU(),
		Threads: runtime.NumCPU(),
	}

	name, err := cpuModelName(ctx)
	if err != nil {
		return info, err
	}
	info.Model = name
	return info, nil
}

func (d *defaultDetector) detectRAM(ctx context.Context) (int, error) {
	return detectRAM(ctx)
}

func (d *defaultDetector) detectGPUs(ctx context.Context) ([]GPUInfo, error) {
	return detectGPUs(ctx, d.logger)
}

func (d *defaultDetector) detectDisk(ctx context.Context) (int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return detectDisk(home)
}

func detectOllama(ctx context.Context, logger *slog.Logger, ollamaURL string) (OllamaInfo, error) {
	info := OllamaInfo{}

	ollamaPath, err := findExecutable("ollama")
	if err != nil {
		info.Available = false
		return info, nil
	}

	info.Available = true

	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := execCommand(ctxTimeout, ollamaPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		// Try alternative: get version from API
		req, _ := httpNewRequest("GET", ollamaURL+"/api/version", nil)
		client := newClient(5 * time.Second)
		resp, err := client.do(ctxTimeout, req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == 200 {
				body, _ := io.ReadAll(resp.Body)
				var verResp struct {
					Version string `json:"version"`
				}
				if jsonUnmarshal(body, &verResp) == nil && verResp.Version != "" {
					// Clean up version string (Ollama API returns "0.34.0" but sometimes includes "is ")
					ver := strings.TrimSpace(verResp.Version)
					ver = strings.TrimPrefix(ver, "is ")
					ver = strings.TrimSpace(ver)
					if ver != "" && ver != "0.0.0" {
						info.Version = ver
					}
				}
			}
		}
	} else {
		verStr := strings.TrimSpace(string(output))
		verStr = strings.TrimPrefix(verStr, "ollama version ")
		verStr = strings.TrimPrefix(verStr, "ollama ")
		verStr = strings.TrimPrefix(verStr, "is ")
		verStr = strings.TrimSpace(verStr)
		if verStr != "" && verStr != "0.0.0" {
			info.Version = verStr
		}
	}

	info.Running = checkOllamaRunning(ctxTimeout, logger, ollamaURL)

	models, err := listOllamaModels(ctxTimeout, ollamaURL)
	if err == nil {
		info.Installed = models
	}

	home, err := os.UserHomeDir()
	if err == nil {
		ollamaDir := filepath.Join(home, ".ollama")
		if _, err := os.Stat(ollamaDir); err == nil {
			info.ModelsPath = ollamaDir
		}
	}

	return info, nil
}

func checkOllamaRunning(ctx context.Context, logger *slog.Logger, baseURL string) bool {
	if ctx.Err() != nil {
		logger.Debug("context cancelled before ollama check")
		return false
	}
	req, err := httpNewRequest("GET", baseURL+"/api/tags", nil)
	if err != nil {
		logger.Debug("failed to create ollama check request", "error", err)
		return false
	}
	client := newClient(5 * time.Second)
	resp, err := client.do(ctx, req)
	if err != nil {
		logger.Debug("ollama check request failed", "error", err)
		return false
	}
	defer resp.Body.Close()
	logger.Debug("ollama check response", "status", resp.StatusCode)
	return resp.StatusCode == 200
}

func listOllamaModels(ctx context.Context, baseURL string) ([]OllamaModelInfo, error) {
	req, err := httpNewRequest("GET", baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	client := newClient(10 * time.Second)
	resp, err := client.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ollama /api/tags returned %d", resp.StatusCode)
	}

	body, err := readAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tagsResp struct {
		Models []struct {
			Name     string `json:"name"`
			Size     int64  `json:"size"`
			Modified int64  `json:"modified_at"`
			Digest   string `json:"digest"`
		} `json:"models"`
	}
	if err := jsonUnmarshal(body, &tagsResp); err != nil {
		return nil, err
	}

	models := make([]OllamaModelInfo, 0, len(tagsResp.Models))
	for _, m := range tagsResp.Models {
		models = append(models, OllamaModelInfo{
			Name:     m.Name,
			Size:     m.Size,
			Modified: time.Unix(m.Modified, 0).UTC(),
			Digest:   m.Digest,
		})
	}
	return models, nil
}
