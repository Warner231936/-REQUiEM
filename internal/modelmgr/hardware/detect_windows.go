//go:build windows

package hardware

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modkernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = modkernel32.NewProc("GlobalMemoryStatusEx")
	procGetDiskFreeSpaceExW  = modkernel32.NewProc("GetDiskFreeSpaceExW")
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func cpuModelName(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "wmic", "cpu", "get", "Name").Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 1 {
			return strings.TrimSpace(lines[1]), nil
		}
	}

	return registryReadCPUName()
}

func registryReadCPUName() (string, error) {
	var h syscall.Handle
	err := syscall.RegOpenKeyEx(syscall.HKEY_LOCAL_MACHINE,
		syscall.StringToUTF16Ptr("HARDWARE\\DESCRIPTION\\System\\CentralProcessor\\0"),
		0, syscall.KEY_READ, &h)
	if err != nil {
		return "unknown", fmt.Errorf("cannot open registry key: %w", err)
	}
	defer syscall.RegCloseKey(h)

	var buf [256]uint16
	size := uint32(len(buf) * 2)
	err = syscall.RegQueryValueEx(h,
		syscall.StringToUTF16Ptr("PROCESSOR_IDENTIFIER"),
		nil, nil, (*byte)(unsafe.Pointer(&buf[0])), &size)
	if err != nil {
		return "unknown", fmt.Errorf("cannot read registry value: %w", err)
	}
	return syscall.UTF16ToString(buf[:]), nil
}

func detectRAM(ctx context.Context) (int, error) {
	return detectRAMWindows()
}

func detectRAMWindows() (int, error) {
	var mem memoryStatusEx
	mem.Length = uint32(unsafe.Sizeof(mem))

	r1, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&mem)))
	if r1 == 0 {
		return 0, fmt.Errorf("GlobalMemoryStatusEx failed")
	}

	return int(mem.TotalPhys / (1024 * 1024)), nil
}

func detectGPUs(ctx context.Context, logger interface{}) ([]GPUInfo, error) {
	if nvidiaGpus, err := detectNVIDIAGpu(ctx); err == nil && len(nvidiaGpus) > 0 {
		return nvidiaGpus, nil
	}

	return detectWmiGpu(ctx)
}

func detectNVIDIAGpu(ctx context.Context) ([]GPUInfo, error) {
	out, err := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=name,memory.total",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi not available: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var gpus []GPUInfo
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

func detectWmiGpu(ctx context.Context) ([]GPUInfo, error) {
	out, err := exec.CommandContext(ctx, "wmic", "path", "win32_VideoController", "get", "AdapterRAM,Name").Output()
	if err != nil {
		return nil, fmt.Errorf("wmic gpu query failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var gpus []GPUInfo
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		ramStr := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		ramBytes, err := strconv.ParseInt(ramStr, 10, 64)
		if err != nil {
			continue
		}
		vramMB := int(ramBytes / (1024 * 1024))
		gpus = append(gpus, GPUInfo{
			Model:   name,
			VRAMMB:  vramMB,
			Backend: "wddm",
		})
	}
	return gpus, nil
}

func detectDisk(path string) (int, error) {
	var freeBytes uint64
	r1, _, err := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(path))),
		uintptr(unsafe.Pointer(&freeBytes)),
		0, 0,
	)
	if r1 == 0 {
		return 0, fmt.Errorf("GetDiskFreeSpaceEx failed: %v", err)
	}
	return int(freeBytes / (1024 * 1024)), nil
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

	parts := strings.SplitN(query, " ", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid wmic query")
	}

	args := strings.Split(parts[1], " ")
	cmd := exec.CommandContext(ctx, "wmic", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func registryOpen(keyPath, valueName string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

var _ = os.Stat
