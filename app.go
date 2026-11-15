package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
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
	mainWindowW     = 1280
	mainWindowH     = 800
	miniHUDW        = 380
	miniHUDH        = 44
	gameOverlayW    = 460
	gameOverlayH    = 44
	mainMinW        = 760
	mainMinH        = 560
	createNoWindow  = 0x08000000 // CREATE_NO_WINDOW
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
	modKernel32               = windows.NewLazySystemDLL("kernel32.dll")
)

// App is the PulseCore application backend.
type App struct {
	ctx context.Context

	mu            sync.RWMutex
	metrics       SystemMetrics
	lagEvents     []LagEvent
	stutterEvents []StutterEvent
	vampires      []VampireProcess
	vampTracks    map[string]*vampTrack
	pollInterval  time.Duration
	miniHUD       bool
	gameOverlay   bool
	hidden        bool
	powerMode     string
	language      string
	activeTab     string
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
	ecoPIDs            map[int32]bool
	throttledProcesses map[string]time.Time
	prefs              appPrefs

	lastBattery        time.Time
	cachedBat          BatteryMetrics
	lastTemp           time.Time
	cachedTemp         float64
	tempUnavailable    bool
	lastGPU            time.Time
	cachedGPUPct       float64
	cachedVRAMUsed     float64
	cachedVRAMShare    float64
	cachedVRAMDed      float64
	cachedGPUTemp      float64
	cachedGPUTempOK    bool
	cachedGPUPowerW    float64
	cachedGPUPowerOK   bool
	cachedGPUClockCore float64
	cachedGPUClockMem  float64
	nvidiaMissing      bool
	cachedTempSource   string
	cachedTempLabelRu  string
	cachedTempLabelEn  string
	lastDisk           time.Time
	cachedDisks        []DiskMetrics
	diskCollecting     atomic.Bool
	lastVamp           time.Time

	sessionStart      time.Time
	sessionHits       map[string]*sessionHit
	lastSession       time.Time
	lagRing           RingBuffer
	bb                *blackBox
	ecoExitConfirmed  bool
	forceQuit         bool
	stats             *statsEngine
	gameDet           *gameDetectState
	gameBook          *gameBook
	lastStatsTick     time.Time
	tray              *trayManager
	radar             *processRadar
	memWatch          *memoryWatch
	health            *healthEngine
	startupScan       *startupCache
	netMon            *networkMonitor
	netCheck          *netCheckCache
	intel             *intelligenceEngine
	prepareGame       *prepareGameState
	diskSMARTCache    map[string]DiskHealthReport // drive letter → last SMART
	diskSMARTCacheAt  map[string]time.Time
	diskWatch         *diskWatchEngine
	smartFixJournal   *smartFixJournalEngine
	dismissedProblems *dismissedProblemsEngine
	cleanHistory      *cleanHistoryEngine
}

// SystemMetrics is a full snapshot of live hardware state.
type SystemMetrics struct {
	Timestamp                string           `json:"timestamp"`
	CPUName                  string           `json:"cpuName"`
	CPUPercent               float64          `json:"cpuPercent"`
	CPUPerCore               []float64        `json:"cpuPerCore"`
	GPUName                  string           `json:"gpuName"`
	GPUPercent               float64          `json:"gpuPercent"`
	VRAMUsedMB               float64          `json:"vramUsedMB"`
	VRAMTotalMB              float64          `json:"vramTotalMB"`
	VRAMSharedMB             float64          `json:"vramSharedMB"`
	VRAMDedicatedMB          float64          `json:"vramDedicatedMB"`
	RAMPercent               float64          `json:"ramPercent"`
	RAMUsedMB                uint64           `json:"ramUsedMB"`
	RAMTotalMB               uint64           `json:"ramTotalMB"`
	SwapPercent              float64          `json:"swapPercent"`
	SwapUsedMB               uint64           `json:"swapUsedMB"`
	SwapTotalMB              uint64           `json:"swapTotalMB"`
	Disks                    []DiskMetrics    `json:"disks"`
	Battery                  BatteryMetrics   `json:"battery"`
	Hardware                 HardwareInfo     `json:"hardware"`
	TemperatureC             float64          `json:"temperatureC"`
	TemperatureAvailable     bool             `json:"temperatureAvailable"`
	TemperatureSource        string           `json:"temperatureSource"` // hwinfo|lhm|acpi|""
	TemperatureSourceLabelRu string           `json:"temperatureSourceLabelRu"`
	TemperatureSourceLabelEn string           `json:"temperatureSourceLabelEn"`
	GPUTemperatureC          float64          `json:"gpuTemperatureC"`
	GPUTemperatureAvailable  bool             `json:"gpuTemperatureAvailable"`
	GPUPowerW                float64          `json:"gpuPowerW"`
	GPUPowerAvailable        bool             `json:"gpuPowerAvailable"`
	GPUClockCoreMHz          float64          `json:"gpuClockCoreMHz"`
	GPUClockMemMHz           float64          `json:"gpuClockMemMHz"`
	PowerMode                string           `json:"powerMode"`
	PollIntervalMS           int              `json:"pollIntervalMS"`
	MiniHUD                  bool             `json:"miniHUD"`
	Language                 string           `json:"language"`
	LagEvents                []LagEvent       `json:"lagEvents"`
	StutterEvents            []StutterEvent   `json:"stutterEvents"`
	Vampires                 []VampireProcess `json:"vampires"`
}

