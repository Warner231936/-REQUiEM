package config

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type HotReloadConfig struct {
	mu      sync.RWMutex
	inner   *Config
	path    string
	logger  *slog.Logger
	watcher *fsnotify.Watcher
	done    chan struct{}
}

func NewHotReloadConfig(path string, logger *slog.Logger) (*HotReloadConfig, error) {
	if logger == nil {
		logger = slog.Default()
	}

	cfg, err := Load(path)
	if err != nil {
		return nil, fmt.Errorf("initial config load failed: %w", err)
	}

	hrc := &HotReloadConfig{
		inner:  cfg,
		path:   path,
		logger: logger,
		done:   make(chan struct{}),
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		logger.Warn("config file does not exist, hot reload disabled", "path", path)
		return hrc, nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Warn("failed to create file watcher, hot reload disabled", "error", err)
		return hrc, nil
	}

	hrc.watcher = watcher

	if err := watcher.Add(path); err != nil {
		logger.Warn("failed to watch config file", "error", err)
		watcher.Close()
		return hrc, nil
	}

	go hrc.watch()

	logger.Info("config hot reload enabled", "path", path)
	return hrc, nil
}

func (h *HotReloadConfig) watch() {
	debounce := time.NewTimer(0)
	<-debounce.C

	for {
		select {
		case <-h.done:
			return
		case event, ok := <-h.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				debounce.Reset(500 * time.Millisecond)
				select {
				case <-h.done:
					return
				case <-debounce.C:
					h.reload()
				}
			}
		case err, ok := <-h.watcher.Errors:
			if !ok {
				return
			}
			h.logger.Warn("config watcher error", "error", err)
		}
	}
}

func (h *HotReloadConfig) reload() {
	cfg, err := Load(h.path)
	if err != nil {
		h.logger.Error("config reload failed", "error", err)
		return
	}

	h.mu.Lock()
	old := h.inner
	h.inner = cfg
	h.mu.Unlock()

	h.logger.Info("config reloaded successfully",
		"old_port", old.Server.Port,
		"new_port", cfg.Server.Port,
	)
}

func (h *HotReloadConfig) Get() *Config {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.inner
}

func (h *HotReloadConfig) Close() {
	close(h.done)
	if h.watcher != nil {
		h.watcher.Close()
	}
}
