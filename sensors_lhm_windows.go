//go:build windows

package main

import (
	"strings"
	"sync"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

var (
	lhmMu      sync.Mutex
	lhmLastTry time.Time
	lhmLast    externalTempReading
	lhmAbsent  bool
)

func readTempFromLibreHardwareMonitor() externalTempReading {
	lhmMu.Lock()
	defer lhmMu.Unlock()
	if lhmAbsent {
		return externalTempReading{}
	}
	if !lhmLastTry.IsZero() && time.Since(lhmLastTry) < 3*time.Second {
		return lhmLast
	}
	lhmLastTry = time.Now()

	ensureOLE()
	locator, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return externalTempReading{}
	}
	defer locator.Release()
	dl, err := locator.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return externalTempReading{}
	}
	defer dl.Release()

	serviceRaw, err := oleutil.CallMethod(dl, "ConnectServer", nil, `root\LibreHardwareMonitor`)
	if err != nil {
		// Namespace missing → LHM not installed/running with WMI
		lhmAbsent = true
		lhmLast = externalTempReading{}
		return lhmLast
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()

	resultRaw, err := oleutil.CallMethod(service, "ExecQuery",
		`SELECT Name, Value, SensorType, Parent FROM Sensor WHERE SensorType='Temperature'`)
	if err != nil {
		lhmLast = externalTempReading{}
		return lhmLast
	}
	result := resultRaw.ToIDispatch()
	defer result.Release()

	countVar, _ := oleutil.GetProperty(result, "Count")
	count := int(countVar.Val)
	type cand struct {
		score int
		temp  float64
		name  string
	}
	var best cand
	for i := 0; i < count; i++ {
		itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
		if err != nil {
			continue
		}
		item := itemRaw.ToIDispatch()
		nameVar, _ := oleutil.GetProperty(item, "Name")
		valVar, _ := oleutil.GetProperty(item, "Value")
		parentVar, _ := oleutil.GetProperty(item, "Parent")
		item.Release()
		name := strings.TrimSpace(nameVar.ToString())
		parent := strings.TrimSpace(parentVar.ToString())
		temp := float64(valVar.Val)
		// Value may be VT_R8
		if valVar.VT == ole.VT_R8 || valVar.VT == ole.VT_R4 {
			temp = valVar.Value().(float64)
		}
		if temp < 10 || temp > 125 {
			continue
		}
		blob := strings.ToLower(name + " " + parent)
		if strings.Contains(blob, "gpu") || strings.Contains(blob, "hdd") || strings.Contains(blob, "ssd") || strings.Contains(blob, "drive") {
			continue
		}
		score := 0
		switch {
		case strings.Contains(blob, "package"):
			score = 100
		case strings.Contains(blob, "ccd") || strings.Contains(blob, "tctl") || strings.Contains(blob, "tdie"):
			score = 95
		case strings.Contains(blob, "cpu") && strings.Contains(blob, "max"):
			score = 85
		case strings.Contains(blob, "cpu"):
			score = 70
		case strings.Contains(parent, "cpu") || strings.Contains(parent, "amd") || strings.Contains(parent, "intel"):
			score = 60
		default:
			continue
		}
		if score > best.score {
			best = cand{score: score, temp: temp, name: name}
		}
	}
	if best.score == 0 {
		lhmLast = externalTempReading{}
		return lhmLast
	}
	lhmLast = externalTempReading{
		TempC: best.temp, OK: true, Source: "lhm",
		LabelRu: "температура из LibreHardwareMonitor",
		LabelEn: "temperature from LibreHardwareMonitor",
		SensorName: best.name,
	}
	// Allow retry later if user restarts LHM after failed empty read
	lhmAbsent = false
	return lhmLast
}

// resetExternalTempProbes clears "absent" latches so a newly started HWiNFO/LHM can be found.
func resetExternalTempProbes() {
	lhmMu.Lock()
	lhmAbsent = false
	lhmLastTry = time.Time{}
	lhmMu.Unlock()
	hwinfoMu.Lock()
	hwinfoLastTry = time.Time{}
	hwinfoMu.Unlock()
}