// DiskMetrics describes a mounted volume and its IO throughput.
type DiskMetrics struct {
	Mount     string  `json:"mount"`
	UsedGB    float64 `json:"usedGB"`
	TotalGB   float64 `json:"totalGB"`
	Percent   float64 `json:"percent"`
	ReadMBps  float64 `json:"readMBps"`
	WriteMBps float64 `json:"writeMBps"`
	FreeGB    float64 `json:"freeGB"`
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
	ReasonRu     string  `json:"reasonRu"`
	ReasonEn     string  `json:"reasonEn"`
}

// NewApp constructs the application shell.
func NewApp() *App {
	return &App{
		pollInterval:  time.Duration(defaultPollMS) * time.Millisecond,
		powerMode:     "balanced",
		language:      "ru",
		activeTab:     "overview",
		prevDiskIO:    make(map[string]disk.IOCountersStat),
		lastLagAt:     make(map[int]time.Time),
		lagEvents:     make([]LagEvent, 0, maxLagEvents),
		stutterEvents: make([]StutterEvent, 0, maxStutterEvents),
		vampires:      make([]VampireProcess, 0),
		stopWorker:    make(chan struct{}),
		ecoPIDs:       make(map[int32]bool),
		prefs:         defaultPrefs(),
		sessionStart:  time.Now(),
		sessionHits:   make(map[string]*sessionHit),
		bb:            newBlackBox(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.CleanupOldBinary()
	a.loadPrefs()
	a.loadEcoState()
	a.loadStats()
	a.loadDiskWatch()
	a.loadSmartFixJournal()
	a.loadGameBook()
	if a.prefs.StartMinimized {
		runtime.WindowHide(ctx)
	}
	a.staticHW = collectStaticHardware()
	a.cpuName = a.staticHW.cpuName
	a.gpuName = a.staticHW.gpuName
	a.gpuTotalVRAM = a.staticHW.gpuVRAM
	a.workerOnce.Do(func() {
		go a.metricsWorker()
	})
	a.flightOnce.Do(func() {
		// Lag Black Box detector (replaces the old Flight Recorder tick as the
		// authoritative incident source). Legacy SmoothnessStatus still works.
		a.startLagDetector()
	})
	a.startTray()
	go a.peripheralsWatchdog()
	a.StartConflictWatcher()
}

func (a *App) shutdown(ctx context.Context) {
	nvmlShutdown()
	a.stopTray()
	DestroyLingeringConflictWindows()
	setGameTimerPeriod(false)
	a.persistEcoState()
	a.flushStats(true)
	a.flushDiskWatch()
	a.flushSmartFixJournal()
	if h := a.health; h != nil {
		h.mu.Lock()
		h.dirty = true
		a.flushHealthLocked(h)
		h.mu.Unlock()
	}
	if e := a.intel; e != nil {
		e.mu.Lock()
		e.dirty = true
		a.flushIntelLocked(e)
		e.mu.Unlock()
	}
	select {
	case <-a.stopWorker:
	default:
		close(a.stopWorker)
	}
}

func (a *App) recordThrottledProcess(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.throttledProcesses == nil {
		a.throttledProcesses = make(map[string]time.Time)
	}
	key := strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	if key != "" {
		a.throttledProcesses[key] = time.Now().Add(5 * time.Minute)
	}
}

func (a *App) isThrottledProcess(name string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.throttledProcesses == nil {
		return false
	}
	key := strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	if key == "" {
		return false
	}
	until, ok := a.throttledProcesses[key]
	if !ok {
		return false
	}
	return time.Now().Before(until)
}

func (a *App) metricsWorker() {
	defer func() {
		if r := recover(); r != nil {
			logCrash("metricsWorker-fatal", r)
		}
	}()
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logCrash("metricsWorker-loop", r)
				}
			}()
			a.collectOnce()
		}()
		if !a.pauseCollect() {
			return
		}
	}
}

// pauseCollect blocks the sampler for at least one second (or the chosen slower interval)
// and returns false when the app is shutting down.
func (a *App) pauseCollect() bool {
	a.mu.RLock()
	wait := a.pollInterval
	a.mu.RUnlock()
	if wait < time.Second {
		wait = time.Second
	}
	deadline := time.Now().Add(wait)
	for {
		select {
		case <-a.stopWorker:
			return false
		case <-a.ctx.Done():
			return false
		default:
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return true
		}
		step := time.Second
		if remaining < step {
			step = remaining
		}
		time.Sleep(step)
	}
}

