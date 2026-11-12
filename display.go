//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

const (
	enumCurrentSettings            = 0xFFFFFFFF
	enumRegistrySettings           = 0xFFFFFFFE
	cdsUpdateRegistry              = 0x00000001
	cdsTest                        = 0x00000002
	cdsReset                       = 0x40000000
	dmPelsWidth                    = 0x00080000
	dmPelsHeight                   = 0x00100000
	dmDisplayFrequency             = 0x00400000
	dmBitsPerPel                   = 0x00040000
	displayDeviceAttachedToDesktop = 0x00000001
	displayDevicePrimaryDevice     = 0x00000004
	dispChangeSuccessful           = 0
	refreshTrialTimeout            = 15 * time.Second
)

var (
	modUser32Display             = windows.NewLazySystemDLL("user32.dll")
	procEnumDisplayDevicesW      = modUser32Display.NewProc("EnumDisplayDevicesW")
	procEnumDisplaySettingsExW   = modUser32Display.NewProc("EnumDisplaySettingsExW")
	procChangeDisplaySettingsExW = modUser32Display.NewProc("ChangeDisplaySettingsExW")
)

// DisplayInfo / MonitorInfo — one attached display with refresh capability.
type DisplayInfo struct {
	DeviceName     string `json:"deviceName"`  // \\.\DISPLAY1
	MonitorName    string `json:"monitorName"` // EDID / registry model
	AdapterName    string `json:"adapterName"`
	IsPrimary      bool   `json:"isPrimary"`
	CurrentWidth   int    `json:"currentWidth"`
	CurrentHeight  int    `json:"currentHeight"`
	Width          int    `json:"width"`  // alias of CurrentWidth (legacy)
	Height         int    `json:"height"` // alias of CurrentHeight (legacy)
	CurrentHz      int    `json:"currentHz"`
	MaxHz          int    `json:"maxHz"`
	MaxSupportedHz int    `json:"maxSupportedHz"` // legacy alias of MaxHz
	AvailableHz    []int  `json:"availableHz"`
	NeedsAttention bool   `json:"needsAttention"` // CurrentHz < MaxHz
	Underclocked   bool   `json:"underclocked"`   // legacy alias
}

// MonitorInfo is kept as an alias for existing Smart Fix / home callers.
type MonitorInfo = DisplayInfo

type displayDeviceW struct {
	Cb           uint32
	DeviceName   [32]uint16
	DeviceString [128]uint16
	StateFlags   uint32
	DeviceID     [128]uint16
	DeviceKey    [128]uint16
}

// DEVMODEW layout matching Windows (size 220).
type devModeFull struct {
	DeviceName       [32]uint16
	SpecVersion      uint16
	DriverVersion    uint16
	Size             uint16
	DriverExtra      uint16
	Fields           uint32
	PositionX        int32
	PositionY        int32
	DisplayOrient    uint32
	DisplayFixedOut  uint32
	Color            int16
	Duplex           int16
	YResolution      int16
	TTOption         int16
	Collate          int16
	FormName         [32]uint16
	LogPixels        uint16
	BitsPerPel       uint32
	PelsWidth        uint32
	PelsHeight       uint32
	DisplayFlags     uint32
	DisplayFrequency uint32
	ICMMethod        uint32
	ICMIntent        uint32
	MediaType        uint32
	DitherType       uint32
	Reserved1        uint32
	Reserved2        uint32
	PanningWidth     uint32
	PanningHeight    uint32
}

type refreshTrial struct {
	device     string
	targetHz   int
	previousHz int
	cancel     chan struct{}
}

var (
	refreshTrialMu sync.Mutex
	activeTrial    *refreshTrial
)

// ListMonitors enumerates attached displays with current vs max refresh at current res.
func (a *App) ListMonitors() []DisplayInfo {
	return listMonitors()
}

// ListDisplays is the explicit multi-monitor API for the monitors modal.
func (a *App) ListDisplays() []DisplayInfo {
	return listMonitors()
}

