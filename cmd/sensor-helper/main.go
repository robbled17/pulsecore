// PulseCore sensor helper: lightweight Windows sidecar for CPU/GPU temps.
// Built into pulsecore.exe via //go:embed and extracted to
// %LOCALAPPDATA%\PulseCore\bin\ when missing or version-changed.
//
// Usage: pulsecore-sensor-helper.exe [--once]
// Prints one JSON object to stdout and exits.
//
//go:build windows

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
)

const helperVersion = "1.6.2"

type sensorPayload struct {
	Version      string  `json:"version"`
	OK           bool    `json:"ok"`
	CPUTempC     float64 `json:"cpuTempC"`
	GPUTempC     float64 `json:"gpuTempC"`
	GPUUtilPct   float64 `json:"gpuUtilPct"`
	VRAMUsedMB   float64 `json:"vramUsedMB"`
	VRAMTotalMB  float64 `json:"vramTotalMB"`
	NeedsAdmin   bool    `json:"needsAdmin"`
	ErrorMessage string  `json:"errorMessage,omitempty"`
}

func main() {
	_ = ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	defer ole.CoUninitialize()

	out := sensorPayload{Version: helperVersion}
	cpu := readACPITemp()
	gpuTemp, gpuUtil, vramUsed, vramTotal := readNVIDIASMI()
	out.CPUTempC = cpu
	out.GPUTempC = gpuTemp
	out.GPUUtilPct = gpuUtil
	out.VRAMUsedMB = vramUsed
	out.VRAMTotalMB = vramTotal
	out.OK = cpu > 0 || gpuTemp > 0
	if !out.OK {
		out.NeedsAdmin = !isElevated()
		if out.NeedsAdmin {
			out.ErrorMessage = "sensors unavailable; administrator rights may be required"
		} else {
			out.ErrorMessage = "sensors unavailable on this hardware"
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(out)
}

func isElevated() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

func readACPITemp() float64 {
	var maxC float64
	defer func() { _ = recover() }()
	locator, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return 0
	}
	defer locator.Release()
	dl, err := locator.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return 0
	}
	defer dl.Release()
	serviceRaw, err := oleutil.CallMethod(dl, "ConnectServer", nil, `root\wmi`)
	if err != nil {
		return 0
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()
	resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT CurrentTemperature FROM MSAcpi_ThermalZoneTemperature")
	if err != nil {
		return 0
	}
	result := resultRaw.ToIDispatch()
	defer result.Release()
	countVar, _ := oleutil.GetProperty(result, "Count")
	count := int(countVar.Val)
	for i := 0; i < count; i++ {
		itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
		if err != nil {
			continue
		}
		item := itemRaw.ToIDispatch()
		tempVar, _ := oleutil.GetProperty(item, "CurrentTemperature")
		item.Release()
		c := (float64(tempVar.Val) / 10.0) - 273.15
		if c > maxC {
			maxC = c
		}
	}
	if maxC < 10 || maxC > 125 {
		return 0
	}
	return maxC
}

func windowsRoot() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return root
}

func nvidiaSMIPath() string {
	pf := os.Getenv("ProgramFiles")
	candidates := []string{
		filepath.Join(windowsRoot(), "System32", "nvidia-smi.exe"),
	}
	if pf != "" {
		candidates = append([]string{
			filepath.Join(pf, "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe"),
		}, candidates...)
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func readNVIDIASMI() (tempC, utilPct, usedMB, totalMB float64) {
	smi := nvidiaSMIPath()
	if smi == "" {
		return 0, 0, 0, 0
	}
	ctxDone := time.After(4 * time.Second)
	type result struct {
		out []byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		cmd := exec.Command(smi,
			"--query-gpu=temperature.gpu,utilization.gpu,memory.used,memory.total",
			"--format=csv,noheader,nounits")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		out, err := cmd.Output()
		ch <- result{out, err}
	}()
	select {
	case <-ctxDone:
		return 0, 0, 0, 0
	case r := <-ch:
		if r.err != nil {
			return 0, 0, 0, 0
		}
		line := strings.TrimSpace(string(r.out))
		if line == "" {
			return 0, 0, 0, 0
		}
		// Multi-GPU: take first discrete row (first line).
		if i := strings.IndexByte(line, '\n'); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			return 0, 0, 0, 0
		}
		tempC, _ = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		utilPct, _ = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		usedMB, _ = strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
		totalMB, _ = strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
		if tempC < 10 || tempC > 125 {
			tempC = 0
		}
		return tempC, utilPct, usedMB, totalMB
	}
}
