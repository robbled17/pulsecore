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

func killProcessByNameNative(name string) error {
	base := filepath.Base(name)
	if !strings.HasSuffix(strings.ToLower(base), ".exe") {
		base += ".exe"
	}
	return RunHiddenCommand("taskkill.exe", "/F", "/T", "/IM", base).Run()
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

func restartProcessByNameNative(name string) error {
	base := filepath.Base(name)
	if !strings.HasSuffix(strings.ToLower(base), ".exe") {
		base += ".exe"
	}
	procs, err := process.Processes()
	if err != nil {
		return err
	}
	var targetExe string
	var targetArgs []string
	var targetCwd string

	for _, p := range procs {
		pName, err := p.Name()
		if err == nil && strings.EqualFold(filepath.Base(pName), base) {
			if exe, err := p.Exe(); err == nil && exe != "" {
				targetExe = exe
				targetArgs, _ = p.CmdlineSlice()
				targetCwd, _ = p.Cwd()
				break
			}
		}
	}

	if targetExe == "" {
		return fmt.Errorf("process %s not found", name)
	}

	_ = killProcessByNameNative(base)
	time.Sleep(300 * time.Millisecond)

	var cmd *exec.Cmd
	if len(targetArgs) > 1 {
		cmd = exec.Command(targetExe, targetArgs[1:]...)
	} else {
		cmd = exec.Command(targetExe)
	}
	if targetCwd != "" {
		cmd.Dir = targetCwd
	}
	cmd.SysProcAttr = &windows.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | 0x00000008, // DETACHED_PROCESS
	}
	return cmd.Start()
}

// sampleGPUTopProcesses identifies top processes utilizing GPU / VRAM, grouped by name.
func sampleGPUTopProcesses() []HardwareProcessItem {
	procs, err := process.Processes()
	if err != nil {
		return nil
	}

	myPID := int32(os.Getpid())
	type gpuAgg struct {
		pid   int32
		name  string
		count int
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

	byName := map[string]*gpuAgg{}

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
			vram := ram / 2 // estimate dedicated+shared VRAM proportional to RSS
			addScore := score + float64(ram)/(100*1024*1024)
			g := byName[base]
			if g == nil {
				byName[base] = &gpuAgg{
					pid:   p.Pid,
					name:  name,
					count: 1,
					vram:  vram,
					score: addScore,
				}
			} else {
				g.count++
				g.vram += vram
				g.score += addScore * 0.3 // smooth scaling for multi-process apps
			}
		}
	}

	candidates := make([]*gpuAgg, 0, len(byName))
	for _, g := range byName {
		candidates = append(candidates, g)
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
		displayName := c.name
		if c.count > 1 {
			displayName = fmt.Sprintf("%s (%d)", c.name, c.count)
		}
		vramMB := float64(c.vram) / (1024 * 1024)
		var label string
		if vramMB >= 10 {
			label = fmt.Sprintf("%.1f%% GPU · %.0f МБ VRAM", c.score, vramMB)
		} else {
			label = fmt.Sprintf("%.1f%% GPU", c.score)
		}

		out = append(out, HardwareProcessItem{
			PID:        c.pid,
			Name:       displayName,
			ValueLabel: label,
			ValueNum:   c.score,
		})
	}

	return out
}
