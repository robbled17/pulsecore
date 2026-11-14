package main

import (
	"fmt"
	"os"
	"strings"
)

type HardwareProcessItem struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	ValueLabel string  `json:"valueLabel"`
	ValueNum   float64 `json:"valueNum"`
}

type HardwareTopProcesses struct {
	CPU []HardwareProcessItem `json:"cpu"`
	GPU []HardwareProcessItem `json:"gpu"`
	RAM []HardwareProcessItem `json:"ram"`
}

// KillProcess terminates a process by PID.
func (a *App) KillProcess(pid int) error {
	if pid <= 4 {
		return fmt.Errorf("cannot terminate system process")
	}
	return killProcessNative(pid)
}

// KillProcessByName terminates all instances of a process by executable name.
func (a *App) KillProcessByName(name string) error {
	clean := strings.TrimSpace(name)
	if idx := strings.Index(clean, " ("); idx > 0 {
		clean = clean[:idx]
	}
	clean = strings.TrimSpace(clean)
	if clean == "" || isSelfOrWebView(clean) || isCriticalSystemProc(clean) {
		return fmt.Errorf("cannot terminate protected process: %s", clean)
	}
	return killProcessByNameNative(clean)
}

// RestartProcess saves the process executable path and arguments, terminates it, and starts it anew.
func (a *App) RestartProcess(pid int) error {
	if pid <= 4 {
		return fmt.Errorf("cannot restart system process")
	}
	return restartProcessNative(pid)
}

// RestartProcessByName terminates and restarts a process by executable name.
func (a *App) RestartProcessByName(name string) error {
	clean := strings.TrimSpace(name)
	if idx := strings.Index(clean, " ("); idx > 0 {
		clean = clean[:idx]
	}
	clean = strings.TrimSpace(clean)
	if clean == "" || isSelfOrWebView(clean) || isCriticalSystemProc(clean) {
		return fmt.Errorf("cannot restart protected process: %s", clean)
	}
	return restartProcessByNameNative(clean)
}

func formatRAMProcLabel(bytes uint64) string {
	mb := float64(bytes) / (1024 * 1024)
	if mb >= 1024 {
		return fmt.Sprintf("%.1f ГБ RAM", mb/1024)
	}
	return fmt.Sprintf("%.0f МБ RAM", mb)
}

// GetHardwareTopProcesses returns top processes for CPU, GPU, and RAM cards on the Home screen.
func (a *App) GetHardwareTopProcesses() HardwareTopProcesses {
	topCPU, _, topRAM := sampleProcessTops(15, 3, 15)

	out := HardwareTopProcesses{
		CPU: make([]HardwareProcessItem, 0, 5),
		GPU: make([]HardwareProcessItem, 0, 5),
		RAM: make([]HardwareProcessItem, 0, 5),
	}

	myPID := int32(os.Getpid())
	for _, p := range topCPU {
		if p.PID == myPID || isSelfOrWebView(p.Name) {
			continue
		}
		displayName := p.Name
		if len(p.PIDs) > 1 {
			displayName = fmt.Sprintf("%s (%d)", p.Name, len(p.PIDs))
		}
		out.CPU = append(out.CPU, HardwareProcessItem{
			PID:        p.PID,
			Name:       displayName,
			ValueLabel: fmt.Sprintf("%.1f%% CPU", p.CPUPercent),
			ValueNum:   p.CPUPercent,
		})
		if len(out.CPU) >= 5 {
			break
		}
	}

	for _, p := range topRAM {
		if p.PID == myPID || isSelfOrWebView(p.Name) {
			continue
		}
		displayName := p.Name
		if len(p.PIDs) > 1 {
			displayName = fmt.Sprintf("%s (%d)", p.Name, len(p.PIDs))
		}
		out.RAM = append(out.RAM, HardwareProcessItem{
			PID:        p.PID,
			Name:       displayName,
			ValueLabel: formatRAMProcLabel(p.RAMBytes),
			ValueNum:   float64(p.RAMBytes),
		})
		if len(out.RAM) >= 5 {
			break
		}
	}

	out.GPU = sampleGPUTopProcesses()

	return out
}
