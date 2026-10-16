package modelmgr

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

type IntegrityResult struct {
	Valid        bool
	Sha256       string
	ExpectedSHA  string
	Size         int64
	ExpectedSize int64
	Corrupted    bool
	Error        string
}

type Verifier struct {
	logger *slog.Logger
}

func NewVerifier(logger *slog.Logger) *Verifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &Verifier{logger: logger}
}

func (v *Verifier) Verify(filePath string, expectedSHA string, expectedSize int64) (*IntegrityResult, error) {
	result := &IntegrityResult{
		ExpectedSHA:  expectedSHA,
		ExpectedSize: expectedSize,
	}

	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", filePath)
		}
		return nil, fmt.Errorf("stat file: %w", err)
	}

	result.Size = info.Size()

	if expectedSize > 0 && result.Size != expectedSize {
		result.Corrupted = true
		result.Valid = false
		result.Error = fmt.Sprintf("size mismatch: expected %d, got %d", expectedSize, result.Size)
		v.logger.Warn("size verification failed",
			"file", filePath,
			"expected", expectedSize,
			"actual", result.Size,
		)
		return result, nil
	}

	hash, err := computeSHA256(filePath, v.logger)
	if err != nil {
		return nil, fmt.Errorf("compute sha256: %w", err)
	}

	result.Sha256 = hash

	if expectedSHA != "" {
		if !equalSHA(hash, expectedSHA) {
			result.Corrupted = true
			result.Valid = false
			result.Error = fmt.Sprintf("checksum mismatch: expected %s, got %s", expectedSHA, hash)
			v.logger.Warn("checksum verification failed",
				"file", filePath,
				"expected_sha", expectedSHA,
				"actual_sha", hash,
			)
			return result, nil
		}
	}

	result.Valid = true
	result.Corrupted = false

	v.logger.Debug("integrity verified",
		"file", filePath,
		"sha256", hash,
		"size", result.Size,
	)

	return result, nil
}

func (v *Verifier) VerifyFile(filePath string) (*IntegrityResult, error) {
	result := &IntegrityResult{}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	result.Size = info.Size()

	hash, err := computeSHA256(filePath, v.logger)
	if err != nil {
		return nil, fmt.Errorf("compute sha256: %w", err)
	}

	result.Sha256 = hash
	result.ExpectedSHA = hash
	result.ExpectedSize = result.Size
	result.Valid = true
	result.Corrupted = false

	return result, nil
}

func computeSHA256(filePath string, logger *slog.Logger) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	buf := make([]byte, 1024*1024)
	start := time.Now()
	totalRead := int64(0)

	for {
		n, err := f.Read(buf)
		if n > 0 {
			hasher.Write(buf[:n])
			totalRead += int64(n)

			if logger != nil && totalRead%(10*1024*1024) == 0 {
				logger.Debug("hashing progress",
					"file", filePath,
					"bytes", totalRead,
					"elapsed", time.Since(start),
				)
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func equalSHA(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca := a[i]
		cb := b[i]
		if ca >= 'A' && ca <= 'F' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'F' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
