package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	defaultPollMS   = 1000
	mainWindowW     = 1020
	mainWindowH     = 680
	miniHUDW        = 280
	miniHUDH        = 56
	gameOverlayW    = 340
	gameOverlayH    = 64
	mainMinW        = 880
	mainMinH        = 560
	lagSpikeCore    = 95.0
	lagSpikeOverall = 70.0
	vampireCPUPct   = 4.0
	thermalLimitC   = 88.0
	thermalHoldSec  = 15
	maxLagEvents    = 5
)

var (
	modUser32                 = windows.NewLazySystemDLL("user32.dll")
	procMessageBeep           = modUser32.NewProc("MessageBeep")
	procGetForegroundWindow   = modUser32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcId = modUser32.NewProc("GetWindowThreadProcessId")
	modKernel32 = windows.NewLazySystemDLL("kernel32.dll")
)

// App is the PulseCore application backend.
type App struct {
	ctx context.Context

	mu           sync.RWMutex
	metrics      SystemMetrics
	lagEvents     []LagEvent
	stutterEvents []StutterEvent
	vampires      []VampireProcess
	pollInterval  time.Duration
	miniHUD       bool
	gameOverlay   bool
	hidden        bool
	powerMode     string
	language      string
	periphFP      string
	cpuName       string
	gpuName       string
	gpuTotalVRAM  uint64
	staticHW      staticHardware

	prevDiskIO   map[string]disk.IOCountersStat
	prevDiskTime time.Time
	prevCorePct  []float64
	lastLagAt    map[int]time.Time
	hotSince     time.Time
	thermalArmed bool
	stopWorker   chan struct{}
	workerOnce   sync.Once
	flightOnce   sync.Once
	ecoPIDs      map[int32]bool
}

// SystemMetrics is a full snapshot of live hardware state.
type SystemMetrics struct {
	Timestamp      string             `json:"timestamp"`
	CPUName        string             `json:"cpuName"`
	CPUPercent     float64            `json:"cpuPercent"`
	CPUPerCore     []float64          `json:"cpuPerCore"`
	GPUName        string             `json:"gpuName"`
	GPUPercent     float64            `json:"gpuPercent"`
	VRAMUsedMB     float64            `json:"vramUsedMB"`
	VRAMTotalMB    float64            `json:"vramTotalMB"`
	VRAMSharedMB   float64            `json:"vramSharedMB"`
	VRAMDedicatedMB float64           `json:"vramDedicatedMB"`
	RAMPercent     float64            `json:"ramPercent"`
	RAMUsedMB      uint64             `json:"ramUsedMB"`
	RAMTotalMB     uint64             `json:"ramTotalMB"`
	SwapPercent    float64            `json:"swapPercent"`
	SwapUsedMB     uint64             `json:"swapUsedMB"`
	SwapTotalMB    uint64             `json:"swapTotalMB"`
	Disks          []DiskMetrics      `json:"disks"`
	Battery        BatteryMetrics     `json:"battery"`
	Hardware       HardwareInfo       `json:"hardware"`
	TemperatureC   float64            `json:"temperatureC"`
	TempAvailable  bool               `json:"tempAvailable"`
	GPUAvailable   bool               `json:"gpuAvailable"`
	PowerMode      string             `json:"powerMode"`
	PollIntervalMS int                `json:"pollIntervalMS"`
	MiniHUD        bool               `json:"miniHUD"`
	Language       string             `json:"language"`
	LagEvents      []LagEvent         `json:"lagEvents"`
	StutterEvents  []StutterEvent     `json:"stutterEvents"`
	Vampires       []VampireProcess   `json:"vampires"`
}

// DiskMetrics describes a mounted volume and its IO throughput.
type DiskMetrics struct {
	Mount      string  `json:"mount"`
	UsedGB     float64 `json:"usedGB"`
	TotalGB    float64 `json:"totalGB"`
	Percent    float64 `json:"percent"`
	ReadMBps   float64 `json:"readMBps"`
	WriteMBps  float64 `json:"writeMBps"`
	FreeGB     float64 `json:"freeGB"`
	Removable  bool    `json:"removable,omitempty"`
}

