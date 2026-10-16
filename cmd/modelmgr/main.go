package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/requiem/cognitive/internal/modelmgr"
)

type commandFunc func(context.Context, *modelmgr.ModelManager, *modelmgr.Config, *flag.FlagSet) error

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]
	if cmd == "help" || cmd == "--help" || cmd == "-h" {
		printUsage()
		return
	}

	flags := flag.NewFlagSet(cmd, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", "config/model-manager.yaml", "model manager configuration")
	jsonOutput := flags.Bool("json", false, "emit JSON output")
	timeout := flags.Duration("timeout", 35*time.Minute, "command timeout")

	// Command-specific flags
	var parallel, noActivate, force bool
	if cmd == "bootstrap" {
		flags.BoolVar(&parallel, "parallel", false, "acquire tiers concurrently")
		flags.BoolVar(&noActivate, "no-activate", false, "do not activate healthy models")
		flags.BoolVar(&force, "force", false, "ignore existing usable models")
	}

	if err := flags.Parse(args); err != nil {
		os.Exit(2)
	}

	cfg, err := modelmgr.LoadConfig(*configPath)
	if err != nil {
		exitf("load configuration: %v", err)
	}
	if err := cfg.EnsureDirectories(); err != nil {
		exitf("prepare model directories: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	manager := modelmgr.NewModelManager(modelmgr.ModelManagerConfig{Config: cfg}, logger)

	handlers := map[string]commandFunc{
		"detect":           runDetect,
		"hardware":         runDetect,
		"list":             runList,
		"list_models":      runList,
		"get":              runGet,
		"get_model":        runGet,
		"active":           runActive,
		"get_active_model": runActive,
		"request":          runRequest,
		"acquire":          runRequest,
		"bootstrap": func(ctx context.Context, mgr *modelmgr.ModelManager, cfg *modelmgr.Config, fs *flag.FlagSet) error {
			return runBootstrap(ctx, mgr, cfg, fs, parallel, noActivate, force)
		},
		"recover":    runRecover,
		"repair":     runRepair,
		"test":       runTest,
		"test_model": runTest,
		"activate":   runActivate,
		"remove":     runRemove,
		"update":     runUpdate,
		"registry":   runList,
	}

	handler, ok := handlers[cmd]
	if !ok {
		exitf("unknown command: %s", cmd)
	}

	if err := handler(ctx, manager, &cfg, flags); err != nil {
		exitf("%v", err)
	}

	if *jsonOutput {
		return
	}
}

func runDetect(ctx context.Context, manager *modelmgr.ModelManager, cfg *modelmgr.Config, flags *flag.FlagSet) error {
	spec, err := manager.DetectHardware(ctx)
	if err != nil {
		return err
	}
	return printJSON(spec)
}

func runList(ctx context.Context, manager *modelmgr.ModelManager, cfg *modelmgr.Config, flags *flag.FlagSet) error {
	models, err := manager.ListModels()
	if err != nil {
		return err
	}
	return printJSON(map[string]any{"models": models, "count": len(models)})
}

func runGet(ctx context.Context, manager *modelmgr.ModelManager, cfg *modelmgr.Config, flags *flag.FlagSet) error {
	id := flags.Arg(0)
	if id == "" {
		return fmt.Errorf("model id is required")
	}
	model, err := manager.GetModel(id)
	if err != nil {
		return err
	}
	return printJSON(model)
}

func runActive(ctx context.Context, manager *modelmgr.ModelManager, cfg *modelmgr.Config, flags *flag.FlagSet) error {
	tier := flags.Arg(0)
	if tier == "" {
		tier = "general"
	}
	parsed, err := modelmgr.ParseCapabilityTier(tier)
	if err != nil {
		return err
	}
	model, err := manager.GetActiveModel(parsed)
	if err != nil {
		return err
	}
	return printJSON(model)
}

func runRequest(ctx context.Context, manager *modelmgr.ModelManager, cfg *modelmgr.Config, flags *flag.FlagSet) error {
	var req modelmgr.ModelRequest
	if path := flags.Arg(0); path != "" && path != "-" {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &req); err != nil {
			return err
		}
	} else {
		decoder := json.NewDecoder(os.Stdin)
		if err := decoder.Decode(&req); err != nil && err != io.EOF {
			return err
		}
	}

	if req.Action == "" {
		req.Action = "acquire_model"
	}
	if req.Tier == "" {
		req.Tier = string(modelmgr.TierGeneral)
	}
	if req.Reason == "" {
		req.Reason = string(modelmgr.ReasonNewRequirement)
	}

	result, err := manager.RequestModel(ctx, req)
	if err != nil {
		return err
	}
	return printJSON(result)
}

func runBootstrap(ctx context.Context, manager *modelmgr.ModelManager, cfg *modelmgr.Config, flags *flag.FlagSet, parallel, noActivate, force bool) error {
	bootstrapConfig := modelmgr.DefaultBootstrapConfig()
	bootstrapConfig.Parallel = parallel
	bootstrapConfig.AutoActivate = !noActivate
	bootstrapConfig.SkipExisting = !force

	results, err := manager.Bootstrap(ctx, bootstrapConfig)
	if err != nil {
		return err
	}
	return printJSON(results)
}

func runRecover(ctx context.Context, manager *modelmgr.ModelManager, cfg *modelmgr.Config, flags *flag.FlagSet) error {
	report, err := manager.RunRecovery(ctx)
	if err != nil {
		return err
	}
	return printJSON(report)
}

func runRepair(ctx context.Context, manager *modelmgr.ModelManager, cfg *modelmgr.Config, flags *flag.FlagSet) error {
	id := flags.Arg(0)
	if id == "" {
		return fmt.Errorf("model id is required")
	}
	if err := manager.RepairModel(ctx, id); err != nil {
		return err
	}
	return printJSON(map[string]string{"status": "repaired", "model_id": id})
}

func runTest(ctx context.Context, manager *modelmgr.ModelManager, cfg *modelmgr.Config, flags *flag.FlagSet) error {
	id := flags.Arg(0)
	if id == "" {
		return fmt.Errorf("model id is required")
	}
	health, err := manager.TestModel(ctx, id)
	if err != nil {
		return err
	}
	return printJSON(health)
}

func runActivate(ctx context.Context, manager *modelmgr.ModelManager, cfg *modelmgr.Config, flags *flag.FlagSet) error {
	tierArg := flags.Arg(0)
	modelID := flags.Arg(1)
	if tierArg == "" || modelID == "" {
		return fmt.Errorf("tier and model id are required")
	}
	tier, err := modelmgr.ParseCapabilityTier(tierArg)
	if err != nil {
		return err
	}
	if err := manager.Activate(ctx, tier, modelID); err != nil {
		return err
	}
	return printJSON(map[string]string{"status": "activated", "tier": tierArg, "model_id": modelID})
}

func runRemove(ctx context.Context, manager *modelmgr.ModelManager, cfg *modelmgr.Config, flags *flag.FlagSet) error {
	id := flags.Arg(0)
	if id == "" {
		return fmt.Errorf("model id is required")
	}
	if err := manager.RemoveModel(ctx, id); err != nil {
		return err
	}
	return printJSON(map[string]string{"status": "removed", "model_id": id})
}

func runUpdate(ctx context.Context, manager *modelmgr.ModelManager, cfg *modelmgr.Config, flags *flag.FlagSet) error {
	id := flags.Arg(0)
	if id == "" {
		return fmt.Errorf("model id is required")
	}
	if err := manager.UpdateModel(ctx, id); err != nil {
		return err
	}
	return printJSON(map[string]string{"status": "updated", "model_id": id})
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, strings.TrimSpace(`
Requiem Model Manager

Usage:
  modelmgr <command> [options]

Commands:
  detect       Detect hardware and Ollama runtime
  list         List registered models
  get          Get one model by id
  active       Show the active model for a tier
  request      Submit an acquire_model request (JSON file or stdin)
  bootstrap    Acquire fast, general, and high capability tiers
  recover      Scan and repair missing, corrupt, or failed models
  repair       Repair one registered model
  test         Run a model load and inference health test
  activate     Activate a model for a tier
  remove       Remove a model from runtime and registry
  update       Re-acquire a model from its catalog definition

Options:
  -config path       Model manager YAML configuration
  -json              Emit JSON output
  -timeout duration  Command timeout
`))
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
