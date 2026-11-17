//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/windows"
)

func killProcessNative(pid int) error {
	if pid <= 4 || pid == os.Getpid() {
		return fmt.Errorf("cannot terminate system or own process %d", pid)
	}
	name := processNameByPID(pid)
	if name != "" && (isCriticalSystemProc(name) || isPPLProtected(name) || isSelfOrWebView(name)) {
		return fmt.Errorf("process %s (PID %d) is protected by system security policy", name, pid)
	}

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
	if isCriticalSystemProc(base) || isPPLProtected(base) || isSelfOrWebView(base) {
		return fmt.Errorf("process %s is protected by system security policy", base)
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

var (
	modpdh                           = syscall.NewLazyDLL("pdh.dll")
	procPdhOpenQueryW                = modpdh.NewProc("PdhOpenQueryW")
	procPdhAddEnglishCounterW        = modpdh.NewProc("PdhAddEnglishCounterW")
	procPdhCollectQueryData          = modpdh.NewProc("PdhCollectQueryData")
	procPdhGetFormattedCounterArrayW = modpdh.NewProc("PdhGetFormattedCounterArrayW")
	procPdhCloseQuery                = modpdh.NewProc("PdhCloseQuery")
)

const (
	pdhFmtDouble = 0x00000200
	pdhFmtLarge  = 0x00000400
)

type pdhItemDouble struct {
	szName  *uint16
	cStatus uint32
	padding uint32
	value   float64
}

type pdhItemLarge struct {
	szName  *uint16
	cStatus uint32
	padding uint32
	value   int64
}

type pdhGPUQuery struct {
	mu            sync.Mutex
	hQuery        uintptr
	hEngineCtr    uintptr
	hDedicatedCtr uintptr
	hSharedCtr    uintptr
	initialized   bool
	initFailed    bool
}

var globalPDHGPU pdhGPUQuery

func (q *pdhGPUQuery) initLocked() bool {
	if q.initialized {
		return !q.initFailed
	}
	q.initialized = true

	var hQuery uintptr
	r, _, _ := procPdhOpenQueryW.Call(0, 0, uintptr(unsafe.Pointer(&hQuery)))
	if r != 0 {
		q.initFailed = true
		return false
	}
	q.hQuery = hQuery

	pEngine, _ := syscall.UTF16PtrFromString(`\GPU Engine(*)\Utilization Percentage`)
	pDedicated, _ := syscall.UTF16PtrFromString(`\GPU Adapter Memory(*)\Dedicated Usage`)
	pShared, _ := syscall.UTF16PtrFromString(`\GPU Adapter Memory(*)\Shared Usage`)

	rEng, _, _ := procPdhAddEnglishCounterW.Call(hQuery, uintptr(unsafe.Pointer(pEngine)), 0, uintptr(unsafe.Pointer(&q.hEngineCtr)))
	rDed, _, _ := procPdhAddEnglishCounterW.Call(hQuery, uintptr(unsafe.Pointer(pDedicated)), 0, uintptr(unsafe.Pointer(&q.hDedicatedCtr)))
	rSha, _, _ := procPdhAddEnglishCounterW.Call(hQuery, uintptr(unsafe.Pointer(pShared)), 0, uintptr(unsafe.Pointer(&q.hSharedCtr)))

	if rEng != 0 && rDed != 0 && rSha != 0 {
		procPdhCloseQuery.Call(q.hQuery)
		q.hQuery = 0
		q.initFailed = true
		return false
	}

	// Initial baseline sample
	procPdhCollectQueryData.Call(q.hQuery)
	return true
}

func (q *pdhGPUQuery) sample(dedicatedMB float64) (pct, usedMB, sharedMB, totalDedicatedMB float64, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.initLocked() {
		return 0, 0, 0, dedicatedMB, false
	}

	r, _, _ := procPdhCollectQueryData.Call(q.hQuery)
	if r != 0 {
		return 0, 0, 0, dedicatedMB, false
	}

	totalDedicatedMB = dedicatedMB

	// 1. GPU Engine Utilization (% across 3D/Compute/Video engines)
	if q.hEngineCtr != 0 {
		var bufSize, itemCount uint32
		procPdhGetFormattedCounterArrayW.Call(q.hEngineCtr, pdhFmtDouble, uintptr(unsafe.Pointer(&bufSize)), uintptr(unsafe.Pointer(&itemCount)), 0)
		if bufSize > 0 {
			buf := make([]byte, bufSize)
			rArr, _, _ := procPdhGetFormattedCounterArrayW.Call(q.hEngineCtr, pdhFmtDouble, uintptr(unsafe.Pointer(&bufSize)), uintptr(unsafe.Pointer(&itemCount)), uintptr(unsafe.Pointer(&buf[0])))
			if rArr == 0 && itemCount > 0 {
				items := (*[1 << 20]pdhItemDouble)(unsafe.Pointer(&buf[0]))[:itemCount:itemCount]
				var sumPct float64
				for i := 0; i < int(itemCount); i++ {
					if items[i].cStatus == 0 && items[i].value > 0 {
						sumPct += items[i].value
					}
				}
				if sumPct > 100.0 {
					sumPct = 99.5
				}
				pct = sumPct
			}
		}
	}

	// 2. Dedicated VRAM Usage (bytes -> MB)
	if q.hDedicatedCtr != 0 {
		var bufSize, itemCount uint32
		procPdhGetFormattedCounterArrayW.Call(q.hDedicatedCtr, pdhFmtLarge, uintptr(unsafe.Pointer(&bufSize)), uintptr(unsafe.Pointer(&itemCount)), 0)
		if bufSize > 0 {
			buf := make([]byte, bufSize)
			rArr, _, _ := procPdhGetFormattedCounterArrayW.Call(q.hDedicatedCtr, pdhFmtLarge, uintptr(unsafe.Pointer(&bufSize)), uintptr(unsafe.Pointer(&itemCount)), uintptr(unsafe.Pointer(&buf[0])))
			if rArr == 0 && itemCount > 0 {
				items := (*[1 << 20]pdhItemLarge)(unsafe.Pointer(&buf[0]))[:itemCount:itemCount]
				var totalBytes int64
				for i := 0; i < int(itemCount); i++ {
					if items[i].cStatus == 0 && items[i].value > 0 {
						totalBytes += items[i].value
					}
				}
				usedMB = float64(totalBytes) / (1024 * 1024)
			}
		}
	}

	// 3. Shared VRAM Usage (bytes -> MB)
	if q.hSharedCtr != 0 {
		var bufSize, itemCount uint32
		procPdhGetFormattedCounterArrayW.Call(q.hSharedCtr, pdhFmtLarge, uintptr(unsafe.Pointer(&bufSize)), uintptr(unsafe.Pointer(&itemCount)), 0)
		if bufSize > 0 {
			buf := make([]byte, bufSize)
			rArr, _, _ := procPdhGetFormattedCounterArrayW.Call(q.hSharedCtr, pdhFmtLarge, uintptr(unsafe.Pointer(&bufSize)), uintptr(unsafe.Pointer(&itemCount)), uintptr(unsafe.Pointer(&buf[0])))
			if rArr == 0 && itemCount > 0 {
				items := (*[1 << 20]pdhItemLarge)(unsafe.Pointer(&buf[0]))[:itemCount:itemCount]
				var totalBytes int64
				for i := 0; i < int(itemCount); i++ {
					if items[i].cStatus == 0 && items[i].value > 0 {
						totalBytes += items[i].value
					}
				}
				sharedMB = float64(totalBytes) / (1024 * 1024)
			}
		}
	}

	if totalDedicatedMB <= 0 && usedMB > 0 {
		totalDedicatedMB = usedMB
	}

	return pct, usedMB, sharedMB, totalDedicatedMB, true
}

// sampleOverallGPUUsage queries real Windows PDH system counters for GPU engine utilization
// and VRAM usage on integrated (AMD/Intel) or non-NVIDIA GPUs, with a heuristic fallback.
func sampleOverallGPUUsage(dedicatedMB float64) (pct, usedMB, sharedMB, totalDedicatedMB float64) {
	if p, u, s, t, ok := globalPDHGPU.sample(dedicatedMB); ok {
		return p, u, s, t
	}
	return sampleOverallGPUUsageFallback(dedicatedMB)
}

func sampleOverallGPUUsageFallback(dedicatedMB float64) (pct, usedMB, sharedMB, totalDedicatedMB float64) {
	totalDedicatedMB = dedicatedMB
	procs, err := process.Processes()
	if err != nil {
		return 0, 0, 0, totalDedicatedMB
	}

	myPID := int32(os.Getpid())
	knownGPUPriority := map[string]float64{
		"dwm.exe":            15.0,
		"csrss.exe":          4.0,
		"chrome.exe":         12.0,
		"msedge.exe":         11.5,
		"firefox.exe":        10.0,
		"opera.exe":          10.0,
		"discord.exe":        8.0,
		"telegram.exe":       5.0,
		"spotify.exe":        4.0,
		"steam.exe":          6.0,
		"steamwebhelper.exe": 7.0,
		"obs64.exe":          25.0,
		"blender.exe":        40.0,
		"photoshop.exe":      30.0,
		"premiere.exe":       45.0,
	}

	var sumScore float64
	var sumVRAMBytes uint64

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
		if score == 0 {
			if strings.Contains(base, "game") || strings.Contains(base, "client") || strings.Contains(base, "vulkan") || strings.Contains(base, "d3d") {
				score = 20.0
			} else if ram > 500*1024*1024 {
				score = 5.0
			}
		}

		if score > 0 {
			vram := ram / 2
			sumVRAMBytes += vram
			sumScore += score * 0.4
		}
	}

	pct = sumScore
	if pct > 100.0 {
		pct = 99.5
	}
	usedMB = float64(sumVRAMBytes) / (1024 * 1024)
	if usedMB > totalDedicatedMB && totalDedicatedMB > 0 {
		sharedMB = usedMB - totalDedicatedMB
	}
	return pct, usedMB, sharedMB, totalDedicatedMB
}