func (a *App) collectOnce() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered in collectOnce: %v", r)
		}
	}()
	if a.ctx != nil {
		select {
		case <-a.ctx.Done():
			return
		default:
		}
	}

	now := time.Now()
	cpuPct, perCore := sampleCPULoad()
	vm, _ := mem.VirtualMemory()
	var sm *mem.SwapMemoryStat
	if s, err := mem.SwapMemory(); err == nil {
		sm = s
	}

	a.mu.RLock()
	tab := a.activeTab
	a.mu.RUnlock()
	if tab == "" {
		tab = "overview"
	}

	disks := a.disksForTab(tab, now)
	bat := a.batteryCached(now)
	gpuPct, vramUsed, vramShared, vramDedicated := a.gpuCached(now, tab)
	temp := a.tempCached(now, cpuPct, tab)
	a.mu.RLock()
	tempReal := temp > 0 && !a.tempUnavailable
	a.mu.RUnlock()
	if tempReal {
		a.TriggerThermalCheck(temp)
	}

	// Core spikes are cheap (no process walk). The culprit process is resolved only when a spike fires.
	a.LagSpotterCheck(perCore, cpuPct)

	a.mu.RLock()
	vampDue := a.lastVamp.IsZero() || now.Sub(a.lastVamp) >= 2*time.Second
	sessionDue := a.lastSession.IsZero() || now.Sub(a.lastSession) >= 2*time.Second
	a.mu.RUnlock()
	var vamps []VampireProcess
	ranVamps := false
	if tab == "lag" && vampDue {
		vamps = a.GetVampireProcesses()
		a.mu.Lock()
		a.vampires = vamps
		a.lastVamp = time.Now()
		a.lastSession = time.Now()
		a.mu.Unlock()
		a.accrueSessionFromVampires(vamps, 2.0)
		ranVamps = true
	} else {
		a.mu.RLock()
		vamps = append([]VampireProcess(nil), a.vampires...)
		a.mu.RUnlock()
	}
	if sessionDue && !ranVamps {
		a.accumulateSessionMetrics(2.0)
		a.mu.Lock()
		a.lastSession = time.Now()
		a.mu.Unlock()
	}
	a.pushBlackBoxSnapshot(now, cpuPct, perCore, vm, sm, disks, gpuPct, temp)

	intervalSec := 1.0
	a.mu.RLock()
	if !a.lastStatsTick.IsZero() {
		intervalSec = now.Sub(a.lastStatsTick).Seconds()
	}
	a.mu.RUnlock()
	if intervalSec < 0.2 {
		intervalSec = 0.2
	}
	if intervalSec > 10 {
		intervalSec = 10
	}
	gpuTemp, gpuTempOK, _, _, _, _ := a.gpuExtras()
	if !gpuTempOK {
		gpuTemp = 0
	}
	a.recordStatsTick(cpuPct, gpuPct, temp, gpuTemp, intervalSec)
	a.tickGameDetect(now, gpuPct, cpuPct)
	diskBusy := 0.0
	diskOK := len(disks) > 0
	if diskOK {
		var rw float64
		for _, d := range disks {
			rw += d.ReadMBps + d.WriteMBps
		}
		diskBusy = math.Min(100, rw*2.5)
	}
	a.mu.RLock()
	tempOK := temp > 0 && !a.tempUnavailable
	gpuOK := !a.nvidiaMissing
	a.mu.RUnlock()
	ramPct := 0.0
	swapPct := 0.0
	if vm != nil {
		ramPct = vm.UsedPercent
	}
	if sm != nil {
		swapPct = sm.UsedPercent
	}
	sysUsed, sysOK := systemDiskUsedPct(disks)
	a.recordHealthTick(now, cpuPct, ramPct, swapPct, gpuPct, temp, diskBusy, sysUsed, tempOK, gpuOK, diskOK, sysOK)
	a.recordIntelligenceTick(now, cpuPct, ramPct, gpuPct, temp, tempOK, gpuOK, a.isGameSessionActive())
	a.mu.Lock()
	a.lastStatsTick = now
	a.mu.Unlock()

	snapshot := a.buildSnapshot(now, cpuPct, perCore, vm, sm, disks, bat, gpuPct, vramUsed, vramShared, vramDedicated, temp)

	a.mu.Lock()
	a.metrics = snapshot
	a.metrics.LagEvents = append([]LagEvent(nil), a.lagEvents...)
	a.metrics.StutterEvents = append([]StutterEvent(nil), a.stutterEvents...)
	a.metrics.Vampires = append([]VampireProcess(nil), a.vampires...)
	a.metrics.PowerMode = a.powerMode
	a.metrics.PollIntervalMS = int(a.pollInterval / time.Millisecond)
	a.metrics.MiniHUD = a.miniHUD
	a.metrics.Language = a.language
	a.mu.Unlock()
}

