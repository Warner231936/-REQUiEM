# Model Manager Bug Report & Fixes

## Critical Bugs Found

### 1. **Manager - checkExistingAcquisition is a No-Op** (manager.go:396-404)
```go
func (m *ModelManager) checkExistingAcquisition(ctx context.Context, manifest *ModelManifest, candidate Candidate) error {
    existing, err := m.registry.Get(manifest.ID)
    if err == nil && existing != nil {
        if existing.IsUsable() && existing.RuntimeName != "" {
            manifest = existing  // BUG: Only changes local pointer!
        }
    }
    return nil
}
```
**Fix**: Return the existing manifest or copy fields properly.

### 2. **Manager - Ollama Port Mismatch** (config.yaml vs candidates)
- Config uses port 11435 (auto-configured)
- Candidates in catalog.go use default Ollama URL (assumes 11434)
- Model manager passes `cfg.Config.OllamaURL` to client but Ollama native pulls don't use this

### 3. **Manager - acquireOllamaNative uses Background Context** (manager.go:260)
```go
func (mb *ollamaBackend) IsAvailable() bool {
    return mb.client.IsRunning(context.Background())  // Should use passed context
}
```

### 4. **Downloader - HEAD Request Doesn't Handle Auth Errors** (downloader.go:306-336)
```go
resp, err := d.client.Do(req)
if err != nil {
    return 0, false, urlStr, nil  // Returns nil error for 401/403!
}
```
**Fix**: Check for 401/403/404 and return proper errors.

### 5. **Catalog - BestForTier Returns Error Instead of Fallback** (catalog.go:70-85)
The fallback logic was removed. If no candidate fits budget, it returns error instead of picking smallest viable model.

### 5. **HealthTester - Hardcoded Test Prompt** (health.go)
```go
testPrompt = "Briefly confirm you are ready."  // Not configurable, may not work for all models
```

### 6. **Bootstrap - Parallel Mode Race Condition** (bootstrap.go:152-207)
Parallel bootstrap calls `acquireForBootstrap` concurrently which registers manifests with same IDs, causing race conditions.

### 7. **Registry - Save Not Fully Thread-Safe** (registry.go:67-97)
`save()` doesn't hold lock during entire operation. `Save()` method locks but `save()` is also called from `Register()` which already holds lock.

### 8. **Importer - Modelfile FROM Path Issue** (importer.go:106)
```go
modelfile := fmt.Sprintf("FROM %s\n%s", ggufPath, strings.Join(filtered, "\n"))
```
Uses absolute path in FROM which may not work in Ollama.

### 9. **Manager - config field type** (manager.go:31 vs 46)
`ModelManager.config` is `Config` but `ModelManagerConfig.Config` is `Config` (value not pointer), causing copy on each access.

### 10. **Manager - ollamaHasModel Uses Background Context** (bootstrap.go:378)
```go
return ob.client.HasModel(context.Background(), name)
```

---

## Fixes Applied

Let me fix the most critical issues now: