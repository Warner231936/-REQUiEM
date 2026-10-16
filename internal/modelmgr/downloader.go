package modelmgr

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DownloadResult struct {
	FilePath   string
	Size       int64
	Sha256     string
	Downloaded bool
	FromCache  bool
}

type ProgressCallback func(progress DownloadProgress)

type DownloadProgress struct {
	Stage      InstallStage
	BytesRead  int64
	TotalBytes int64
	Percent    float64
	Speed      float64
	Elapsed    time.Duration
}

type Downloader struct {
	client     *http.Client
	maxRetries int
	chunkSize  int64
	logger     *slog.Logger
	progressCb ProgressCallback
}

func NewDownloader(maxRetries int, timeout time.Duration, chunkSize int64, logger *slog.Logger) *Downloader {
	if logger == nil {
		logger = slog.Default()
	}
	if chunkSize <= 0 {
		chunkSize = 1024 * 1024
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &Downloader{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		maxRetries: maxRetries,
		chunkSize:  chunkSize,
		logger:     logger,
	}
}

func (d *Downloader) SetProgressCallback(cb ProgressCallback) {
	d.progressCb = cb
}

func (d *Downloader) Download(ctx context.Context, url, destPath string) (*DownloadResult, error) {
	absDest, err := filepath.Abs(destPath)
	if err != nil {
		return nil, fmt.Errorf("resolve destination path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(absDest), 0755); err != nil {
		return nil, fmt.Errorf("create destination dir: %w", err)
	}

	tmpPath := absDest + ".tmp"

	existingSize, err := getPartialSize(tmpPath)
	if err != nil {
		existingSize = 0
	}

	contentLength, supportsResume, serverURL, err := d.headRequest(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("HEAD request failed: %w", err)
	}

	if supportsResume && existingSize > 0 && contentLength > 0 && existingSize >= contentLength {
		return d.finalizeExisting(ctx, tmpPath, absDest, existingSize, url)
	}

	result, err := d.downloadWithRetry(ctx, url, tmpPath, absDest, existingSize, contentLength, supportsResume, serverURL)
	if err != nil {
		_ = cleanupFile(tmpPath)
		return nil, err
	}

	return result, nil
}

func (d *Downloader) downloadWithRetry(ctx context.Context, url, tmpPath, absDest string, existingSize, contentLength int64, supportsResume bool, serverURL string) (*DownloadResult, error) {
	var lastErr error

	for attempt := 0; attempt <= d.maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if attempt > 0 {
			d.logger.Warn("retrying download",
				"attempt", attempt+1,
				"max_retries", d.maxRetries+1,
				"error", lastErr,
			)
			backoff := time.Duration(attempt*attempt) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		result, err := d.downloadAttempt(ctx, serverURL, tmpPath, absDest, existingSize, contentLength, supportsResume)
		if err == nil {
			return result, nil
		}
		lastErr = err

		if isCorruptionError(err) {
			d.logger.Error("download corrupted, not retrying", "error", err)
			_ = cleanupFile(tmpPath)
			return nil, err
		}

		if os.IsNotExist(err) {
			existingSize = 0
		}
	}

	return nil, fmt.Errorf("download failed after %d attempts: %w", d.maxRetries+1, lastErr)
}

func (d *Downloader) downloadAttempt(ctx context.Context, downloadURL, tmpPath, absDest string, existingSize, contentLength int64, supportsResume bool) (*DownloadResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}

	var file *os.File
	var startOffset int64 = 0

	if supportsResume && existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
		startOffset = existingSize

		file, err = os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("open temp file for append: %w", err)
		}
	} else {
		file, err = os.Create(tmpPath)
		if err != nil {
			return nil, fmt.Errorf("create temp file: %w", err)
		}
	}
	defer file.Close()

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		_ = file.Close()
		_ = cleanupFile(tmpPath)
		return d.downloadAttempt(ctx, downloadURL, tmpPath, absDest, 0, contentLength, false)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	actualContentLength := resp.ContentLength
	if actualContentLength > 0 {
		if resp.StatusCode == http.StatusPartialContent {
			contentLength = startOffset + actualContentLength
		}
	}

	progress := DownloadProgress{
		Stage:      StageDownload,
		TotalBytes: contentLength,
	}
	if d.progressCb != nil {
		d.progressCb(progress)
	}

	startTime := time.Now()
	totalBytes := startOffset

	buf := make([]byte, d.chunkSize)
	for {
		if ctx.Err() != nil {
			_ = file.Close()
			_ = cleanupFile(tmpPath)
			return nil, ctx.Err()
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := file.Write(buf[:n]); werr != nil {
				_ = file.Close()
				_ = cleanupFile(tmpPath)
				return nil, fmt.Errorf("write to temp file: %w", werr)
			}
			totalBytes += int64(n)

			progress.BytesRead = totalBytes
			progress.Percent = calcPercent(totalBytes, contentLength)
			progress.Speed = float64(totalBytes-startOffset) / time.Since(startTime).Seconds()
			progress.Elapsed = time.Since(startTime)
			if d.progressCb != nil {
				d.progressCb(progress)
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			_ = file.Close()
			_ = cleanupFile(tmpPath)
			return nil, fmt.Errorf("read from source: %w", err)
		}
	}

	// Always compute hash of entire file after download
	_ = file.Close()
	sha256Sum, err := computeSHA256(tmpPath, d.logger)
	if err != nil {
		_ = cleanupFile(tmpPath)
		return nil, fmt.Errorf("compute hash: %w", err)
	}

	fileInfo, err := os.Stat(tmpPath)
	if err != nil {
		_ = cleanupFile(tmpPath)
		return nil, fmt.Errorf("stat temp file: %w", err)
	}

	downloaded := startOffset == 0 && totalBytes > 0
	return d.finalizeDownload(tmpPath, absDest, fileInfo.Size(), sha256Sum, downloaded)
}