func (a *App) disksForTab(tab string, now time.Time) []DiskMetrics {
	a.mu.RLock()
	cached := append([]DiskMetrics(nil), a.cachedDisks...)
	due := a.lastDisk.IsZero() || now.Sub(a.lastDisk) >= 2*time.Second
	a.mu.RUnlock()
	if tab != "overview" || !due {
		return cached
	}

	if !a.diskCollecting.CompareAndSwap(false, true) {
		return cached
	}

	ch := make(chan []DiskMetrics, 1)
	go func() {
		defer a.diskCollecting.Store(false)
		ch <- a.collectDisks(now)
	}()

	select {
	case disks := <-ch:
		a.mu.Lock()
		a.cachedDisks = append([]DiskMetrics(nil), disks...)
		a.lastDisk = time.Now()
		a.mu.Unlock()
		return disks
	case <-time.After(500 * time.Millisecond):
		// Slow HDD or hanging drive: return cached snapshot immediately and store result when ready
		go func() {
			if disks, ok := <-ch; ok && len(disks) > 0 {
				a.mu.Lock()
				a.cachedDisks = append([]DiskMetrics(nil), disks...)
				a.lastDisk = time.Now()
				a.mu.Unlock()
			}
		}()
		return cached
	}
}

func (a *App) batteryCached(now time.Time) BatteryMetrics {
	a.mu.RLock()
	cached := a.cachedBat
	last := a.lastBattery
	a.mu.RUnlock()
	if !last.IsZero() && now.Sub(last) < 60*time.Second {
		return cached
	}
	bat := collectBattery()
	a.mu.Lock()
	a.cachedBat = bat
	a.lastBattery = time.Now()
	a.mu.Unlock()
	return bat
}

func (a *App) gpuCached(now time.Time, tab string) (pct, used, shared, dedicated float64) {
	a.mu.RLock()
	pct, used, shared, dedicated = a.cachedGPUPct, a.cachedVRAMUsed, a.cachedVRAMShare, a.cachedVRAMDed
	last := a.lastGPU
	missing := a.nvidiaMissing
	mini := a.miniHUD
	a.mu.RUnlock()
	need := tab == "overview" || tab == "hardware" || mini
	if missing || !need || (!last.IsZero() && now.Sub(last) < 5*time.Second) {
		return pct, used, shared, dedicated
	}
	pct, used, shared, dedicated = a.collectGPUUsage()
	a.mu.Lock()
	a.cachedGPUPct = pct
	a.cachedVRAMUsed = used
	a.cachedVRAMShare = shared
	a.cachedVRAMDed = dedicated
	a.lastGPU = time.Now()
	a.mu.Unlock()
	return pct, used, shared, dedicated
}

func (a *App) gpuExtras() (temp float64, tempOK bool, powerW float64, powerOK bool, clockCore, clockMem float64) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cachedGPUTemp, a.cachedGPUTempOK, a.cachedGPUPowerW, a.cachedGPUPowerOK, a.cachedGPUClockCore, a.cachedGPUClockMem
}

func (a *App) tempCached(now time.Time, cpuPct float64, tab string) float64 {
	_ = cpuPct // never invent a fake °C from load - show unavailable instead
	a.mu.RLock()
	cached := a.cachedTemp
	last := a.lastTemp
	mini := a.miniHUD
	a.mu.RUnlock()
	need := tab == "overview" || tab == "hardware" || tab == "health" || tab == "home" || mini
	if !need && !last.IsZero() {
		return cached // may be 0 = unknown
	}
	if !last.IsZero() && now.Sub(last) < 30*time.Second {
		return cached
	}
	maybeResetExternalTempProbes()
	reading := readCPUTemperatureSmart()
	a.mu.Lock()
	a.lastTemp = time.Now()
	if reading.OK && reading.TempC > 0 {
		a.tempUnavailable = false
		a.cachedTemp = reading.TempC
		a.cachedTempSource = reading.Source
		a.cachedTempLabelRu = reading.LabelRu
		a.cachedTempLabelEn = reading.LabelEn
	} else {
		// Do not permanently latch: HWiNFO/LHM may start later. Keep showing unavailable.
		a.tempUnavailable = true
		a.cachedTemp = 0
		a.cachedTempSource = ""
		a.cachedTempLabelRu = ""
		a.cachedTempLabelEn = ""
	}
	out := a.cachedTemp
	a.mu.Unlock()
	return out
}