// BatteryMetrics describes AC/DC power state.
type BatteryMetrics struct {
	Percent           float64 `json:"percent"`
	PluggedIn         bool    `json:"pluggedIn"`
	Status            string  `json:"status"`
	StatusText        string  `json:"statusText"`
	PowerDrawWatts    float64 `json:"powerDrawWatts"`
	Available         bool    `json:"available"`
	WearPercent       float64 `json:"wearPercent"`
	WearLabel         string  `json:"wearLabel"`
	CapacitymWh       float64 `json:"capacitymWh"`
	DesignCapacitymWh float64 `json:"designCapacitymWh"`
	DeviceName        string  `json:"deviceName"`
}

// LagEvent is a detected micro-freeze / core spike incident.
type LagEvent struct {
	Timestamp       string  `json:"timestamp"`
	CoreID          int     `json:"coreID"`
	CoreLoad        float64 `json:"coreLoad"`
	OverallLoad     float64 `json:"overallLoad"`
	CulpritProcess  string  `json:"culpritProcess"`
	CulpritFriendly string  `json:"culpritFriendly"`
	CulpritPID      int32   `json:"culpritPID"`
	Explanation     string  `json:"explanation"`
	Icon            string  `json:"icon"`
}

// VampireProcess is a background resource hog (may represent a grouped set).
type VampireProcess struct {
	PID          int32   `json:"pid"`
	PIDs         []int32 `json:"pids"`
	Count        int     `json:"count"`
	Name         string  `json:"name"`
	FriendlyName string  `json:"friendlyName"`
	CPUPercent   float64 `json:"cpuPercent"`
	DiskMBps     float64 `json:"diskMBps"`
	Score        float64 `json:"score"`
	EcoMode      bool    `json:"ecoMode"`
	Frozen       bool    `json:"frozen"` // alias of EcoMode for UI compatibility
}

// NewApp constructs the application shell.
func NewApp() *App {
	return &App{
		pollInterval: time.Duration(defaultPollMS) * time.Millisecond,
		powerMode:    "balanced",
		language:     "ru",
		prevDiskIO:   make(map[string]disk.IOCountersStat),
		lastLagAt:    make(map[int]time.Time),
		lagEvents:     make([]LagEvent, 0, maxLagEvents),
		stutterEvents: make([]StutterEvent, 0, maxStutterEvents),
		vampires:      make([]VampireProcess, 0),
		stopWorker:    make(chan struct{}),
		ecoPIDs:       make(map[int32]bool),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.staticHW = collectStaticHardware()
	a.cpuName = a.staticHW.cpuName
	a.gpuName = a.staticHW.gpuName
	a.gpuTotalVRAM = a.staticHW.gpuVRAM
	a.workerOnce.Do(func() {
		go a.metricsWorker()
	})
	a.flightOnce.Do(func() {
		a.startFlightRecorder()
	})
	go a.peripheralsWatchdog()
}

func (a *App) shutdown(ctx context.Context) {
	select {
	case <-a.stopWorker:
	default:
		close(a.stopWorker)
	}
}

func (a *App) metricsWorker() {
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	a.collectOnce()

	for {
		select {
		case <-a.stopWorker:
			return
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.mu.RLock()
			interval := a.pollInterval
			a.mu.RUnlock()
			if interval != 0 && ticker != nil {
				ticker.Reset(interval)
			}
			a.collectOnce()
		}
	}
}

func (a *App) collectOnce() {
	defer func() {
		if r := recover(); r != nil {
			// Keep UI alive if a Windows sensor path panics.
			fmt.Printf("collectOnce recovered: %v\n", r)
		}
	}()
	snapshot := a.buildSnapshot()
	a.LagSpotterCheck(snapshot.CPUPerCore, snapshot.CPUPercent)
	vampires := a.GetVampireProcesses()
	realTemp := snapshot.TemperatureC
	if realTemp <= 0 {
		// Do not invent a fake ACPI reading: UI shows Н/Д when TempAvailable=false.
		snapshot.TemperatureC = 0
		snapshot.TempAvailable = false
	} else {
		snapshot.TempAvailable = true
		a.TriggerThermalCheck(realTemp)
	}
	gpuName := strings.TrimSpace(snapshot.GPUName)
	snapshot.GPUAvailable = gpuName != "" && !strings.EqualFold(gpuName, "Unknown GPU")

	a.mu.Lock()
	a.metrics = snapshot
	a.vampires = vampires
	a.metrics.LagEvents = append([]LagEvent(nil), a.lagEvents...)
	a.metrics.StutterEvents = append([]StutterEvent(nil), a.stutterEvents...)
	a.metrics.Vampires = append([]VampireProcess(nil), a.vampires...)
	a.metrics.PowerMode = a.powerMode
	a.metrics.PollIntervalMS = int(a.pollInterval / time.Millisecond)
	a.metrics.MiniHUD = a.miniHUD
	a.metrics.Language = a.language
	a.mu.Unlock()

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "metrics:update", a.GetMetrics())
	}
}

