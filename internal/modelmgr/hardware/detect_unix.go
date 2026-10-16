//go:build !windows

package hardware

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func cpuModelName(ctx context.Context) (string, error) {
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1]), nil
				}
			}
		}
	}

	out, err := exec.CommandContext(ctx, "lscpu").Output()
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "Model name:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1]), nil
				}
			}
		}
	}

	return "unknown", fmt.Errorf("could not detect CPU model")
}

func detectRAM(ctx context.Context) (int, error) {
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					kb, err := strconv.ParseInt(fields[1], 10, 64)
					if err == nil {
						return int(kb / 1024), nil
					}
				}
			}
		}
	}

	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err == nil {
		return int(info.Totalram * uint64(info.Mem_unit) / (1024 * 1024)), nil
	}

	return 0, fmt.Errorf("could not detect RAM")
}

func detectGPUs(ctx context.Context, logger interface{}) ([]GPUInfo, error) {
	var gpus []modelmgr.GPUInfo

	if nvidiaGpus, err := detectNVIDIAGpu(ctx); err == nil && len(nvidiaGpus) > 0 {
		return nvidiaGpus, nil
	}

	if amdGpus, err := detectAmdGpu(ctx); err == nil && len(amdGpus) > 0 {
		return amdGpus, nil
	}

	return gpus, fmt.Errorf("no GPU detected")
}

func detectNVIDIAGpu(ctx context.Context) ([]GPUInfo, error) {
	out, err := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=name,memory.total",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi not available: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var gpus []modelmgr.GPUInfo
	for _, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), ",", 2)
		if len(parts) < 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		vramStr := strings.TrimSpace(parts[1])
		vramMB, err := strconv.Atoi(vramStr)
		if err != nil {
			continue
		}
		gpus = append(gpus, GPUInfo{
			Model:   name,
			VRAMMB:  vramMB,
			Backend: "cuda",
		})
	}
	return gpus, nil
}

func detectAmdGpu(ctx context.Context) ([]modelmgr.GPUInfo, error) {
	out, err := exec.CommandContext(ctx, "rocm-smi", "--showproductname", "--showmeminfo", "vram").Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var gpus []modelmgr.GPUInfo
	var current modelmgr.GPUInfo
	for _, line := range lines {
		if strings.Contains(line, "Card series") || strings.Contains(line, "Card model") || strings.Contains(line, "Card vendor") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				current.Model = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(line, "VRAM Total") || strings.Contains(line, "VRAM Total Memory") {
			parts := strings.Fields(line)
			for _, p := range parts {
				if num, err := strconv.ParseInt(p, 10, 64); err == nil {
					current.VRAMMB = int(num / (1024 * 1024))
					break
				}
			}
			if current.Model != "" {
				current.Backend = "rocm"
				gpus = append(gpus, current)
				current = modelmgr.GPUInfo{}
			}
		}
	}
	return gpus, nil
}

func detectDisk(path string) (int, error) {
	var statfs syscall.Statfs_t
	if err := syscall.Statfs(path, &statfs); err != nil {
		return 0, fmt.Errorf("statfs: %w", err)
	}
	return int(statfs.Bavail * statfs.Bsize / (1024 * 1024)), nil
}

func findExecutable(name string) (string, error) {
	return exec.LookPath(name)
}

func execCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

func wmicQuery(ctx context.Context, query string, timeoutSec int) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	parts := strings.Fields(query)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid query: %s", query)
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", query)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func registryOpen(keyPath, valueName string) (string, error) {
	return "", fmt.Errorf("registry not available on unix")
}

var _ = filepath.Join