func listMonitors() []DisplayInfo {
	out := make([]DisplayInfo, 0, 4)
	for i := uint32(0); i < 16; i++ {
		var dd displayDeviceW
		dd.Cb = uint32(unsafe.Sizeof(dd))
		r, _, _ := procEnumDisplayDevicesW.Call(0, uintptr(i), uintptr(unsafe.Pointer(&dd)), 0)
		if r == 0 {
			break
		}
		if dd.StateFlags&displayDeviceAttachedToDesktop == 0 {
			continue
		}
		deviceName := windows.UTF16ToString(dd.DeviceName[:])
		adapterName := windows.UTF16ToString(dd.DeviceString[:])
		monitorName := adapterName

		var mon displayDeviceW
		mon.Cb = uint32(unsafe.Sizeof(mon))
		rr, _, _ := procEnumDisplayDevicesW.Call(
			uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(deviceName))),
			0,
			uintptr(unsafe.Pointer(&mon)),
			0,
		)
		if rr != 0 {
			if s := windows.UTF16ToString(mon.DeviceString[:]); s != "" {
				monitorName = s
			}
		}

		cur := readDisplaySettings(deviceName, enumCurrentSettings)
		if cur.PelsWidth == 0 {
			cur = readDisplaySettings(deviceName, enumRegistrySettings)
		}
		if cur.PelsWidth == 0 {
			continue
		}
		w, h := int(cur.PelsWidth), int(cur.PelsHeight)
		curHz := int(cur.DisplayFrequency)
		hzSet := map[int]struct{}{}
		maxHz := curHz
		for modeIdx := uint32(0); modeIdx < 512; modeIdx++ {
			m := readDisplaySettingsIndex(deviceName, modeIdx)
			if m.PelsWidth == 0 {
				break
			}
			if int(m.PelsWidth) != w || int(m.PelsHeight) != h {
				continue
			}
			hz := int(m.DisplayFrequency)
			if hz < 30 || hz > 500 {
				continue
			}
			hzSet[hz] = struct{}{}
			if hz > maxHz {
				maxHz = hz
			}
		}
		if curHz >= 30 {
			hzSet[curHz] = struct{}{}
		}
		available := make([]int, 0, len(hzSet))
		for hz := range hzSet {
			available = append(available, hz)
		}
		sort.Ints(available)
		needs := curHz > 0 && maxHz > curHz
		info := DisplayInfo{
			DeviceName: deviceName, MonitorName: cleanMonitorName(monitorName),
			AdapterName: adapterName, IsPrimary: dd.StateFlags&displayDevicePrimaryDevice != 0,
			CurrentWidth: w, CurrentHeight: h, Width: w, Height: h,
			CurrentHz: curHz, MaxHz: maxHz, MaxSupportedHz: maxHz,
			AvailableHz: available, NeedsAttention: needs, Underclocked: needs,
		}
		out = append(out, info)
	}
	return out
}

func cleanMonitorName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Monitor"
	}
	return s
}

func readDisplaySettings(device string, mode uint32) (dm devModeFull) {
	dm.Size = uint16(unsafe.Sizeof(dm))
	var devPtr uintptr
	if device != "" {
		devPtr = uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(device)))
	}
	r, _, _ := procEnumDisplaySettingsExW.Call(devPtr, uintptr(mode), uintptr(unsafe.Pointer(&dm)), 0)
	if r == 0 {
		return devModeFull{}
	}
	return dm
}

func readDisplaySettingsIndex(device string, index uint32) (dm devModeFull) {
	dm.Size = uint16(unsafe.Sizeof(dm))
	var devPtr uintptr
	if device != "" {
		devPtr = uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(device)))
	}
	r, _, _ := procEnumDisplaySettingsExW.Call(devPtr, uintptr(index), uintptr(unsafe.Pointer(&dm)), 0)
	if r == 0 {
		return devModeFull{}
	}
	return dm
}

func prepareDevMode(deviceName string, hz int) (devModeFull, error) {
	cur := readDisplaySettings(deviceName, enumCurrentSettings)
	if cur.PelsWidth == 0 {
		return devModeFull{}, fmt.Errorf("cannot read current mode")
	}
	cur.Size = uint16(unsafe.Sizeof(cur))
	cur.Fields = dmBitsPerPel | dmPelsWidth | dmPelsHeight | dmDisplayFrequency
	cur.DisplayFrequency = uint32(hz)
	return cur, nil
}