func (a *App) buildSnapshot() SystemMetrics {
	now := time.Now()

	overall, _ := cpu.Percent(0, false)
	perCore, _ := cpu.Percent(0, true)
	cpuPct := 0.0
	if len(overall) > 0 {
		cpuPct = overall[0]
	}

	vm, _ := mem.VirtualMemory()
	sm, _ := mem.SwapMemory()

	disks := a.collectDisks(now)
	bat := collectBattery()
	gpuPct, vramUsed, vramShared, vramDedicated := a.collectGPUUsage()

	temp := readCPUTemperature()

	a.mu.RLock()
	cpuName := a.cpuName
	gpuName := a.gpuName
	staticHW := a.staticHW
	vramTotal := float64(a.gpuTotalVRAM) / (1024 * 1024)
	if vramTotal <= 0 && vramDedicated > 0 {
		vramTotal = vramDedicated
	}
	powerMode := a.powerMode
	pollMS := int(a.pollInterval / time.Millisecond)
	mini := a.miniHUD
	lang := a.language
	lags := append([]LagEvent(nil), a.lagEvents...)
	stutters := append([]StutterEvent(nil), a.stutterEvents...)
	vamps := append([]VampireProcess(nil), a.vampires...)
	a.mu.RUnlock()

	hw := buildHardwareInfo(staticHW, cpuPct, perCore, gpuPct, vramUsed, vramShared, vramDedicated, vm, bat)

	return SystemMetrics{
		Timestamp:       now.Format(time.RFC3339),
		CPUName:         cpuName,
		CPUPercent:      cpuPct,
		CPUPerCore:      perCore,
		GPUName:         gpuName,
		GPUPercent:      gpuPct,
		VRAMUsedMB:      vramUsed,
		VRAMTotalMB:     vramTotal,
		VRAMSharedMB:    vramShared,
		VRAMDedicatedMB: vramDedicated,
		RAMPercent:      vm.UsedPercent,
		RAMUsedMB:       vm.Used / (1024 * 1024),
		RAMTotalMB:      vm.Total / (1024 * 1024),
		SwapPercent:     sm.UsedPercent,
		SwapUsedMB:      sm.Used / (1024 * 1024),
		SwapTotalMB:     sm.Total / (1024 * 1024),
		Disks:           disks,
		Battery:         bat,
		Hardware:        hw,
		TemperatureC:    temp,
		PowerMode:       powerMode,
		PollIntervalMS:  pollMS,
		MiniHUD:         mini,
		Language:        lang,
		LagEvents:       lags,
		StutterEvents:   stutters,
		Vampires:        vamps,
	}
}

// GetMetrics returns the latest cached metrics snapshot.
func (a *App) GetMetrics() SystemMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := a.metrics
	out.LagEvents = append([]LagEvent(nil), a.lagEvents...)
	out.StutterEvents = append([]StutterEvent(nil), a.stutterEvents...)
	out.Vampires = append([]VampireProcess(nil), a.vampires...)
	out.CPUPerCore = append([]float64(nil), a.metrics.CPUPerCore...)
	out.Disks = append([]DiskMetrics(nil), a.metrics.Disks...)
	if hw := out.Hardware; hw.CPU.PerCore != nil {
		out.Hardware.CPU.PerCore = append([]float64(nil), hw.CPU.PerCore...)
	}
	out.PowerMode = a.powerMode
	out.PollIntervalMS = int(a.pollInterval / time.Millisecond)
	out.MiniHUD = a.miniHUD
	out.Language = a.language
	return out
}

// SetPollInterval updates the background sampling cadence (ms).
func (a *App) SetPollInterval(ms int) {
	if ms < 500 {
		ms = 500
	}
	if ms > 5000 {
		ms = 5000
	}
	a.mu.Lock()
	a.pollInterval = time.Duration(ms) * time.Millisecond
	a.mu.Unlock()
}

// SetLanguage switches UI language code.
func (a *App) SetLanguage(lang string) {
	lang = strings.ToLower(strings.TrimSpace(lang))
	switch lang {
	case "ru", "en", "kk", "tt", "de", "ja", "zh":
	default:
		lang = "ru"
	}
	a.mu.Lock()
	a.language = lang
	a.mu.Unlock()
}

