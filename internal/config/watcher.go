package config

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	cfg       *Config
	path      string
	mu        sync.RWMutex
	watcher   *fsnotify.Watcher
	callbacks []func(*Config)
	logger    *slog.Logger
	stopCh    chan struct{}
}

func NewWatcher(path string, logger *slog.Logger) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	// Watch the directory containing the config file
	dir := filepath.Dir(absPath)
	if err := w.Add(dir); err != nil {
		w.Close()
		return nil, fmt.Errorf("watch config dir: %w", err)
	}

	return &Watcher{
		path:      absPath,
		watcher:   w,
		callbacks: make([]func(*Config), 0),
		logger:    logger,
		stopCh:    make(chan struct{}),
	}, nil
}

func (w *Watcher) SetConfig(cfg *Config) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cfg = cfg
}

func (w *Watcher) Config() *Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.cfg
}

func (w *Watcher) OnChange(callback func(*Config)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.callbacks = append(w.callbacks, callback)
}

func (w *Watcher) Start() error {
	go w.watchLoop()
	w.logger.Info("config watcher started", "path", w.path)
	return nil
}

func (w *Watcher) Stop() error {
	close(w.stopCh)
	return w.watcher.Close()
}

func (w *Watcher) watchLoop() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				if filepath.Clean(event.Name) == w.path {
					w.logger.Info("config file changed, reloading", "path", event.Name)
					w.reload()
				}
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.logger.Error("config watcher error", "error", err)
		case <-w.stopCh:
			return
		}
	}
}

func (w *Watcher) reload() {
	// Debounce: wait a bit for the write to complete
	time.Sleep(100 * time.Millisecond)

	cfg, err := Load(w.path)
	if err != nil {
		w.logger.Error("failed to reload config", "error", err)
		return
	}

	w.mu.Lock()
	w.cfg = cfg
	callbacks := make([]func(*Config), len(w.callbacks))
	copy(callbacks, w.callbacks)
	w.mu.Unlock()

	for _, cb := range callbacks {
		cb(cfg)
	}

	w.logger.Info("config reloaded successfully")
}
