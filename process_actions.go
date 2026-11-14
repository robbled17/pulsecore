package main

import (
	"fmt"
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

// RestartProcess saves the process executable path and arguments, terminates it, and starts it anew.
func (a *App) RestartProcess(pid int) error {
	if pid <= 4 {
		return fmt.Errorf("cannot restart system process")
	}
	return restartProcessNative(pid)
}

// GetHardwareTopProcesses returns top processes for CPU, GPU, and RAM cards on the Home screen.
func (a *App) GetHardwareTopProcesses() HardwareTopProcesses {
	topCPU, _, topRAM := sampleProcessTops(5, 3, 5)

	out := HardwareTopProcesses{
		CPU: make([]HardwareProcessItem, 0, len(topCPU)),
		GPU: make([]HardwareProcessItem, 0, 5),
		RAM: make([]HardwareProcessItem, 0, len(topRAM)),
	}

	for _, p := range topCPU {
		out.CPU = append(out.CPU, HardwareProcessItem{
			PID:        p.PID,
			Name:       p.Name,
			ValueLabel: fmt.Sprintf("%.1f%%", p.CPUPercent),
			ValueNum:   p.CPUPercent,
		})
	}

	for _, p := range topRAM {
		out.RAM = append(out.RAM, HardwareProcessItem{
			PID:        p.PID,
			Name:       p.Name,
			ValueLabel: formatSizeLabel(int64(p.RAMBytes)),
			ValueNum:   float64(p.RAMBytes),
		})
	}

	out.GPU = sampleGPUTopProcesses()

	return out
}