// GetLanguage returns the active language code.
func (a *App) GetLanguage() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.language
}

// LagSpotterCheck detects per-core spikes against a calm overall load.
func (a *App) LagSpotterCheck(cores []float64, overall float64) []LagEvent {
	if len(cores) == 0 {
		a.mu.RLock()
		defer a.mu.RUnlock()
		return append([]LagEvent(nil), a.lagEvents...)
	}

	culpritName, culpritPID := findHottestProcess()
	now := time.Now().Format("15:04:05")

	a.mu.Lock()
	defer a.mu.Unlock()

	for i, load := range cores {
		spiked := load >= lagSpikeCore && overall < lagSpikeOverall
		if !spiked && len(a.prevCorePct) == len(cores) {
			delta := load - a.prevCorePct[i]
			if delta >= 40 && load >= 90 && overall < 75 {
				spiked = true
			}
		}
		if !spiked {
			continue
		}
		if last, ok := a.lastLagAt[i]; ok && time.Since(last) < 3*time.Second {
			continue
		}
		// Never attribute peak-core lags to AV / critical OS (no Eco Mode banner).
		if culpritName != "" && isNoiseBackgroundProcess(culpritName) {
			continue
		}
		a.lastLagAt[i] = time.Now()
		friendly := friendlyProcessName(culpritName)
		ev := LagEvent{
			Timestamp:       now,
			CoreID:          i,
			CoreLoad:        load,
			OverallLoad:     overall,
			CulpritProcess:  culpritName,
			CulpritFriendly: friendly,
			CulpritPID:      culpritPID,
			Icon:            processIcon(culpritName),
			Explanation:     fmt.Sprintf("lag_core_spike|%d|%.0f", i, load),
		}
		a.lagEvents = append([]LagEvent{ev}, a.lagEvents...)
		if len(a.lagEvents) > maxLagEvents {
			a.lagEvents = a.lagEvents[:maxLagEvents]
		}
	}
	a.prevCorePct = append([]float64(nil), cores...)
	return append([]LagEvent(nil), a.lagEvents...)
}

// GetLagEvents returns the recent micro-freeze log.
func (a *App) GetLagEvents() []LagEvent {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]LagEvent(nil), a.lagEvents...)
}

// GetVampireProcesses finds background CPU/disk hogs that are not foreground.
// Identical executables are merged into one row with summed metrics.
func (a *App) GetVampireProcesses() []VampireProcess {
	fgPID := foregroundPID()
	procs, err := process.Processes()
	if err != nil {
		return []VampireProcess{}
	}

	type agg struct {
		name   string
		pids   []int32
		cpu    float64
		disk   float64
		ecoAll bool
	}
	byName := map[string]*agg{}

	for _, p := range procs {
		if int32(p.Pid) == fgPID || p.Pid == 0 {
			continue
		}
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}
		lower := strings.ToLower(filepath.Base(name))
		if isSelfOrWebView(lower) || lower == "system" || lower == "idle" {
			continue
		}
		if isNoiseBackgroundProcess(name) || isNoiseBackgroundProcess(lower) {
			continue
		}

		cpuPct, err := p.CPUPercent()
		if err != nil {
			continue
		}
		io, _ := p.IOCounters()
		diskMBps := 0.0
		if io != nil {
			diskMBps = float64(io.ReadBytes+io.WriteBytes) / (1024 * 1024) / 60.0
			if diskMBps > 500 {
				diskMBps = 500
			}
		}
		if cpuPct < vampireCPUPct && diskMBps < 2.0 {
			continue
		}

		a.mu.RLock()
		ecoOn := a.ecoPIDs[p.Pid]
		a.mu.RUnlock()

		g := byName[lower]
		if g == nil {
			g = &agg{name: name, ecoAll: true}
			byName[lower] = g
		}
		g.pids = append(g.pids, p.Pid)
		g.cpu += cpuPct
		g.disk += diskMBps
		if !ecoOn {
			g.ecoAll = false
		}
	}

	list := make([]VampireProcess, 0, len(byName))
	for _, g := range byName {
		score := g.cpu*1.4 + g.disk*0.6
		pid := int32(0)
		if len(g.pids) > 0 {
			pid = g.pids[0]
		}
		list = append(list, VampireProcess{
			PID:          pid,
			PIDs:         append([]int32(nil), g.pids...),
			Count:        len(g.pids),
			Name:         g.name,
			FriendlyName: friendlyProcessName(g.name),
			CPUPercent:   g.cpu,
			DiskMBps:     g.disk,
			Score:        score,
			EcoMode:      g.ecoAll,
			Frozen:       g.ecoAll,
		})
	}

	sort.Slice(list, func(i, j int) bool { return list[i].Score > list[j].Score })
	limit := 12
	if len(list) < limit {
		limit = len(list)
	}
	out := list[:limit]

	a.mu.Lock()
	a.vampires = append([]VampireProcess(nil), out...)
	a.mu.Unlock()
	return out
}

