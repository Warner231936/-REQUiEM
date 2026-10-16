package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/requiem/cognitive/pkg/models"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig    `yaml:"server"`
	Auth       AuthConfig      `yaml:"auth"`
	Database   DatabaseConfig  `yaml:"database"`
	LLM        LLMConfig       `yaml:"llm"`
	Embeddings EmbeddingConfig `yaml:"embeddings"`
	Pipeline   PipelineConfig  `yaml:"pipeline"`
	Tools      ToolUseConfig   `yaml:"tools"`
}

type ToolUseConfig struct {
	Enabled       bool `yaml:"enabled"`
	MaxIterations int  `yaml:"max_iterations"`
}

type AuthConfig struct {
	Enabled                bool     `yaml:"enabled"`
	MasterKeyEnvVar        string   `yaml:"master_key_env_var"`
	SecretsFile            string   `yaml:"secrets_file"`
	AllowedOrigins         []string `yaml:"allowed_origins"`
	DefaultKey             string   `yaml:"default_key"`
	JWTSecret              string   `yaml:"jwt_secret"`
	BootstrapAdminEmail    string   `yaml:"bootstrap_admin_email"`
	BootstrapAdminPassword string   `yaml:"bootstrap_admin_password"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type DatabaseConfig struct {
	MongoDB MongoDBConfig `yaml:"mongodb"`
}

type MongoDBConfig struct {
	URI      string `yaml:"uri"`
	Database string `yaml:"database"`
}

type LLMConfig struct {
	DefaultProvider string                    `yaml:"default_provider"`
	Providers       map[string]LLMProviderCfg `yaml:"providers"`
}

type LLMProviderCfg struct {
	BaseUrl string `yaml:"base_url"`
}

type EmbeddingConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

type PipelineConfig struct {
	SynchronousConsolidation bool        `yaml:"synchronous_consolidation"`
	Debug                    bool        `yaml:"debug"`
	Stage0                   StageConfig `yaml:"stage0"`
	Stage1                   StageConfig `yaml:"stage1"`
	Stage2                   StageConfig `yaml:"stage2"`
	Stage3                   StageConfig `yaml:"stage3"`
	Stage4                   StageConfig `yaml:"stage4"`
	Stage5                   StageConfig `yaml:"stage5"`
	Stage6                   StageConfig `yaml:"stage6"`
}

type StageConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	content := expandEnvVars(data)

	var cfg Config
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	setDefaults(&cfg)
	applyEnvOverrides(&cfg)
	return &cfg, nil
}

func LoadFromEnv() *Config {
	cfg := &Config{}
	setDefaults(cfg)
	applyEnvOverrides(cfg)
	return cfg
}

func applyEnvOverrides(cfg *Config) {
	if uri := os.Getenv("MONGODB_URI"); uri != "" {
		cfg.Database.MongoDB.URI = uri
	}
	if db := os.Getenv("MONGODB_DATABASE"); db != "" {
		cfg.Database.MongoDB.Database = db
	}
	if debug := os.Getenv("COGNITIVE_DEBUG"); debug == "true" {
		cfg.Pipeline.Debug = true
	}
	if host := os.Getenv("SERVER_HOST"); host != "" {
		cfg.Server.Host = host
	}
	if port := os.Getenv("SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Server.Port = p
		}
	}
	if origin := os.Getenv("CORS_ALLOWED_ORIGIN"); origin != "" {
		cfg.Auth.AllowedOrigins = []string{origin}
	}
	if os.Getenv("REQUIEM_AUTH_DISABLED") == "true" {
		cfg.Auth.Enabled = false
	} else if val := os.Getenv("REQUIEM_AUTH_ENABLED"); val != "" {
		cfg.Auth.Enabled = (val == "true")
	}
	if jwtSecret := os.Getenv("JWT_SECRET"); jwtSecret != "" {
		cfg.Auth.JWTSecret = jwtSecret
	}
}

func expandEnvVars(data []byte) []byte {
	content := string(data)
	re := regexp.MustCompile(`\$\{([^}]+)\}`)

	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		placeholder := match[0]
		envExpr := match[1]

		parts := strings.SplitN(envExpr, ":-", 2)
		envVar := parts[0]
		defaultVal := ""
		if len(parts) == 2 {
			defaultVal = parts[1]
		}

		val := os.Getenv(envVar)
		if val == "" {
			val = defaultVal
		}

		content = strings.Replace(content, placeholder, val, 1)
	}

	return []byte(content)
}

func setDefaults(cfg *Config) {
	if cfg.Server.Host == "" {
		cfg.Server.Host = "127.0.0.1"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 15153
	}
	if cfg.Database.MongoDB.URI == "" {
		cfg.Database.MongoDB.URI = "mongodb://127.0.0.1:27017"
	}
	if cfg.Database.MongoDB.Database == "" {
		cfg.Database.MongoDB.Database = "requiem"
	}
	if cfg.LLM.DefaultProvider == "" {
		cfg.LLM.DefaultProvider = "ollama"
	}

	if cfg.Auth.MasterKeyEnvVar == "" {
		cfg.Auth.MasterKeyEnvVar = "REQUIEM_MASTER_KEY"
	}
	if cfg.Auth.SecretsFile == "" {
		cfg.Auth.SecretsFile = "data/secrets.json.enc"
	}
	if cfg.Auth.DefaultKey == "" {
		cfg.Auth.DefaultKey = ""
	}
	if cfg.Auth.JWTSecret == "" {
		cfg.Auth.JWTSecret = os.Getenv("JWT_SECRET")
	}
	if cfg.Auth.JWTSecret == "" {
		cfg.Auth.JWTSecret = "change-me-in-production-use-JWT_SECRET-env-var"
	}

	if cfg.Tools.MaxIterations == 0 {
		cfg.Tools.MaxIterations = 3
	}
}

func (c *Config) ToPipelineConfig() models.PipelineConfig {
	return models.PipelineConfig{
		SynchronousConsolidation: c.Pipeline.SynchronousConsolidation,
		Debug:                    c.Pipeline.Debug,
		ToolUse: models.ToolUseConfig{
			Enabled:       c.Tools.Enabled,
			MaxIterations: c.Tools.MaxIterations,
		},
		Stage0: models.StageConfig{
			Enabled:  c.Pipeline.Stage0.Enabled,
			Provider: c.Pipeline.Stage0.Provider,
			Model:    c.Pipeline.Stage0.Model,
		},
		Stage1: models.StageConfig{
			Enabled:  c.Pipeline.Stage1.Enabled,
			Provider: c.Pipeline.Stage1.Provider,
			Model:    c.Pipeline.Stage1.Model,
		},
		Stage2: models.StageConfig{
			Enabled:  c.Pipeline.Stage2.Enabled,
			Provider: c.Pipeline.Stage2.Provider,
			Model:    c.Pipeline.Stage2.Model,
		},
		Stage3: models.StageConfig{
			Enabled:  c.Pipeline.Stage3.Enabled,
			Provider: c.Pipeline.Stage3.Provider,
			Model:    c.Pipeline.Stage3.Model,
		},
		Stage4: models.StageConfig{
			Enabled:  c.Pipeline.Stage4.Enabled,
			Provider: c.Pipeline.Stage4.Provider,
			Model:    c.Pipeline.Stage4.Model,
		},
		Stage5: models.StageConfig{
			Enabled:  c.Pipeline.Stage5.Enabled,
			Provider: c.Pipeline.Stage5.Provider,
			Model:    c.Pipeline.Stage5.Model,
		},
		Stage6: models.StageConfig{
			Enabled:  c.Pipeline.Stage6.Enabled,
			Provider: c.Pipeline.Stage6.Provider,
			Model:    c.Pipeline.Stage6.Model,
		},
	}
}

func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