func changeDisplayHz(deviceName string, hz int, flags uint32) (int32, error) {
	dm, err := prepareDevMode(deviceName, hz)
	if err != nil {
		return -1, err
	}
	devPtr := uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(deviceName)))
	r, _, callErr := procChangeDisplaySettingsExW.Call(
		devPtr,
		uintptr(unsafe.Pointer(&dm)),
		0,
		uintptr(flags),
		0,
	)
	code := int32(r)
	if code != dispChangeSuccessful && callErr != nil && callErr != windows.ERROR_SUCCESS {
		return code, callErr
	}
	return code, nil
}

func resolveDisplayDevice(deviceName string) string {
	deviceName = strings.TrimSpace(deviceName)
	if deviceName != "" {
		return deviceName
	}
	mons := listMonitors()
	for _, m := range mons {
		if m.NeedsAttention {
			return m.DeviceName
		}
	}
	if len(mons) > 0 {
		return mons[0].DeviceName
	}
	return ""
}

// SetDisplayRefreshRate permanently applies hz (registry). Prefer Confirm after TestAndSet.
func (a *App) SetDisplayRefreshRate(deviceName string, hz int) map[string]interface{} {
	ok, msg := setDisplayRefreshRate(deviceName, hz)
	return map[string]interface{}{"ok": ok, "message": msg, "hz": hz, "deviceName": deviceName}
}

func setDisplayRefreshRate(deviceName string, hz int) (bool, string) {
	if hz < 30 || hz > 500 {
		return false, "invalid hz"
	}
	deviceName = resolveDisplayDevice(deviceName)
	if deviceName == "" {
		return false, "no display"
	}
	code, err := changeDisplayHz(deviceName, hz, cdsUpdateRegistry|cdsReset)
	if code != dispChangeSuccessful {
		code2, err2 := changeDisplayHz(deviceName, hz, cdsUpdateRegistry)
		if code2 != dispChangeSuccessful {
			if err2 != nil {
				return false, err2.Error()
			}
			if err != nil {
				return false, err.Error()
			}
			return false, fmt.Sprintf("ChangeDisplaySettingsEx failed (%d)", code2)
		}
	}
	return true, "ok"
}

// TestAndSetRefreshRate: CDS_TEST → temporary apply → 15s auto-revert unless Confirm.
func (a *App) TestAndSetRefreshRate(deviceName string, hz int) map[string]interface{} {
	if hz < 30 || hz > 500 {
		return map[string]interface{}{"ok": false, "message": "invalid hz"}
	}
	deviceName = resolveDisplayDevice(deviceName)
	if deviceName == "" {
		return map[string]interface{}{"ok": false, "message": "no display"}
	}
	cur := readDisplaySettings(deviceName, enumCurrentSettings)
	if cur.PelsWidth == 0 {
		return map[string]interface{}{"ok": false, "message": "cannot read current mode"}
	}
	previousHz := int(cur.DisplayFrequency)

	// Step 1: dry-run.
	testCode, _ := changeDisplayHz(deviceName, hz, cdsTest)
	if testCode != dispChangeSuccessful {
		return map[string]interface{}{
			"ok": false,
			"message": "Режим не поддерживается видеодрайвером",
			"messageEn": "Mode is not supported by the video driver",
			"code": testCode,
		}
	}

	// Cancel any previous trial on this or another device.
	a.cancelRefreshTrialLocked(false)

	// Step 2: temporary apply (no registry).
	applyCode, err := changeDisplayHz(deviceName, hz, 0)
	if applyCode != dispChangeSuccessful {
		msg := "Режим не поддерживается видеодрайвером"
		if err != nil {
			msg = err.Error()
		}
		return map[string]interface{}{"ok": false, "message": msg, "code": applyCode}
	}

	cancel := make(chan struct{})
	trial := &refreshTrial{
		device: deviceName, targetHz: hz, previousHz: previousHz, cancel: cancel,
	}
	refreshTrialMu.Lock()
	activeTrial = trial
	refreshTrialMu.Unlock()

	go func(t *refreshTrial) {
		timer := time.NewTimer(refreshTrialTimeout)
		defer timer.Stop()
		select {
		case <-t.cancel:
			return
		case <-timer.C:
			refreshTrialMu.Lock()
			if activeTrial != t {
				refreshTrialMu.Unlock()
				return
			}
			activeTrial = nil
			refreshTrialMu.Unlock()
			_, _ = changeDisplayHz(t.device, t.previousHz, 0)
			if a != nil && a.ctx != nil {
				runtime.EventsEmit(a.ctx, "display-refresh-reverted", map[string]interface{}{
					"deviceName": t.device, "hz": t.previousHz, "reason": "timeout",
				})
			}
		}
	}(trial)

	return map[string]interface{}{
		"ok": true, "message": "ok",
		"deviceName": deviceName, "hz": hz, "previousHz": previousHz,
		"timeoutSec": int(refreshTrialTimeout.Seconds()),
	}
}