// SetPowerProfile switches Silent / Balanced / Full Throttle via PowrProf.dll.
func (a *App) SetPowerProfile(mode string) error {
	normalized, err := setPowerProfileNative(mode)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.powerMode = normalized
	a.mu.Unlock()
	return nil
}

// TriggerThermalCheck fires a native alert when temperature stays hot.
func (a *App) TriggerThermalCheck(temp float64) {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()

	if temp > thermalLimitC {
		if a.hotSince.IsZero() {
			a.hotSince = now
		}
		if now.Sub(a.hotSince) >= time.Duration(thermalHoldSec)*time.Second && !a.thermalArmed {
			a.thermalArmed = true
			go a.emitThermalAlarm(temp)
		}
		return
	}
	a.hotSince = time.Time{}
	a.thermalArmed = false
}

func (a *App) emitThermalAlarm(temp float64) {
	// Native beep only: no hidden PowerShell toast hosts (AV/SmartScreen hygiene).
	_, _, _ = procMessageBeep.Call(0x00000030) // MB_ICONEXCLAMATION
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "thermal:alarm", map[string]interface{}{
			"temperature": temp,
			"limit":       thermalLimitC,
			"message": fmt.Sprintf(
				"CPU temperature %.1f°C exceeded %.0f°C for %ds. Consider Silent mode.",
				temp, thermalLimitC, thermalHoldSec,
			),
		})
	}
}

// ToggleMiniHUD switches between main window and floating compact overlay.
func (a *App) ToggleMiniHUD(enable bool) {
	a.mu.Lock()
	a.miniHUD = enable
	if enable {
		a.gameOverlay = false
	}
	a.mu.Unlock()

	if a.ctx == nil {
		return
	}

	if enable {
		runtime.WindowUnmaximise(a.ctx)
		runtime.WindowSetMinSize(a.ctx, miniHUDW, miniHUDH)
		runtime.WindowSetMaxSize(a.ctx, miniHUDW, miniHUDH)
		runtime.WindowSetSize(a.ctx, miniHUDW, miniHUDH)
		runtime.WindowSetAlwaysOnTop(a.ctx, true)
		a.positionMiniHUD()
		runtime.WindowShow(a.ctx)
		runtime.EventsEmit(a.ctx, "ui:minihud", true)
		runtime.EventsEmit(a.ctx, "ui:gameoverlay", false)
		return
	}

	a.restoreMainWindow()
	runtime.EventsEmit(a.ctx, "ui:minihud", false)
}

// ToggleGameOverlay switches a compact always-on-top FPS / frametime strip (F12).
func (a *App) ToggleGameOverlay(enable bool) {
	a.mu.Lock()
	a.gameOverlay = enable
	if enable {
		a.miniHUD = false
	}
	a.mu.Unlock()

	if a.ctx == nil {
		return
	}

	if enable {
		runtime.WindowUnmaximise(a.ctx)
		runtime.WindowSetMinSize(a.ctx, gameOverlayW, gameOverlayH)
		runtime.WindowSetMaxSize(a.ctx, gameOverlayW, gameOverlayH)
		runtime.WindowSetSize(a.ctx, gameOverlayW, gameOverlayH)
		runtime.WindowSetAlwaysOnTop(a.ctx, true)
		a.positionGameOverlay()
		runtime.WindowShow(a.ctx)
		runtime.EventsEmit(a.ctx, "ui:gameoverlay", true)
		runtime.EventsEmit(a.ctx, "ui:minihud", false)
		return
	}

	a.restoreMainWindow()
	runtime.EventsEmit(a.ctx, "ui:gameoverlay", false)
}