func (a *App) buildSnapshot(now time.Time, cpuPct float64, perCore []float64, vm *mem.VirtualMemoryStat, sm *mem.SwapMemoryStat, disks []DiskMetrics, bat BatteryMetrics, gpuPct, vramUsed, vramShared, vramDedicated, temp float64) SystemMetrics {
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

	gpuTemp, gpuTempOK, gpuPower, gpuPowerOK, gpuClockCore, gpuClockMem := a.gpuExtras()
	a.mu.RLock()
	tempSrc := a.cachedTempSource
	tempLblRu := a.cachedTempLabelRu
	tempLblEn := a.cachedTempLabelEn
	a.mu.RUnlock()
	hw := buildHardwareInfo(staticHW, cpuPct, perCore, gpuPct, vramUsed, vramShared, vramDedicated, vm, bat)
	hw.GPU.TemperatureC = gpuTemp
	hw.GPU.TempAvailable = gpuTempOK
	hw.GPU.PowerW = gpuPower
	hw.GPU.PowerAvailable = gpuPowerOK
	hw.GPU.ClockCoreMHz = gpuClockCore
	hw.GPU.ClockMemMHz = gpuClockMem

	return SystemMetrics{
		Timestamp:                now.Format(time.RFC3339),
		CPUName:                  cpuName,
		CPUPercent:               cpuPct,
		CPUPerCore:               perCore,
		GPUName:                  gpuName,
		GPUPercent:               gpuPct,
		VRAMUsedMB:               vramUsed,
		VRAMTotalMB:              vramTotal,
		VRAMSharedMB:             vramShared,
		VRAMDedicatedMB:          vramDedicated,
		RAMPercent:               ramPercentOf(vm),
		RAMUsedMB:                ramUsedOf(vm),
		RAMTotalMB:               ramTotalOf(vm),
		SwapPercent:              swapPercentOf(sm),
		SwapUsedMB:               swapUsedOf(sm),
		SwapTotalMB:              swapTotalOf(sm),
		Disks:                    disks,
		Battery:                  bat,
		Hardware:                 hw,
		TemperatureC:             temp,
		TemperatureAvailable:     temp > 0,
		TemperatureSource:        tempSrc,
		TemperatureSourceLabelRu: tempLblRu,
		TemperatureSourceLabelEn: tempLblEn,
		GPUTemperatureC:          gpuTemp,
		GPUTemperatureAvailable:  gpuTempOK,
		GPUPowerW:                gpuPower,
		GPUPowerAvailable:        gpuPowerOK,
		GPUClockCoreMHz:          gpuClockCore,
		GPUClockMemMHz:           gpuClockMem,
		PowerMode:                powerMode,
		PollIntervalMS:           pollMS,
		MiniHUD:                  mini,
		Language:                 lang,
		LagEvents:                lags,
		StutterEvents:            stutters,
		Vampires:                 vamps,
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
	out.PowerMode = a.powerMode
	out.PollIntervalMS = int(a.pollInterval / time.Millisecond)
	out.MiniHUD = a.miniHUD
	out.Language = a.language
	return out
}

// GetSystemMetrics is the UI poll entry: it returns the cached snapshot and never blocks on WMI.
func (a *App) GetSystemMetrics() SystemMetrics {
	return a.GetMetrics()
}

// SetActiveTab tells the sampler which pane is visible so disks, vampires and WMI stay off the hot path.
func (a *App) SetActiveTab(tab string) {
	switch tab {
	case "overview", "lag", "hardware", "settings", "modes":
	case "gamelab", "gamepulse":
		tab = "gamelab"
	default:
		tab = "overview"
	}
	a.mu.Lock()
	a.activeTab = tab
	a.mu.Unlock()
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

	a.mu.RLock()
	prev := append([]float64(nil), a.prevCorePct...)
	lastLag := make(map[int]time.Time, len(a.lastLagAt))
	for k, v := range a.lastLagAt {
		lastLag[k] = v
	}
	a.mu.RUnlock()

	spikedIdx := make([]int, 0, 1)
	for i, load := range cores {
		spiked := load >= lagSpikeCore && overall < lagSpikeOverall
		if !spiked && len(prev) == len(cores) {
			delta := load - prev[i]
			if delta >= 40 && load >= 90 && overall < 75 {
				spiked = true
			}
		}
		if !spiked {
			continue
		}
		if last, ok := lastLag[i]; ok && time.Since(last) < 3*time.Second {
			continue
		}
		spikedIdx = append(spikedIdx, i)
	}

	culpritName, culpritPID := "", int32(0)
	if len(spikedIdx) > 0 {
		culpritName, culpritPID = findHottestProcess()
	}
	now := time.Now().Format("15:04:05")

	a.mu.Lock()
	defer a.mu.Unlock()

	for _, i := range spikedIdx {
		load := cores[i]
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

func (a *App) accrueSessionFromVampires(vamps []VampireProcess, intervalSec float64) {
	if intervalSec <= 0 {
		intervalSec = 2
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sessionHits == nil {
		a.sessionHits = make(map[string]*sessionHit)
	}
	now := time.Now()
	for _, v := range vamps {
		key := strings.ToLower(filepath.Base(v.Name))
		if key == "" {
			continue
		}
		s := a.sessionHits[key]
		if s == nil {
			s = &sessionHit{Name: v.Name, FriendlyName: v.FriendlyName}
			a.sessionHits[key] = s
		}
		s.CPUSec += (clamp01to100(v.CPUPercent) / 100.0) * intervalSec
		s.DiskMB += math.Max(0, v.DiskMBps) * intervalSec
		s.LastSeen = now
	}
}

func (a *App) accumulateSessionMetrics(intervalSec float64) {
	if intervalSec <= 0 {
		intervalSec = 2
	}
	fgPID := foregroundPID()
	procs, err := process.Processes()
	if err != nil {
		return
	}
	alive := map[int32]bool{}
	type hit struct {
		name string
		cpu  float64
		disk float64
	}
	byName := map[string]*hit{}
	for _, p := range procs {
		if p.Pid == 0 || int32(p.Pid) == fgPID {
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
		raw, err := p.CPUPercent()
		if err != nil {
			continue
		}
		cpu := normalizeProcessCPU(raw)
		io, _ := p.IOCounters()
		disk := processDiskRateMBps(p.Pid, io)
		alive[p.Pid] = true
		if cpu < 0.5 && disk < 0.15 {
			continue
		}
		h := byName[lower]
		if h == nil {
			h = &hit{name: name}
			byName[lower] = h
		}
		h.cpu += cpu
		h.disk += disk
	}
	pruneProcIOCache(alive)

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sessionHits == nil {
		a.sessionHits = make(map[string]*sessionHit)
	}
	now := time.Now()
	for key, h := range byName {
		cpu := clamp01to100(h.cpu)
		s := a.sessionHits[key]
		if s == nil {
			s = &sessionHit{Name: h.name, FriendlyName: friendlyProcessName(h.name)}
			a.sessionHits[key] = s
		}
		s.CPUSec += (cpu / 100.0) * intervalSec
		s.DiskMB += h.disk * intervalSec
		s.LastSeen = now
	}
}

// GetVampireProcesses is implemented in vampires.go (MA + hold-time anti-flicker).

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

	if !a.prefs.TempAlerts {
		a.hotSince = time.Time{}
		a.thermalArmed = false
		return
	}
	threshold := float64(a.prefs.ThermalAlertThreshold)
	if threshold < 75 || threshold > 95 {
		threshold = 85
	}
	if temp > threshold {
		if a.hotSince.IsZero() {
			a.hotSince = now
		}
		if now.Sub(a.hotSince) >= time.Duration(thermalHoldSec)*time.Second && !a.thermalArmed {
			a.thermalArmed = true
			go a.emitThermalAlarm(temp, threshold)
		}
		return
	}
	a.hotSince = time.Time{}
	a.thermalArmed = false
}

func (a *App) emitThermalAlarm(temp float64, threshold float64) {
	defer func() {
		if r := recover(); r != nil {
			logCrash("emitThermalAlarm", r)
		}
	}()
	_, _, _ = procMessageBeep.Call(0x00000030) // MB_ICONEXCLAMATION
	title := "PulseCore Emergency Hardware Alarm"
	body := fmt.Sprintf("CPU temperature %.1f°C exceeded %.0f°C for %ds. Consider Silent mode.", temp, threshold, thermalHoldSec)
	showWindowsToast(title, body)
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "thermal:alarm", map[string]interface{}{
			"temperature": temp,
			"limit":       threshold,
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
		invalidateMainHWNDCache()
		runtime.WindowUnmaximise(a.ctx)
		runtime.WindowSetMinSize(a.ctx, miniHUDW, miniHUDH)
		runtime.WindowSetMaxSize(a.ctx, miniHUDW, miniHUDH)
		runtime.WindowSetSize(a.ctx, miniHUDW, miniHUDH)
		runtime.WindowSetAlwaysOnTop(a.ctx, true)
		// Do not steal focus from the foreground game.
		runtime.WindowShow(a.ctx)
		a.positionMiniHUD()
		// Win10: clip our own window only (PID-verified HWND).
		a.ApplyOverlayWindowRegion(miniHUDW, miniHUDH, miniHUDH/2)
		runtime.EventsEmit(a.ctx, "ui:minihud", true)
		runtime.EventsEmit(a.ctx, "ui:gameoverlay", false)
		return
	}

	a.ClearOverlayWindowRegion()
	a.restoreMainWindow()
	runtime.EventsEmit(a.ctx, "ui:minihud", false)
	runtime.EventsEmit(a.ctx, "ui:gameoverlay", false)
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
		a.ApplyOverlayWindowRegion(gameOverlayW, gameOverlayH, 12)
		runtime.EventsEmit(a.ctx, "ui:gameoverlay", true)
		runtime.EventsEmit(a.ctx, "ui:minihud", false)
		return
	}

	a.ClearOverlayWindowRegion()
	a.restoreMainWindow()
	runtime.EventsEmit(a.ctx, "ui:gameoverlay", false)
	runtime.EventsEmit(a.ctx, "ui:minihud", false)
}

func (a *App) restoreMainWindow() {
	a.mu.Lock()
	a.miniHUD = false
	a.gameOverlay = false
	a.mu.Unlock()
	a.ClearOverlayWindowRegion()
	runtime.WindowSetMaxSize(a.ctx, 0, 0)
	runtime.WindowSetMinSize(a.ctx, mainMinW, mainMinH)
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
	runtime.WindowSetSize(a.ctx, mainWindowW, mainWindowH)
	runtime.WindowCenter(a.ctx)
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
	defer func() {
		if r := recover(); r != nil {
			logCrash("peripheralsWatchdog", r)
		}
	}()
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.stopWorker:
			return
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.mu.RLock()
			tab := a.activeTab
			a.mu.RUnlock()
			if tab != "hardware" {
				continue
			}
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
	defX := w - miniHUDW - margin
	defY := h - miniHUDH - margin - taskbar
	if defX < 0 {
		defX = margin
	}
	if defY < 0 {
		defY = margin
	}
	a.mu.RLock()
	x, y := a.prefs.MiniHUDPosX, a.prefs.MiniHUDPosY
	a.mu.RUnlock()
	// Restore saved position only if it still lands on the current screen.
	if x < 0 || y < 0 || x > w-40 || y > h-40 {
		x, y = defX, defY
	}
	runtime.WindowSetPosition(a.ctx, x, y)
}

// SaveMiniHUDPosition persists the floating capsule location.
func (a *App) SaveMiniHUDPosition(x, y int) {
	a.mu.Lock()
	a.prefs.MiniHUDPosX = x
	a.prefs.MiniHUDPosY = y
	p := a.prefs
	a.mu.Unlock()
	_ = a.savePrefs(p)
}

// SetMiniHUDOpacity stores 60/80/100 opacity preference for the capsule.
func (a *App) SetMiniHUDOpacity(percent int) {
	if percent != 60 && percent != 80 && percent != 100 {
		percent = 80
	}
	a.mu.Lock()
	a.prefs.MiniHUDOpacityPercent = percent
	p := a.prefs
	a.mu.Unlock()
	_ = a.savePrefs(p)
}

// ResizeMiniHUD ensures deterministic dimensions for the floating capsule.
func (a *App) ResizeMiniHUD(width, height int) {
	if a.ctx == nil || !a.IsMiniHUD() {
		return
	}
	width = miniHUDW
	height = miniHUDH
	runtime.WindowSetMinSize(a.ctx, width, height)
	runtime.WindowSetMaxSize(a.ctx, width, height)
	runtime.WindowSetSize(a.ctx, width, height)
	a.ApplyOverlayWindowRegion(width, height, height/2)
}

// GetMiniHUDSettings returns opacity / acrylic prefs for the frontend capsule.
func (a *App) GetMiniHUDSettings() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return map[string]interface{}{
		"opacityPercent": a.prefs.MiniHUDOpacityPercent,
		"acrylic":        a.prefs.MiniHUDAcrylic,
		"posX":           a.prefs.MiniHUDPosX,
		"posY":           a.prefs.MiniHUDPosY,
	}
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
	dedicatedMB = total

	// Prefer NVML (temp/power/clocks + util) - no admin required.
	if s := readGPUViaNVML(); s.OK {
		pct = s.Pct
		usedMB = s.UsedMB
		if s.TotalMB > 0 {
			dedicatedMB = s.TotalMB
			total = s.TotalMB
		}
		a.mu.Lock()
		a.nvidiaMissing = false
		a.cachedGPUTemp = s.TempC
		a.cachedGPUTempOK = s.TempOK
		a.cachedGPUPowerW = s.PowerW
		a.cachedGPUPowerOK = s.PowerOK
		a.cachedGPUClockCore = s.ClockCoreMHz
		a.cachedGPUClockMem = s.ClockMemMHz
		if total > 0 {
			bytes := snapVRAMBytes(uint64(total * 1024 * 1024))
			if bytes > a.gpuTotalVRAM {
				a.gpuTotalVRAM = bytes
			}
			if bytes > a.staticHW.gpuVRAM {
				a.staticHW.gpuVRAM = bytes
			}
		}
		a.mu.Unlock()
		return pct, usedMB, 0, dedicatedMB
	}

	out, err := runHiddenOutput("nvidia-smi",
		"--query-gpu=utilization.gpu,memory.used,memory.total,temperature.gpu,power.draw",
		"--format=csv,noheader,nounits")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			a.mu.Lock()
			a.nvidiaMissing = true
			a.cachedGPUTempOK = false
			a.cachedGPUPowerOK = false
			a.mu.Unlock()
		}
		return 0, 0, 0, dedicatedMB
	}
	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(parts) < 3 {
		return 0, 0, 0, dedicatedMB
	}
	fmt.Sscanf(strings.TrimSpace(parts[0]), "%f", &pct)
	fmt.Sscanf(strings.TrimSpace(parts[1]), "%f", &usedMB)
	fmt.Sscanf(strings.TrimSpace(parts[2]), "%f", &total)
	dedicatedMB = total
	var smiTemp, smiPower float64
	if len(parts) >= 4 {
		fmt.Sscanf(strings.TrimSpace(parts[3]), "%f", &smiTemp)
	}
	if len(parts) >= 5 {
		fmt.Sscanf(strings.TrimSpace(parts[4]), "%f", &smiPower)
	}
	a.mu.Lock()
	a.nvidiaMissing = false
	if smiTemp >= 10 && smiTemp <= 125 {
		a.cachedGPUTemp = smiTemp
		a.cachedGPUTempOK = true
	} else {
		a.cachedGPUTempOK = false
	}
	if smiPower > 0 && smiPower < 1000 {
		a.cachedGPUPowerW = smiPower
		a.cachedGPUPowerOK = true
	} else {
		a.cachedGPUPowerOK = false
	}
	a.cachedGPUClockCore = 0
	a.cachedGPUClockMem = 0
	if total > 0 {
		bytes := snapVRAMBytes(uint64(total * 1024 * 1024))
		if bytes > a.gpuTotalVRAM {
			a.gpuTotalVRAM = bytes
		}
		if bytes > a.staticHW.gpuVRAM {
			a.staticHW.gpuVRAM = bytes
		}
	}
	a.mu.Unlock()
	return pct, usedMB, 0, dedicatedMB
}