// ConfirmRefreshRate commits the trial mode to the registry and cancels auto-revert.
func (a *App) ConfirmRefreshRate(deviceName string, hz int) map[string]interface{} {
	refreshTrialMu.Lock()
	t := activeTrial
	if t != nil {
		select {
		case <-t.cancel:
		default:
			close(t.cancel)
		}
		if deviceName == "" {
			deviceName = t.device
		}
		if hz <= 0 {
			hz = t.targetHz
		}
		activeTrial = nil
	}
	refreshTrialMu.Unlock()

	deviceName = resolveDisplayDevice(deviceName)
	if hz <= 0 {
		return map[string]interface{}{"ok": false, "message": "invalid hz"}
	}
	ok, msg := setDisplayRefreshRate(deviceName, hz)
	return map[string]interface{}{"ok": ok, "message": msg, "deviceName": deviceName, "hz": hz}
}

// RevertRefreshRate immediately restores the pre-trial refresh rate.
func (a *App) RevertRefreshRate(deviceName string) map[string]interface{} {
	refreshTrialMu.Lock()
	t := activeTrial
	prev := 0
	dev := strings.TrimSpace(deviceName)
	if t != nil {
		select {
		case <-t.cancel:
		default:
			close(t.cancel)
		}
		if dev == "" {
			dev = t.device
		}
		prev = t.previousHz
		activeTrial = nil
	}
	refreshTrialMu.Unlock()

	if prev <= 0 {
		cur := readDisplaySettings(resolveDisplayDevice(dev), enumRegistrySettings)
		prev = int(cur.DisplayFrequency)
	}
	dev = resolveDisplayDevice(dev)
	if dev == "" || prev <= 0 {
		return map[string]interface{}{"ok": false, "message": "nothing to revert"}
	}
	code, err := changeDisplayHz(dev, prev, 0)
	if code != dispChangeSuccessful {
		ok, msg := setDisplayRefreshRate(dev, prev)
		return map[string]interface{}{"ok": ok, "message": msg, "hz": prev, "deviceName": dev}
	}
	_ = err
	return map[string]interface{}{"ok": true, "message": "ok", "hz": prev, "deviceName": dev}
}

func (a *App) cancelRefreshTrialLocked(revert bool) {
	refreshTrialMu.Lock()
	t := activeTrial
	activeTrial = nil
	refreshTrialMu.Unlock()
	if t == nil {
		return
	}
	select {
	case <-t.cancel:
	default:
		close(t.cancel)
	}
	if revert {
		_, _ = changeDisplayHz(t.device, t.previousHz, 0)
	}
}

// queryDisplayRefresh kept for Smart Fix input (primary underclocked / primary monitor).
func queryDisplayRefresh() (cur, max int, ok bool) {
	mons := listMonitors()
	if len(mons) == 0 {
		return 0, 0, false
	}
	var pick DisplayInfo
	for _, m := range mons {
		if m.NeedsAttention {
			pick = m
			break
		}
	}
	if pick.DeviceName == "" {
		for _, m := range mons {
			if m.IsPrimary {
				pick = m
				break
			}
		}
	}
	if pick.DeviceName == "" {
		pick = mons[0]
	}
	if pick.CurrentHz <= 0 || pick.MaxHz <= 0 {
		return 0, 0, false
	}
	return pick.CurrentHz, pick.MaxHz, true
}

func primaryUnderclockedMonitor() (DisplayInfo, bool) {
	for _, m := range listMonitors() {
		if m.NeedsAttention {
			return m, true
		}
	}
	return DisplayInfo{}, false
}
