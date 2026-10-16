package modelmgr

import (
	"errors"
	"fmt"
)

type ManagerError struct {
	Stage   string
	ModelID string
	Reason  string
	Err     error
}

func (e *ManagerError) Error() string {
	msg := fmt.Sprintf("MODEL_MANAGER_ERROR: stage=%s", e.Stage)
	if e.ModelID != "" {
		msg += fmt.Sprintf(" model=%s", e.ModelID)
	}
	if e.Reason != "" {
		msg += fmt.Sprintf(" stage_detail=%s", e.Reason)
	}
	if e.Err != nil {
		msg += fmt.Sprintf(" cause=%v", e.Err)
	}
	return msg
}

func (e *ManagerError) Unwrap() error {
	return e.Err
}

func NewError(stage string, modelID string, reason string, err error) *ManagerError {
	return &ManagerError{
		Stage:   stage,
		ModelID: modelID,
		Reason:  reason,
		Err:     err,
	}
}

var (
	ErrModelNotFound      = errors.New("model not found in registry or catalog")
	ErrModelAlreadyActive = errors.New("model is already the active model")
	ErrInsufficientVRAM   = errors.New("insufficient VRAM for selected model")
	ErrInsufficientDisk   = errors.New("insufficient disk space")
	ErrCorruptManifest    = errors.New("manifest is corrupt or invalid")
	ErrChecksumMismatch   = errors.New("checksum verification failed")
	ErrSizeMismatch       = errors.New("file size verification failed")
	ErrRuntimeUnavailable = errors.New("runtime backend unavailable")
	ErrImportFailed       = errors.New("model import into runtime failed")
	ErrHealthCheckFailed  = errors.New("model health check failed")
	ErrNoSuitableModel    = errors.New("no suitable model found for criteria")
	ErrPathTraversal      = errors.New("path traversal detected")
	ErrInvalidModelID     = errors.New("invalid model identifier")
	ErrOllamaNotRunning   = errors.New("ollama is not running")
)