func ramPercentOf(vm *mem.VirtualMemoryStat) float64 {
	if vm == nil {
		return 0
	}
	return vm.UsedPercent
}

func ramUsedOf(vm *mem.VirtualMemoryStat) uint64 {
	if vm == nil {
		return 0
	}
	return vm.Used / (1024 * 1024)
}

func ramTotalOf(vm *mem.VirtualMemoryStat) uint64 {
	if vm == nil {
		return 0
	}
	return vm.Total / (1024 * 1024)
}

func swapPercentOf(sm *mem.SwapMemoryStat) float64 {
	if sm == nil {
		return 0
	}
	return sm.UsedPercent
}

func swapUsedOf(sm *mem.SwapMemoryStat) uint64 {
	if sm == nil {
		return 0
	}
	return sm.Used / (1024 * 1024)
}

func swapTotalOf(sm *mem.SwapMemoryStat) uint64 {
	if sm == nil {
		return 0
	}
	return sm.Total / (1024 * 1024)
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
	ensureOLE()

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

	// ACPI thermal zones (may require elevated privileges).
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
	var maxC float64
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
	if maxC < 10 || maxC > 125 {
		return 0
	}
	return maxC
}

// estimateTempFromLoad is intentionally unused for UI - fake °C misleads users.
// Kept only for rare internal heuristics that must never be shown as a sensor reading.
func estimateTempFromLoad(cpuPct float64) float64 {
	return 45.0 + cpuPct*0.45
}

