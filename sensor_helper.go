package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed third_party/sensor-helper/VERSION
var embeddedSensorHelperVersion string

//go:embed third_party/sensor-helper/pulsecore-sensor-helper.exe
var embeddedSensorHelperEXE []byte

const (
	sensorHelperFileName = "pulsecore-sensor-helper.exe"
	sensorHelperCacheTTL = 3 * time.Second
)

type sensorHelperResult struct {
	Version      string  `json:"version"`
	OK           bool    `json:"ok"`
	CPUTempC     float64 `json:"cpuTempC"`
	GPUTempC     float64 `json:"gpuTempC"`
	GPUUtilPct   float64 `json:"gpuUtilPct"`
	VRAMUsedMB   float64 `json:"vramUsedMB"`
	VRAMTotalMB  float64 `json:"vramTotalMB"`
	NeedsAdmin   bool    `json:"needsAdmin"`
	ErrorMessage string  `json:"errorMessage"`
}

var (
	sensorHelperOnce sync.Once
	sensorHelperPath string
	sensorHelperErr  error

	sensorCacheMu  sync.Mutex
	sensorCacheAt  time.Time
	sensorCacheVal sensorHelperResult
	sensorCacheOK  bool

	gpuPollMu    sync.Mutex
	gpuPollAt    time.Time
	gpuPollPct   float64
	gpuPollUsed  float64
	gpuPollTotal float64
	gpuPollHit   bool
)

func sensorHelperVersion() string {
	return strings.TrimSpace(embeddedSensorHelperVersion)
}

func pulseCoreBinDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if strings.TrimSpace(base) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, "AppData", "Local")
	}
	dir := filepath.Join(base, "PulseCore", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// ensureSensorHelper extracts the embedded helper when missing or version-changed.
func ensureSensorHelper() (string, error) {
	sensorHelperOnce.Do(func() {
		dir, err := pulseCoreBinDir()
		if err != nil {
			sensorHelperErr = err
			return
		}
		exePath := filepath.Join(dir, sensorHelperFileName)
		verPath := filepath.Join(dir, "VERSION")
		want := sensorHelperVersion()
		needWrite := true
		if st, err := os.Stat(exePath); err == nil && !st.IsDir() && st.Size() > 0 {
			if b, err := os.ReadFile(verPath); err == nil && strings.TrimSpace(string(b)) == want {
				needWrite = false
			}
		}
		if needWrite {
			if len(embeddedSensorHelperEXE) == 0 {
				sensorHelperErr = fmt.Errorf("embedded sensor helper is empty")
				return
			}
			tmp := exePath + ".tmp"
			if err := os.WriteFile(tmp, embeddedSensorHelperEXE, 0o755); err != nil {
				sensorHelperErr = err
				return
			}
			_ = os.Remove(exePath)
			if err := os.Rename(tmp, exePath); err != nil {
				_ = os.WriteFile(exePath, embeddedSensorHelperEXE, 0o755)
			}
			_ = os.WriteFile(verPath, []byte(want+"\n"), 0o644)
		}
		sensorHelperPath = exePath
	})
	return sensorHelperPath, sensorHelperErr
}

func querySensorHelperCached() (sensorHelperResult, bool) {
	sensorCacheMu.Lock()
	if sensorCacheOK && time.Since(sensorCacheAt) < sensorHelperCacheTTL {
		v := sensorCacheVal
		sensorCacheMu.Unlock()
		return v, true
	}
	sensorCacheMu.Unlock()

	path, err := ensureSensorHelper()
	if err != nil || path == "" {
		return sensorHelperResult{}, false
	}
	out, err := runHiddenOutput(path, "--once")
	if err != nil {
		return sensorHelperResult{}, false
	}
	out = bytes.TrimSpace(out)
	var parsed sensorHelperResult
	if err := json.Unmarshal(out, &parsed); err != nil {
		return sensorHelperResult{}, false
	}
	sensorCacheMu.Lock()
	sensorCacheVal = parsed
	sensorCacheAt = time.Now()
	sensorCacheOK = true
	sensorCacheMu.Unlock()
	return parsed, true
}

// readCPUTemperatureCached prefers the embedded helper, then ACPI, with TTL cache.
// Heavy WMI is never run on every poll tick without reuse.
func readCPUTemperatureCached() (tempC float64, needsAdmin bool) {
	if helper, ok := querySensorHelperCached(); ok {
		if helper.CPUTempC > 0 {
			return helper.CPUTempC, false
		}
		// Do not substitute GPU package temp as CPU - UI labels it as CPU.
		needsAdmin = helper.NeedsAdmin
	}
	acpi := readCPUTemperatureACPI()
	if acpi > 0 {
		return acpi, false
	}
	return 0, needsAdmin || !isProcessElevated()
}

func readGPUFromHelperCached() (pct, usedMB, totalMB, tempC float64, ok bool) {
	helper, hit := querySensorHelperCached()
	if !hit {
		return 0, 0, 0, 0, false
	}
	if helper.GPUUtilPct > 0 || helper.VRAMUsedMB > 0 || helper.GPUTempC > 0 {
		return helper.GPUUtilPct, helper.VRAMUsedMB, helper.VRAMTotalMB, helper.GPUTempC, true
	}
	return 0, 0, 0, 0, false
}

// readNVIDIASMICached runs nvidia-smi at most once per sensorHelperCacheTTL.
func readNVIDIASMICached() (pct, usedMB, totalMB float64, ok bool) {
	gpuPollMu.Lock()
	if gpuPollHit && time.Since(gpuPollAt) < sensorHelperCacheTTL {
		pct, usedMB, totalMB = gpuPollPct, gpuPollUsed, gpuPollTotal
		gpuPollMu.Unlock()
		return pct, usedMB, totalMB, true
	}
	gpuPollMu.Unlock()

	smi := nvidiaSMIPath()
	if smi == "" {
		return 0, 0, 0, false
	}
	out, err := runHiddenOutput(smi,
		"--query-gpu=utilization.gpu,memory.used,memory.total",
		"--format=csv,noheader,nounits")
	if err != nil {
		return 0, 0, 0, false
	}
	line := strings.TrimSpace(string(out))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	parts := strings.Split(line, ",")
	if len(parts) < 3 {
		return 0, 0, 0, false
	}
	fmt.Sscanf(strings.TrimSpace(parts[0]), "%f", &pct)
	fmt.Sscanf(strings.TrimSpace(parts[1]), "%f", &usedMB)
	fmt.Sscanf(strings.TrimSpace(parts[2]), "%f", &totalMB)
	gpuPollMu.Lock()
	gpuPollPct, gpuPollUsed, gpuPollTotal = pct, usedMB, totalMB
	gpuPollAt = time.Now()
	gpuPollHit = true
	gpuPollMu.Unlock()
	return pct, usedMB, totalMB, true
}
