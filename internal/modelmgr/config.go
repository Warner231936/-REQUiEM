package modelmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ModelsDir         string        `yaml:"models_dir"`
	ManifestDir       string        `yaml:"manifest_dir"`
	RegistryFile      string        `yaml:"registry_file"`
	DownloadRetries   int           `yaml:"download_retries"`
	DownloadTimeout   time.Duration `yaml:"download_timeout"`
	ChunkSize         int64         `yaml:"chunk_size"`
	MaxConcurrentDL   int           `yaml:"max_concurrent_downloads"`
	VerifyWorkers     int           `yaml:"verify_workers"`
	HealthTestPrompt  string        `yaml:"health_test_prompt"`
	HealthTestExpect  string        `yaml:"health_test_expect"`
	UtilizationTarget float64       `yaml:"utilization_target"`
	OllamaURL         string        `yaml:"ollama_url"`
	TempDir           string        `yaml:"temp_dir"`
	LogLevel          string        `yaml:"log_level"`
}

func DefaultConfig(baseDir string) Config {
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		baseDir = filepath.Join(home, ".requiem", "models")
	}

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		absBase = baseDir
	}

	return Config{
		ModelsDir:         filepath.Join(absBase, "models"),
		ManifestDir:       filepath.Join(absBase, "manifests"),
		RegistryFile:      filepath.Join(absBase, "registry.json"),
		DownloadRetries:   3,
		DownloadTimeout:   30 * time.Minute,
		ChunkSize:         1024 * 1024,
		MaxConcurrentDL:   2,
		VerifyWorkers:     2,
		HealthTestPrompt:  "Briefly confirm you are ready to assist with a short acknowledgment.",
		HealthTestExpect:  "",
		UtilizationTarget: 0.70,
		OllamaURL:         "http://127.0.0.1:11434",
		TempDir:           filepath.Join(absBase, "tmp"),
		LogLevel:          "info",
	}
}

func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig("")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if cfg.ModelsDir == "" {
		cfg.ModelsDir = DefaultConfig("").ModelsDir
	}
	if cfg.ManifestDir == "" {
		cfg.ManifestDir = DefaultConfig("").ManifestDir
	}
	if cfg.RegistryFile == "" {
		cfg.RegistryFile = DefaultConfig("").RegistryFile
	}
	if cfg.TempDir == "" {
		cfg.TempDir = DefaultConfig("").TempDir
	}
	if cfg.DownloadRetries == 0 {
		cfg.DownloadRetries = 3
	}
	if cfg.DownloadTimeout == 0 {
		cfg.DownloadTimeout = 30 * time.Minute
	}
	if cfg.ChunkSize == 0 {
		cfg.ChunkSize = 1024 * 1024
	}
	if cfg.MaxConcurrentDL == 0 {
		cfg.MaxConcurrentDL = 2
	}
	if cfg.VerifyWorkers == 0 {
		cfg.VerifyWorkers = 2
	}
	if cfg.HealthTestPrompt == "" {
		cfg.HealthTestPrompt = DefaultConfig("").HealthTestPrompt
	}
	if cfg.UtilizationTarget == 0 {
		cfg.UtilizationTarget = 0.70
	}
	if cfg.OllamaURL == "" {
		cfg.OllamaURL = "http://127.0.0.1:11434"
	}
	return cfg, nil
}

func (c *Config) EnsureDirectories() error {
	dirs := []string{c.ModelsDir, c.ManifestDir, c.TempDir}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
	}
	return nil
}
