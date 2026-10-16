package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/requiem/cognitive/internal/api"
	"github.com/requiem/cognitive/internal/cognitive/integration"
	"github.com/requiem/cognitive/internal/cognitive/mind"
	"github.com/requiem/cognitive/internal/cognitive/pipeline"
	"github.com/requiem/cognitive/internal/cognitive/tool"
	"github.com/requiem/cognitive/internal/config"
	"github.com/requiem/cognitive/internal/events"
	"github.com/requiem/cognitive/internal/explainability"
	"github.com/requiem/cognitive/internal/llm"
	"github.com/requiem/cognitive/internal/metrics"
	"github.com/requiem/cognitive/internal/security"
	"github.com/requiem/cognitive/internal/storage"
	"github.com/requiem/cognitive/pkg/models"
)

var startTime time.Time

func main() {
	startTime = time.Now().UTC()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		logger.Warn("failed to load config file, using defaults", "error", err)
		cfg = config.LoadFromEnv()
	}

	// Config watcher for hot-reload will be set up after pipeline creation
	// (moved below to capture pipeline reference)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoStore, err := storage.NewMongoStore(ctx, storage.MongoConfig{
		URI:      cfg.Database.MongoDB.URI,
		Database: cfg.Database.MongoDB.Database,
	}, logger)
	if err != nil {
		logger.Error("failed to connect to MongoDB", "error", err)
		os.Exit(1)
	}
	defer mongoStore.Close(context.Background())

	registry := llm.NewProviderRegistry(logger)

	var embeddingService *llm.EmbeddingService
	if ollamaURL, ok := cfg.LLM.Providers["ollama"]; ok {
		ollamaProvider := llm.NewOllamaProvider(ollamaURL.BaseUrl, logger)
		registry.Register("ollama", ollamaProvider)
		logger.Info("ollama provider registered", "url", ollamaURL.BaseUrl)

		ollamaEmbedder := llm.NewOllamaEmbeddingProvider(ollamaURL.BaseUrl, cfg.Embeddings.Model, logger)
		registry.RegisterEmbedder("ollama-embeddings", ollamaEmbedder)
		embeddingService = llm.NewEmbeddingService(ollamaEmbedder)
	}

	eventBus := events.NewEventBus(logger)
	driftDetector := mind.NewDriftDetector(mongoStore, logger)
	snapshotService := mind.NewSnapshotService(mongoStore, logger)
	decayService := mind.NewDecayService(mongoStore, mind.DefaultDecayConfig(), logger)
	hypothesisService := mind.NewHypothesisService(mongoStore, mind.DefaultHypothesisPromotionRule(), logger)
	valueService := mind.NewValueService(mongoStore, mind.DefaultValueModificationRule(), logger)
	metricsCollector := metrics.NewMetrics()
	healthChecker := api.NewHealthChecker(mongoStore, registry, logger)

	go runDecayService(ctx, decayService, logger)
	go runHypothesisPromotion(ctx, hypothesisService, mongoStore, logger)
	go runValueMaintenance(ctx, valueService, mongoStore, logger)

	// Create tool registry and register built-in tools
	toolRegistry := tool.NewToolRegistry()
	toolRegistry.Register(tool.NewSearchMemoriesTool(mongoStore, embeddingService))
	toolRegistry.Register(tool.NewListBeliefsTool(mongoStore))
	toolRegistry.Register(tool.NewListOpinionsTool(mongoStore))
	toolRegistry.Register(tool.NewListPreferencesTool(mongoStore))
	toolRegistry.Register(tool.NewListGoalsTool(mongoStore))
	toolRegistry.Register(tool.NewGetCurrentTimeTool())
	toolRegistry.Register(tool.NewSearchConversationsTool(mongoStore))
	toolRegistry.Register(tool.NewReqiCodeTool(getEnvOrDefault("REQICODE_DAEMON_URL", "http://localhost:8080")))

	p := pipeline.NewPipeline(cfg.ToPipelineConfig(), registry, mongoStore, logger)
	p.SetEventBus(eventBus)

	reqicodeClient := integration.NewReQiCodeClient(getEnvOrDefault("REQICODE_DAEMON_URL", "http://localhost:8080"), 120)
	verificationPolicy := pipeline.NewVerificationPolicy(
		mongoStore,
		reqicodeClient,
		logger,
		models.Confidence(0.8),
		30*time.Second,
	)

	p.RegisterStage(0, pipeline.NewStage0Perception(registry, logger))
	p.RegisterStage(1, pipeline.NewStage1Recall(mongoStore, embeddingService, logger))
	p.RegisterStage(2, pipeline.NewStage2Interpretation(registry, logger))
	p.RegisterStage(3, pipeline.NewStage3Reflection(registry, logger))
	p.RegisterStage(4, pipeline.NewStage4Response(registry, logger,
		pipeline.WithTools(toolRegistry),
		pipeline.WithEmbeddingService(embeddingService),
		pipeline.WithMaxToolIterations(cfg.Tools.MaxIterations),
		pipeline.WithVerificationPolicy(verificationPolicy),
	))
	p.RegisterStage(5, pipeline.NewStage5ResponseHumanize(registry, logger))
	stage6 := pipeline.NewStage6Consolidation(mongoStore, embeddingService, logger)
	stage6.SetVerificationPolicy(verificationPolicy)
	p.RegisterStage(6, stage6)

	// Config watcher for hot-reload
	configWatcher, err := config.NewWatcher("config/config.yaml", logger)
	if err != nil {
		logger.Warn("failed to create config watcher, hot-reload disabled", "error", err)
	} else {
		configWatcher.SetConfig(cfg)
		if err := configWatcher.Start(); err != nil {
			logger.Warn("failed to start config watcher", "error", err)
		}
		configWatcher.OnChange(func(newCfg *config.Config) {
			logger.Info("config reloaded, applying changes",
				"old_port", cfg.Server.Port,
				"new_port", newCfg.Server.Port,
				"pipeline_stages", countEnabledStages(newCfg),
			)
			cfg = newCfg
			p.UpdateConfig(cfg.ToPipelineConfig())
		})
		defer configWatcher.Stop()
	}

	apiHandler := api.NewHandlerWithEmbeddings(p, mongoStore, embeddingService, logger)
	mindController := api.NewMindController(mongoStore, logger)
	conversationController := api.NewConversationController(mongoStore, logger)
	explainabilityService := explainability.NewExplainabilityService(mongoStore)
	sseHandler := events.NewSSEHandler(eventBus, logger, cfg.Auth.AllowedOrigins).WithMindStore(mongoStore.Mind())
	timeoutMiddleware := api.NewTimeoutMiddleware(120)
	rateLimiter := api.NewRateLimiter(50, 100, logger)
	sizeLimit := api.NewRequestSizeLimit(1<<20, logger)

	secretsStore := setupSecrets(cfg, logger)
	secMiddleware := security.NewMiddleware(secretsStore, logger, cfg.Auth.AllowedOrigins)

	mux := http.NewServeMux()

	mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := healthChecker.Check(r.Context())
		httpStatus := http.StatusOK
		if status.Status == "degraded" {
			httpStatus = http.StatusServiceUnavailable
		}
		writeJSON(w, httpStatus, status)
	}))

	mux.HandleFunc("/api/metrics", metricsCollector.MetricsHandler())
	mux.HandleFunc("/metrics", metricsCollector.PrometheusHandler())

	mux.HandleFunc("/api/chat", apiHandler.HandleChat)
	mux.Handle("/api/cognitive/", http.HandlerFunc(apiHandler.HandleCognitiveDebug))
	mux.Handle("/api/events/stream", sseHandler)

	requiemHome := os.Getenv("REQUIEM_HOME")
	if requiemHome == "" {
		requiemHome = "."
	}
	keyfilePath := filepath.Join(requiemHome, "public/keyfiles/requiem.key")
	authService := api.NewAuthService(mongoStore, cfg.Auth.JWTSecret, keyfilePath, logger)
	secMiddleware.SetSessionValidator(authService.ValidateSessionAndReturnBool)

	protectedMux := http.NewServeMux()
	protectedMux.HandleFunc("/api/mind", apiHandler.HandleMind)
	protectedMux.HandleFunc("/api/mind/beliefs", apiHandler.HandleBeliefs)
	protectedMux.HandleFunc("/api/mind/opinions", apiHandler.HandleOpinions)
	protectedMux.HandleFunc("/api/mind/preferences", apiHandler.HandlePreferences)
	protectedMux.HandleFunc("/api/mind/values", apiHandler.HandleValues)
	protectedMux.HandleFunc("/api/mind/goals", apiHandler.HandleGoals)
	protectedMux.HandleFunc("/api/mind/hypotheses", apiHandler.HandleHypotheses)
	protectedMux.HandleFunc("/api/mind/uncertainties", apiHandler.HandleUncertainties)
	protectedMux.HandleFunc("/api/mind/self", apiHandler.HandleSelfObservations)
	protectedMux.HandleFunc("/api/mind/contradictions", apiHandler.HandleContradictions)
	protectedMux.HandleFunc("/api/mind/history", apiHandler.HandleMindHistory)
	protectedMux.HandleFunc("/api/memory", apiHandler.HandleMemory)
	protectedMux.HandleFunc("/api/memory/", apiHandler.HandleMemoryByID)
	protectedMux.HandleFunc("/api/embeddings/backfill", apiHandler.HandleEmbeddingsBackfill)
	protectedMux.HandleFunc("/api/belief/explain", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		beliefID := r.URL.Query().Get("id")
		if beliefID == "" {
			writeError(w, http.StatusBadRequest, "belief id is required")
			return
		}
		explanation, err := explainabilityService.ExplainBelief(r.Context(), beliefID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to generate explanation: %v", err))
			return
		}
		writeJSON(w, http.StatusOK, explanation)
	})
	protectedMux.HandleFunc("/api/mind/drift", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		report, err := driftDetector.DetectDrift(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, report)
	})
	protectedMux.HandleFunc("/api/mind/snapshot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		snapshot, err := snapshotService.CreateSnapshot(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	})
	mindController.RegisterAsProtected(protectedMux)
	conversationController.RegisterAsProtected(protectedMux)

	var protectedHandler http.Handler
	if !cfg.Auth.Enabled {
		protectedHandler = protectedMux
		logger.Info("authentication disabled - all endpoints accessible")
	} else if secretsStore != nil {
		protectedHandler = secMiddleware.RequireAuthOrSession(security.PermissionRead)(protectedMux)
		logger.Info("authentication enabled - protected endpoints require API key or session")
	} else {
		protectedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusUnauthorized, "authentication required")
		})
		logger.Warn("authentication enabled but secrets store unavailable - protected endpoints return 401")
	}

	mux.Handle("/api/mind", protectedHandler)
	mux.Handle("/api/mind/", protectedHandler)
	mux.Handle("/api/memory", protectedHandler)
	mux.Handle("/api/memory/", protectedHandler)
	mux.Handle("/api/embeddings/", protectedHandler)
	mux.Handle("/api/belief/", protectedHandler)

	authHandler := api.NewAuthHandler(mongoStore, authService, logger)

	// Bootstrap admin user if configured
	bootstrapAdmin(ctx, mongoStore, authService, cfg, logger)

	mux.HandleFunc("/api/auth/signup", authHandler.Signup)
	mux.HandleFunc("/api/auth/login", authHandler.Login)
	mux.HandleFunc("/api/auth/admin-login", authHandler.AdminLogin)
	mux.HandleFunc("/api/auth/verify", authHandler.VerifyEmail)
	mux.HandleFunc("/api/auth/forgot-password", authHandler.ForgotPassword)
	mux.HandleFunc("/api/auth/reset-password", authHandler.ResetPassword)
	mux.HandleFunc("/api/auth/resend-verification", authHandler.ResendVerification)
	mux.HandleFunc("/api/auth/logout", authHandler.Logout)
	mux.HandleFunc("/api/auth/me", authHandler.Me)

	// Static file serving for landing page (requiem.cfd)
	landingDir := filepath.Join(requiemHome, "web", "landing")
	landingFS := http.FileServer(http.Dir(landingDir))
	mux.Handle("/chat", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		http.ServeFile(w, r, filepath.Join(landingDir, "chat.html"))
	}))
	mux.Handle("/", landingFS)
	// Admin portal (admin.requiem.cfd) - served at /admin
	adminDir := filepath.Join(requiemHome, "web", "admin")
	adminFS := http.FileServer(http.Dir(adminDir))
	mux.Handle("/admin/", http.StripPrefix("/admin/", adminFS))
	mux.HandleFunc("/admin/dashboard", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/dashboard.html", http.StatusFound)
	})
	// Consciousness Streaming Dashboard
	dashboardDir := filepath.Join(requiemHome, "web", "dashboard")
	dashboardFS := http.FileServer(http.Dir(dashboardDir))
	dashboardHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		dashboardFS.ServeHTTP(w, r)
	})
	mux.Handle("/dashboard/", http.StripPrefix("/dashboard/", dashboardHandler))

	handlerWithMetrics := metricsCollector.RequestCountMiddleware(mux)
	handlerWithTimeout := timeoutMiddleware.Wrap(handlerWithMetrics)
	handlerWithSizeLimit := sizeLimit.Wrap(handlerWithTimeout)
	handlerWithRateLimit := rateLimiter.Wrap(handlerWithSizeLimit)
	handlerWithLogging := secMiddleware.Logging(handlerWithRateLimit)
	handlerWithRequestID := secMiddleware.RequestID(handlerWithLogging)
	wrappedHandler := secMiddleware.HardenedCORS(handlerWithRequestID)

	server := &http.Server{
		Addr:              cfg.Address(),
		Handler:           wrappedHandler,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("Requiem cognitive server starting",
			"address", cfg.Address(),
			"auth_enabled", cfg.Auth.Enabled,
			"allowed_origins", cfg.Auth.AllowedOrigins,
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...", "active_requests", metricsCollector.GetSnapshot()["active_requests"])

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	if secretsStore != nil {
		secretsPath := cfg.Auth.SecretsFile
		if secretsPath == "" {
			secretsPath = "data/secrets.json.enc"
		}
		if err := secretsStore.SaveToFile(secretsPath); err != nil {
			logger.Warn("failed to save secrets file", "error", err, "path", secretsPath)
		}
	}

	logger.Info("server stopped")
}

func setupSecrets(cfg *config.Config, logger *slog.Logger) *security.SecretsStore {
	if !cfg.Auth.Enabled {
		return nil
	}
	masterKey := os.Getenv(cfg.Auth.MasterKeyEnvVar)
	if masterKey == "" {
		masterKey = cfg.Auth.DefaultKey
	}
	if masterKey == "" {
		logger.Warn("no API key configured - using insecure default, NOT SECURE FOR PRODUCTION",
			"env_var", cfg.Auth.MasterKeyEnvVar)
		masterKey = "dev-mode-key-change-me"
	}

	secretsStore, err := security.NewSecretsStore(masterKey, logger)
	if err != nil {
		logger.Error("failed to initialize secrets store", "error", err)
		return nil
	}

	secretsFilePath := cfg.Auth.SecretsFile
	if secretsFilePath == "" {
		secretsFilePath = "data/secrets.json.enc"
	}

	if _, err := os.Stat(secretsFilePath); err == nil {
		if err := secretsStore.LoadFromFile(secretsFilePath); err != nil {
			logger.Warn("failed to load secrets from file, generating new key", "error", err, "path", secretsFilePath)
		} else {
			logger.Info("secrets loaded from file", "path", secretsFilePath, "keys", len(secretsStore.ListKeys()))
		}
	}

	if len(secretsStore.ListKeys()) == 0 {
		ttl := 8760 * time.Hour
		_, apiKey, err := secretsStore.GenerateKey("default-admin", []security.Permission{
			security.PermissionAdmin,
			security.PermissionRead,
			security.PermissionWrite,
			security.PermissionPipeline,
		}, &ttl)
		if err != nil {
			logger.Error("failed to generate default API key", "error", err)
			return nil
		}
		logger.Info("generated default API key",
			"key_id", apiKey.ID,
			"permissions", apiKey.Permissions,
			"expires_in_hours", int(ttl.Hours()),
		)
	}

	return secretsStore
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func countEnabledStages(cfg *config.Config) int {
	count := 0
	stages := []config.StageConfig{
		cfg.Pipeline.Stage0, cfg.Pipeline.Stage1, cfg.Pipeline.Stage2,
		cfg.Pipeline.Stage3, cfg.Pipeline.Stage4, cfg.Pipeline.Stage5,
		cfg.Pipeline.Stage6,
	}
	for _, s := range stages {
		if s.Enabled {
			count++
		}
	}
	return count
}

func runDecayService(ctx context.Context, service *mind.DecayService, logger *slog.Logger) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	logger.Info("memory decay service started", "interval", "1h")
	for {
		select {
		case <-ticker.C:
			if err := service.DecayConfidence(ctx); err != nil {
				logger.Warn("decay service error", "error", err)
			}
		case <-ctx.Done():
			logger.Info("memory decay service stopped")
			return
		}
	}
}

func runHypothesisPromotion(ctx context.Context, service *mind.HypothesisService, store *storage.MongoStore, logger *slog.Logger) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	logger.Info("hypothesis promotion service started", "interval", "6h")
	for {
		select {
		case <-ticker.C:
			hypotheses, err := store.Mind().GetHypotheses(ctx, models.MindQuery{Limit: 1000})
			if err != nil {
				logger.Warn("hypothesis promotion scan error", "error", err)
				continue
			}
			for _, h := range hypotheses {
				if service.CanPromote(ctx, h) {
					beliefID, err := service.PromoteToBelief(ctx, h.ID)
					if err != nil {
						logger.Warn("hypothesis promotion failed", "hypothesis_id", h.ID, "error", err)
					} else {
						logger.Info("hypothesis promoted to belief", "hypothesis_id", h.ID, "belief_id", beliefID)
					}
				}
			}
		case <-ctx.Done():
			logger.Info("hypothesis promotion service stopped")
			return
		}
	}
}