func (d *Downloader) finalizeDownload(tmpPath, absDest string, size int64, sha256Sum string, downloaded bool) (*DownloadResult, error) {
	if err := os.Rename(tmpPath, absDest); err != nil {
		if linkErr, ok := err.(*os.LinkError); ok && linkErr.Op == "rename" {
			if copyErr := copyFile(tmpPath, absDest); copyErr != nil {
				return nil, fmt.Errorf("atomic move failed: %w (copy fallback also failed: %v)", err, copyErr)
			}
			_ = os.Remove(tmpPath)
		} else {
			return nil, fmt.Errorf("atomic move: %w", err)
		}
	}

	actualSize, err := getPartialSize(absDest)
	if err != nil {
		return nil, fmt.Errorf("stat final file: %w", err)
	}

	if actualSize != size {
		return nil, fmt.Errorf("size mismatch: expected %d, got %d", size, actualSize)
	}

	return &DownloadResult{
		FilePath:   absDest,
		Size:       actualSize,
		Sha256:     sha256Sum,
		Downloaded: downloaded,
		FromCache:  !downloaded,
	}, nil
}

func (d *Downloader) finalizeExisting(ctx context.Context, tmpPath, absDest string, size int64, sourceURL string) (*DownloadResult, error) {
	sha256Sum, err := computeSHA256(tmpPath, d.logger)
	if err != nil {
		return nil, fmt.Errorf("compute hash: %w", err)
	}

	return d.finalizeDownload(tmpPath, absDest, size, sha256Sum, false)
}

func (d *Downloader) headRequest(ctx context.Context, urlStr string) (contentLength int64, supportsResume bool, effectiveURL string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, urlStr, nil)
	if err != nil {
		return 0, false, urlStr, fmt.Errorf("create HEAD request: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, false, urlStr, fmt.Errorf("HEAD request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusNotImplemented {
		if resp.StatusCode >= 400 {
			return 0, false, urlStr, fmt.Errorf("HEAD request failed with status %d: %s", resp.StatusCode, resp.Status)
		}
	}

	contentLength = resp.ContentLength
	acceptRanges := resp.Header.Get("Accept-Ranges")
	supportsResume = strings.EqualFold(acceptRanges, "bytes")

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusFound {
		effectiveURL = resp.Request.URL.String()
	} else {
		effectiveURL = urlStr
	}

	if contentLength > 0 {
		_, _, _ = mime.ParseMediaType(resp.Header.Get("Content-Type"))
	}

	return contentLength, supportsResume, effectiveURL, nil
}

func getPartialSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func cleanupFile(path string) error {
	if path == "" {
		return nil
	}
	return os.Remove(path)
}

func calcPercent(current, total int64) float64 {
	if total <= 0 {
		return 0
	}
	if current >= total {
		return 100
	}
	return float64(current) / float64(total) * 100
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func isCorruptionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "corrupt") ||
		strings.Contains(msg, "checksum") ||
		strings.Contains(strings.ToLower(msg), "size mismatch")
}

func validateDownloadURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("unsupported URL scheme: %s (must be http or https)", parsed.Scheme)
	}

	return parsed.String(), nil
}

func sanitizeFilename(name string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, name)

	if safe == "" {
		safe = "model"
	}
	return safe
}
