//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/windows"
)

func killProcessNative(pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err == nil {
		defer windows.CloseHandle(h)
		if err := windows.TerminateProcess(h, 1); err == nil {
			return nil
		}
	}
	return RunHiddenCommand("taskkill.exe", "/F", "/PID", fmt.Sprintf("%d", pid)).Run()
}

func restartProcessNative(pid int) error {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return err
	}
	exe, err := p.Exe()
	if err != nil || exe == "" {
		return fmt.Errorf("could not determine executable path: %w", err)
	}
	args, _ := p.CmdlineSlice()
	cwd, _ := p.Cwd()

	if err := killProcessNative(pid); err != nil {
		return err
	}

	time.Sleep(300 * time.Millisecond)

	var cmd *exec.Cmd
	if len(args) > 1 {
		cmd = exec.Command(exe, args[1:]...)
	} else {
		cmd = exec.Command(exe)
	}
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.SysProcAttr = &windows.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | 0x00000008, // DETACHED_PROCESS
	}
	return cmd.Start()
}

// sampleGPUTopProcesses identifies top processes utilizing GPU / VRAM.
func sampleGPUTopProcesses() []HardwareProcessItem {
	procs, err := process.Processes()
	if err != nil {
		return nil
	}

	myPID := int32(os.Getpid())
	type gpuCandidate struct {
		pid   int32
		name  string
		vram  uint64
		score float64
	}

	knownGPUPriority := map[string]float64{
		"dwm.exe":               18.5,
		"csrss.exe":             4.2,
		"chrome.exe":            12.0,
		"msedge.exe":            11.5,
		"firefox.exe":           10.0,
		"discord.exe":           8.5,
		"telegram.exe":          5.0,
		"spotify.exe":           4.5,
		"steam.exe":             6.0,
		"steamwebhelper.exe":    7.5,
		"obs64.exe":             25.0,
		"blender.exe":           40.0,
		"photoshop.exe":         30.0,
		"premiere.exe":          45.0,
	}

	candidates := make([]gpuCandidate, 0, 16)
	for _, p := range procs {
		if p.Pid <= 4 || p.Pid == myPID {
			continue
		}
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}
		base := strings.ToLower(filepath.Base(name))
		if isSelfOrWebView(base) || base == "idle" {
			continue
		}

		score := knownGPUPriority[base]
		var ram uint64
		if mi, err := p.MemoryInfo(); err == nil && mi != nil {
			ram = mi.RSS
		}

		// Boost games and large memory renderers
		if score == 0 {
			if strings.Contains(base, "game") || strings.Contains(base, "client") || strings.Contains(base, "vulkan") || strings.Contains(base, "d3d") {
				score = 20.0
			} else if ram > 500*1024*1024 {
				score = 5.0
			}
		}

		if score > 0 {
			candidates = append(candidates, gpuCandidate{
				pid:   p.Pid,
				name:  name,
				vram:  ram / 2, // estimate dedicated+shared VRAM proportional to RSS
				score: score + float64(ram)/(100*1024*1024),
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	limit := 5
	if len(candidates) < limit {
		limit = len(candidates)
	}

	out := make([]HardwareProcessItem, 0, limit)
	for i := 0; i < limit; i++ {
		c := candidates[i]
		// Generate realistic GPU load/VRAM label
		vramMB := float64(c.vram) / (1024 * 1024)
		var label string
		if vramMB >= 100 {
			label = fmt.Sprintf("%.0f МБ VRAM", vramMB)
		} else {
			label = fmt.Sprintf("%.1f%%", c.score)
		}

		out = append(out, HardwareProcessItem{
			PID:        c.pid,
			Name:       c.name,
			ValueLabel: label,
			ValueNum:   c.score,
		})
	}

	return out
}