func runValueMaintenance(ctx context.Context, service *mind.ValueService, store *storage.MongoStore, logger *slog.Logger) {
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()
	logger.Info("value maintenance service started", "interval", "12h")
	for {
		select {
		case <-ticker.C:
			values, err := store.Mind().GetValues(ctx, models.MindQuery{Limit: 1000})
			if err != nil {
				logger.Warn("value maintenance scan error", "error", err)
				continue
			}
			for _, v := range values {
				if !service.CanModifyValue(ctx, v, float64(v.Importance)) {
					logger.Debug("value modification blocked by evidence threshold", "value_id", v.ID, "importance", float64(v.Importance))
				}
			}
		case <-ctx.Done():
			logger.Info("value maintenance service stopped")
			return
		}
	}
}

func bootstrapAdmin(ctx context.Context, store *storage.MongoStore, authService *api.AuthService, cfg *config.Config, logger *slog.Logger) {
	if cfg.Auth.BootstrapAdminEmail == "" || cfg.Auth.BootstrapAdminPassword == "" {
		logger.Warn("no bootstrap admin configured - set auth.bootstrap_admin_email and auth.bootstrap_admin_password")
		return
	}

	_, err := store.Auth().GetUserByEmail(ctx, cfg.Auth.BootstrapAdminEmail)
	if err == nil {
		logger.Info("admin user already exists", "email", cfg.Auth.BootstrapAdminEmail)
		return
	}

	passwordHash, err := authService.HashPassword(cfg.Auth.BootstrapAdminPassword)
	if err != nil {
		logger.Error("failed to hash admin password", "error", err)
		return
	}

	admin := models.User{
		ID:           models.NewID(),
		Email:        cfg.Auth.BootstrapAdminEmail,
		PasswordHash: passwordHash,
		Name:         cfg.Auth.BootstrapAdminEmail,
		IsVerified:   true,
		IsAdmin:      true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := store.Auth().CreateUser(ctx, admin); err != nil {
		logger.Error("failed to create bootstrap admin", "error", err, "email", cfg.Auth.BootstrapAdminEmail)
		return
	}

	logger.Info("bootstrap admin user created", "email", cfg.Auth.BootstrapAdminEmail, "is_admin", true)
}