func (a *App) restoreMainWindow() {
	runtime.WindowSetMaxSize(a.ctx, 0, 0)
	runtime.WindowSetMinSize(a.ctx, mainMinW, mainMinH)
	runtime.WindowSetSize(a.ctx, mainWindowW, mainWindowH)
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
	a.centerMainWindow()
	runtime.WindowShow(a.ctx)
}

func (a *App) positionGameOverlay() {
	w := 1920
	if screens, err := runtime.ScreenGetAll(a.ctx); err == nil {
		for _, s := range screens {
			if s.IsPrimary || s.IsCurrent {
				if s.Size.Width > 0 {
					w = s.Size.Width
				} else if s.Width > 0 {
					w = s.Width
				}
				break
			}
		}
	}
	x := (w - gameOverlayW) / 2
	if x < 0 {
		x = 18
	}
	runtime.WindowSetPosition(a.ctx, x, 18)
}

// peripheralsWatchdog polls PnP inventory every ~2.5s and emits changes.
func (a *App) peripheralsWatchdog() {
	ticker := time.NewTicker(2500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-a.stopWorker:
			return
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			items := a.collectConnectedDevices()
			fp := peripheralsFingerprint(items)
			a.mu.Lock()
			changed := fp != a.periphFP
			a.periphFP = fp
			a.mu.Unlock()
			if changed && a.ctx != nil {
				runtime.EventsEmit(a.ctx, "peripherals:update", items)
			}
		}
	}
}

func peripheralsFingerprint(items []PeripheralItem) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, it.DeviceID+"|"+it.Name)
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

func (a *App) positionMiniHUD() {
	w, h := 1920, 1080
	if screens, err := runtime.ScreenGetAll(a.ctx); err == nil {
		for _, s := range screens {
			if s.IsPrimary || s.IsCurrent {
				if s.Size.Width > 0 {
					w = s.Size.Width
				} else if s.Width > 0 {
					w = s.Width
				}
				if s.Size.Height > 0 {
					h = s.Size.Height
				} else if s.Height > 0 {
					h = s.Height
				}
				break
			}
		}
	}
	const margin = 18
	const taskbar = 52
	x := w - miniHUDW - margin
	y := h - miniHUDH - margin - taskbar
	if x < 0 {
		x = margin
	}
	if y < 0 {
		y = margin
	}
	runtime.WindowSetPosition(a.ctx, x, y)
}

func (a *App) centerMainWindow() {
	w, h := 1920, 1080
	if screens, err := runtime.ScreenGetAll(a.ctx); err == nil {
		for _, s := range screens {
			if s.IsPrimary || s.IsCurrent {
				if s.Size.Width > 0 {
					w = s.Size.Width
				} else if s.Width > 0 {
					w = s.Width
				}
				if s.Size.Height > 0 {
					h = s.Size.Height
				} else if s.Height > 0 {
					h = s.Height
				}
				break
			}
		}
	}
	x := (w - mainWindowW) / 2
	y := (h - mainWindowH) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	runtime.WindowSetPosition(a.ctx, x, y)
}

// IsMiniHUD reports compact overlay state.
func (a *App) IsMiniHUD() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.miniHUD
}

// ToggleFullscreen toggles native fullscreen (F11 / titlebar control).
func (a *App) ToggleFullscreen() {
	if a.ctx == nil {
		return
	}
	if runtime.WindowIsFullscreen(a.ctx) {
		runtime.WindowUnfullscreen(a.ctx)
		return
	}
	runtime.WindowFullscreen(a.ctx)
}

// IsFullscreen reports whether the window is currently fullscreen.
func (a *App) IsFullscreen() bool {
	if a.ctx == nil {
		return false
	}
	return runtime.WindowIsFullscreen(a.ctx)
}

// ShowWindow restores and focuses the main window.
func (a *App) ShowWindow() {
	if a.ctx == nil {
		return
	}
	a.mu.Lock()
	a.hidden = false
	a.mu.Unlock()
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
}

// HideToTray hides the window while keeping the process alive in the tray.
func (a *App) HideToTray() {
	if a.ctx == nil {
		return
	}
	a.mu.Lock()
	a.hidden = true
	a.mu.Unlock()
	runtime.WindowHide(a.ctx)
}

// QuitApp exits the application and tears down the tray icon.
func (a *App) QuitApp() {
	quitSystray()
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// beforeClose routes the window close affordance into the tray.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	a.mu.Lock()
	a.hidden = true
	a.mu.Unlock()
	runtime.WindowHide(ctx)
	return true
}

