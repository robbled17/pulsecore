//go:build windows

package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	modKernel32Smart   = windows.NewLazySystemDLL("kernel32.dll")
	procGetTickCount64 = modKernel32Smart.NewProc("GetTickCount64")
)

func getTickCount64() uint64 {
	r, _, _ := procGetTickCount64.Call()
	return uint64(r)
}

func queryRAMChannelHints() (single, belowRated, ok bool) {
	type stick struct {
		speed uint32
		cap   uint64
	}
	var sticks []stick
	_ = ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	defer ole.CoUninitialize()
	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return false, false, false
	}
	defer unknown.Release()
	wmi, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return false, false, false
	}
	defer wmi.Release()
	serviceRaw, err := oleutil.CallMethod(wmi, "ConnectServer", nil, `root\cimv2`)
	if err != nil {
		return false, false, false
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()
	resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT Capacity,Speed,ConfiguredClockSpeed FROM Win32_PhysicalMemory")
	if err != nil {
		return false, false, false
	}
	result := resultRaw.ToIDispatch()
	defer result.Release()
	countVar, err := oleutil.GetProperty(result, "Count")
	if err != nil {
		return false, false, false
	}
	count := int(countVar.Val)
	for i := 0; i < count; i++ {
		itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
		if err != nil {
			continue
		}
		item := itemRaw.ToIDispatch()
		capV, _ := oleutil.GetProperty(item, "Capacity")
		spdV, _ := oleutil.GetProperty(item, "ConfiguredClockSpeed")
		if spdV.Val == 0 {
			spdV, _ = oleutil.GetProperty(item, "Speed")
		}
		sticks = append(sticks, stick{speed: uint32(spdV.Val), cap: uint64(capV.Val)})
		item.Release()
	}
	if len(sticks) == 0 {
		return false, false, false
	}
	single = len(sticks) == 1
	// "Below rated" heuristic: configured clock unusually low for DDR4/5 consumer sticks (<2133).
	belowRated = false
	for _, s := range sticks {
		if s.speed > 0 && s.speed < 2133 {
			belowRated = true
			break
		}
	}
	return single, belowRated, true
}

func queryGPUDriverAge() (days int, vendor string, ok bool) {
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	_ = err
	defer ole.CoUninitialize()
	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return 0, "", false
	}
	defer unknown.Release()
	wmi, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return 0, "", false
	}
	defer wmi.Release()
	serviceRaw, err := oleutil.CallMethod(wmi, "ConnectServer", nil, `root\cimv2`)
	if err != nil {
		return 0, "", false
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()
	resultRaw, err := oleutil.CallMethod(service, "ExecQuery",
		"SELECT Name,DriverVersion,DriverDate FROM Win32_VideoController WHERE Availability=3")
	if err != nil {
		return 0, "", false
	}
	result := resultRaw.ToIDispatch()
	defer result.Release()
	countVar, err := oleutil.GetProperty(result, "Count")
	if err != nil || countVar.Val == 0 {
		return 0, "", false
	}
	itemRaw, err := oleutil.CallMethod(result, "ItemIndex", 0)
	if err != nil {
		return 0, "", false
	}
	item := itemRaw.ToIDispatch()
	defer item.Release()
	nameV, _ := oleutil.GetProperty(item, "Name")
	dateV, _ := oleutil.GetProperty(item, "DriverDate")
	vendor = fmt.Sprint(nameV.Value())
	dateStr := fmt.Sprint(dateV.Value())
	// WMI datetime: yyyymmddHHMMSS.mmmmmmsUUU
	if len(dateStr) < 8 {
		return 0, vendor, false
	}
	t, err := time.Parse("20060102", dateStr[:8])
	if err != nil {
		return 0, vendor, false
	}
	days = int(time.Since(t).Hours() / 24)
	if days < 0 {
		days = 0
	}
	return days, vendor, true
}

func queryGameModeFlags() (gameModeOff, gameBarBgOn, ok bool) {
	// Game Mode: HKCU\Software\Microsoft\GameBar AutoGameModeEnabled (1=on) — also GameConfigStore
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\GameBar`, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		if v, _, err := k.GetIntegerValue("AutoGameModeEnabled"); err == nil {
			gameModeOff = v == 0
			ok = true
		} else if v, _, err := k.GetIntegerValue("AllowAutoGameMode"); err == nil {
			gameModeOff = v == 0
			ok = true
		}
	}
	k2, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\GameDVR`, registry.QUERY_VALUE)
	if err == nil {
		defer k2.Close()
		if v, _, err := k2.GetIntegerValue("AppCaptureEnabled"); err == nil {
			gameBarBgOn = v != 0
			ok = true
		}
	}
	return gameModeOff, gameBarBgOn, ok
}

func applyGameModeFix(actionID string) (prev map[string]interface{}, err error) {
	prev = map[string]interface{}{}
	switch actionID {
	case "game_mode_on":
		k, e := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\GameBar`, registry.QUERY_VALUE|registry.SET_VALUE)
		if e != nil {
			k, _, e = registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\GameBar`, registry.SET_VALUE)
		}
		if e != nil {
			return prev, e
		}
		defer k.Close()
		if v, _, e := k.GetIntegerValue("AutoGameModeEnabled"); e == nil {
			prev["AutoGameModeEnabled"] = v
		}
		if v, _, e := k.GetIntegerValue("AllowAutoGameMode"); e == nil {
			prev["AllowAutoGameMode"] = v
		}
		_ = k.SetDWordValue("AutoGameModeEnabled", 1)
		_ = k.SetDWordValue("AllowAutoGameMode", 1)
		return prev, nil
	case "game_bar_bg_off":
		k, e := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\GameDVR`, registry.QUERY_VALUE|registry.SET_VALUE)
		if e != nil {
			k, _, e = registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\GameDVR`, registry.SET_VALUE)
		}
		if e != nil {
			return prev, e
		}
		defer k.Close()
		if v, _, e := k.GetIntegerValue("AppCaptureEnabled"); e == nil {
			prev["AppCaptureEnabled"] = v
		}
		return prev, k.SetDWordValue("AppCaptureEnabled", 0)
	default:
		return prev, fmt.Errorf("unknown game mode action")
	}
}

func restoreGameModeFix(prev map[string]interface{}) error {
	if v, ok := prev["AutoGameModeEnabled"]; ok {
		k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\GameBar`, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer k.Close()
		_ = k.SetDWordValue("AutoGameModeEnabled", toDWORD(v))
	}
	if v, ok := prev["AllowAutoGameMode"]; ok {
		k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\GameBar`, registry.SET_VALUE)
		if err == nil {
			_ = k.SetDWordValue("AllowAutoGameMode", toDWORD(v))
			k.Close()
		}
	}
	if v, ok := prev["AppCaptureEnabled"]; ok {
		k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\GameDVR`, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer k.Close()
		return k.SetDWordValue("AppCaptureEnabled", toDWORD(v))
	}
	return nil
}

func toDWORD(v interface{}) uint32 {
	switch t := v.(type) {
	case float64:
		return uint32(t)
	case int:
		return uint32(t)
	case int64:
		return uint32(t)
	case uint32:
		return t
	case uint64:
		return uint32(t)
	default:
		return 0
	}
}

func (a *App) systemDiskIsHDD() bool {
	if smart, ok := a.cachedSystemDiskSMART(); ok {
		if smart.MediaKind == DiskKindHDD || strings.EqualFold(smart.MediaType, "HDD") {
			return true
		}
	}
	return false
}

func (a *App) gamesLikelyOnHDD() bool {
	// No reliable install-path map yet; avoid false positives.
	return false
}