func isSelfOrWebView(name string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	switch base {
	case "pulsecore.exe", "zeropulse.exe", "zeropulse-gui.exe", "msedgewebview2.exe", "wails.exe":
		return true
	}
	if strings.Contains(base, "pulsecore") || strings.Contains(base, "webview2") {
		return true
	}
	if myExe, err := os.Executable(); err == nil && myExe != "" {
		if strings.EqualFold(base, filepath.Base(myExe)) {
			return true
		}
	}
	return false
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
		lower := strings.ToLower(name)
		if lower == "system" || lower == "idle" || lower == "system idle process" {
			continue
		}
		c, err := p.CPUPercent()
		if err != nil {
			continue
		}
		c = normalizeProcessCPU(c)
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

func showWindowsToast(title, message string) {
	// Native toast via PowerShell WinRT (no extra SDK weight in the binary beyond OS).
	ps := fmt.Sprintf(`
$ErrorActionPreference='SilentlyContinue'
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] > $null
$template = @"
<toast>
  <visual>
    <binding template="ToastGeneric">
      <text>%s</text>
      <text>%s</text>
    </binding>
  </visual>
</toast>
"@
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($template)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("PulseCore").Show($toast)
`, xmlEscape(title), xmlEscape(message))

	_ = runHiddenStart("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps)
}

// RunHiddenCommand builds an exec.Cmd that never flashes a console window.
func RunHiddenCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow, // CREATE_NO_WINDOW
	}
	return cmd
}

func runHiddenCommand(name string, args ...string) error {
	return RunHiddenCommand(name, args...).Run()
}

func runHiddenOutput(name string, args ...string) ([]byte, error) {
	return RunHiddenCommand(name, args...).Output()
}

func runHiddenCombinedOutput(name string, args ...string) ([]byte, error) {
	return RunHiddenCommand(name, args...).CombinedOutput()
}

func runHiddenStart(name string, args ...string) error {
	return RunHiddenCommand(name, args...).Start()
}

func xmlEscape(s string) string {
	r := strings.NewReplacer(
		`&`, `&amp;`,
		`<`, `&lt;`,
		`>`, `&gt;`,
		`"`, `&quot;`,
		`'`, `&apos;`,
	)
	return r.Replace(s)
}