func isNonFixedPartition(p disk.PartitionStat) bool {
	opts := strings.ToLower(strings.Join(p.Opts, ","))
	ft := strings.ToLower(p.Fstype)
	mp := strings.ToLower(p.Mountpoint)
	if strings.Contains(opts, "removable") || strings.Contains(opts, "cdrom") ||
		strings.Contains(opts, "remote") || strings.Contains(opts, "ram") ||
		strings.Contains(opts, "network") {
		return true
	}
	if strings.Contains(ft, "cdfs") || strings.Contains(ft, "udf") || strings.Contains(ft, "iso9660") {
		return true
	}
	if strings.HasPrefix(mp, `\\`) || strings.Contains(mp, "\\\\") {
		return true
	}
	return false
}

func (a *App) collectDisks(now time.Time) []DiskMetrics {
	partitions, err := disk.Partitions(true)
	if err != nil {
		partitions, err = disk.Partitions(false)
		if err != nil {
			return []DiskMetrics{}
		}
	}
	ioCounters, _ := disk.IOCounters()

	elapsed := now.Sub(a.prevDiskTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}

	out := make([]DiskMetrics, 0, len(partitions))
	for _, p := range partitions {
		if isNonFixedPartition(p) {
			continue
		}
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}
		dm := DiskMetrics{
			Mount:   p.Mountpoint,
			UsedGB:  float64(usage.Used) / (1024 * 1024 * 1024),
			TotalGB: float64(usage.Total) / (1024 * 1024 * 1024),
			FreeGB:  float64(usage.Free) / (1024 * 1024 * 1024),
			Percent: usage.UsedPercent,
		}

		// Match IO counter device names loosely to mount (C: -> C: or PhysicalDrive).
		for name, cur := range ioCounters {
			mountKey := strings.TrimRight(p.Mountpoint, `\`)
			if !strings.Contains(strings.ToUpper(name), strings.ToUpper(strings.TrimSuffix(mountKey, ":"))) &&
				!strings.EqualFold(name, mountKey) {
				// Also accept first physical drive mapping for C:
				if !(mountKey == "C:" && strings.Contains(strings.ToLower(name), "0")) {
					continue
				}
			}
			if prev, ok := a.prevDiskIO[name]; ok && a.prevDiskTime.Year() > 1 {
				dm.ReadMBps = float64(cur.ReadBytes-prev.ReadBytes) / (1024 * 1024) / elapsed
				dm.WriteMBps = float64(cur.WriteBytes-prev.WriteBytes) / (1024 * 1024) / elapsed
				if dm.ReadMBps < 0 {
					dm.ReadMBps = 0
				}
				if dm.WriteMBps < 0 {
					dm.WriteMBps = 0
				}
			}
			break
		}
		out = append(out, dm)
	}

	a.prevDiskIO = ioCounters
	a.prevDiskTime = now
	return out
}

func (a *App) collectGPUUsage() (pct, usedMB, sharedMB, dedicatedMB float64) {
	a.mu.RLock()
	total := float64(a.gpuTotalVRAM) / (1024 * 1024)
	a.mu.RUnlock()

	// Best-effort via absolute nvidia-smi path (no PATH lookup / hijack).
	if smi := nvidiaSMIPath(); smi != "" {
		if out, err := runHiddenOutput(smi,
			"--query-gpu=utilization.gpu,memory.used,memory.total",
			"--format=csv,noheader,nounits"); err == nil {
			parts := strings.Split(strings.TrimSpace(string(out)), ",")
			if len(parts) >= 3 {
				fmt.Sscanf(strings.TrimSpace(parts[0]), "%f", &pct)
				fmt.Sscanf(strings.TrimSpace(parts[1]), "%f", &usedMB)
				fmt.Sscanf(strings.TrimSpace(parts[2]), "%f", &total)
				dedicatedMB = usedMB
				if total > usedMB {
					sharedMB = 0
				}
				a.mu.Lock()
				if total > 0 {
					a.gpuTotalVRAM = uint64(total * 1024 * 1024)
				}
				a.mu.Unlock()
				return pct, usedMB, sharedMB, dedicatedMB
			}
		}
	}

	// Fallback: derive light GPU pressure proxy from top 3D-ish processes.
	procs, err := process.Processes()
	if err == nil {
		var sum float64
		for _, p := range procs {
			name, _ := p.Name()
			ln := strings.ToLower(name)
			if strings.Contains(ln, "dwm") || strings.Contains(ln, "chrome") ||
				strings.Contains(ln, "game") || strings.Contains(ln, "steam") ||
				strings.HasSuffix(ln, ".exe") && (strings.Contains(ln, "gfx") || strings.Contains(ln, "nvidia") || strings.Contains(ln, "amd")) {
				c, _ := p.CPUPercent()
				sum += c * 0.15
			}
		}
		if sum > 100 {
			sum = 100
		}
		pct = sum
	}

	dedicatedMB = total * (pct / 100.0)
	usedMB = dedicatedMB
	if vm, err := mem.VirtualMemory(); err == nil {
		sharedMB = float64(vm.Used) / (1024 * 1024) * 0.02
	}
	return pct, usedMB, sharedMB, dedicatedMB
}

func readCPUName() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, registry.QUERY_VALUE)
	if err != nil {
		info, _ := cpu.Info()
		if len(info) > 0 {
			return strings.TrimSpace(info[0].ModelName)
		}
		return "Unknown CPU"
	}
	defer k.Close()
	name, _, err := k.GetStringValue("ProcessorNameString")
	if err != nil {
		return "Unknown CPU"
	}
	return strings.TrimSpace(name)
}

var oleOnce sync.Once

func ensureOLE() {
	oleOnce.Do(func() {
		_ = ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	})
}

func readGPUInfo() (string, uint64) {
	name, vram, _ := readGPUInfoDetailed()
	return name, vram
}

func readCPUTemperature() float64 {
	var maxC float64
	ok := withSensorGate(defaultSensorTimeout, func() {
		ensureOLE()
		locator, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
		if err != nil {
			return
		}
		defer locator.Release()
		dl, err := locator.QueryInterface(ole.IID_IDispatch)
		if err != nil {
			return
		}
		defer dl.Release()

		// ACPI thermal zones (may require elevated privileges).
		serviceRaw, err := oleutil.CallMethod(dl, "ConnectServer", nil, `root\wmi`)
		if err != nil {
			return
		}
		service := serviceRaw.ToIDispatch()
		defer service.Release()

		resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT CurrentTemperature FROM MSAcpi_ThermalZoneTemperature")
		if err != nil {
			return
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
			// Value is in tenths of Kelvin.
			kelvinTenths := float64(tempVar.Val)
			c := (kelvinTenths / 10.0) - 273.15
			if c > maxC {
				maxC = c
			}
		}
	})
	if !ok || maxC < 10 || maxC > 125 {
		return 0
	}
	return maxC
}

func isSelfOrWebView(name string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	switch base {
	case "pulsecore.exe", "zeropulse.exe", "zeropulse-gui.exe", "msedgewebview2.exe", "wails.exe":
		return true
	default:
		return false
	}
}

func processIcon(name string) string {
	n := strings.ToLower(filepath.Base(name))
	switch {
	case strings.Contains(n, "chrome"), strings.Contains(n, "msedge"), strings.Contains(n, "firefox"), strings.Contains(n, "brave"):
		return "🌐"
	case strings.Contains(n, "code"), strings.Contains(n, "cursor"), strings.Contains(n, "devenv"):
		return "💻"
	case strings.Contains(n, "discord"), strings.Contains(n, "telegram"), strings.Contains(n, "slack"):
		return "💬"
	case strings.Contains(n, "steam"), strings.Contains(n, "game"):
		return "🎮"
	case strings.Contains(n, "msmpeng"), strings.Contains(n, "defender"):
		return "🛡"
	default:
		return "⚙️"
	}
}

func findHottestProcess() (string, int32) {
	procs, err := process.Processes()
	if err != nil {
		return "unknown", 0
	}
	var (
		bestName string
		bestPID  int32
		bestCPU  float64
	)
	for _, p := range procs {
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}
		if isSelfOrWebView(name) {
			continue
		}
		lower := strings.ToLower(filepath.Base(name))
		if lower == "system" || lower == "idle" || lower == "system idle process" {
			continue
		}
		if isNoiseBackgroundProcess(name) || isNoiseBackgroundProcess(lower) {
			continue
		}
		c, err := p.CPUPercent()
		if err != nil {
			continue
		}
		if c > bestCPU {
			bestCPU = c
			bestName = name
			bestPID = p.Pid
		}
	}
	if bestName == "" {
		return "unknown", 0
	}
	return bestName, bestPID
}

func foregroundPID() int32 {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return 0
	}
	var pid uint32
	procGetWindowThreadProcId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return int32(pid)
}

