//go:build windows

package main

import (
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	hwinfoMapNameA = "Global\\HWiNFO_SENS_SM2"
	hwinfoMapNameB = "HWiNFO_SENS_SM2"
	hwinfoSig      = 0x53695748 // 'HWiS'
	hwinfoTypeTemp = 1
)

type hwinfoHeader struct {
	Signature              uint32
	Version                uint32
	Revision               uint32
	PollTime               int64
	OffsetOfSensorSection  uint32
	SizeOfSensorElement    uint32
	NumSensorElements      uint32
	OffsetOfReadingSection uint32
	SizeOfReadingElement   uint32
	NumReadingElements     uint32
}

type hwinfoReading struct {
	ReadingType uint32
	SensorIndex uint32
	ReadingID   uint32
	LabelOrig   [128]byte
	LabelUser   [128]byte
	Unit        [16]byte
	Value       float64
	ValueMin    float64
	ValueMax    float64
	ValueAvg    float64
}

type externalTempReading struct {
	TempC     float64
	OK        bool
	Source    string // hwinfo|lhm
	LabelRu   string
	LabelEn   string
	SensorName string
}

var (
	hwinfoMu       sync.Mutex
	hwinfoLastTry  time.Time
	hwinfoLastRead externalTempReading
)

func cString128(b [128]byte) string {
	n := 0
	for n < len(b) && b[n] != 0 {
		n++
	}
	return string(b[:n])
}

func readTempFromHWiNFO() externalTempReading {
	hwinfoMu.Lock()
	defer hwinfoMu.Unlock()
	if !hwinfoLastTry.IsZero() && time.Since(hwinfoLastTry) < 2*time.Second && hwinfoLastRead.OK {
		return hwinfoLastRead
	}
	hwinfoLastTry = time.Now()

	k32 := windows.NewLazySystemDLL("kernel32.dll")
	openMap := k32.NewProc("OpenFileMappingW")
	mapView := k32.NewProc("MapViewOfFile")
	unmapView := k32.NewProc("UnmapViewOfFile")
	const fileMapRead = 0x0004

	var mapHandle windows.Handle
	for _, name := range []string{hwinfoMapNameA, hwinfoMapNameB} {
		p, err := windows.UTF16PtrFromString(name)
		if err != nil {
			continue
		}
		r, _, _ := openMap.Call(uintptr(fileMapRead), 0, uintptr(unsafe.Pointer(p)))
		if r != 0 {
			mapHandle = windows.Handle(r)
			break
		}
	}
	if mapHandle == 0 {
		hwinfoLastRead = externalTempReading{}
		return hwinfoLastRead
	}
	defer windows.CloseHandle(mapHandle)

	addr, _, _ := mapView.Call(uintptr(mapHandle), uintptr(fileMapRead), 0, 0, 0)
	if addr == 0 {
		hwinfoLastRead = externalTempReading{}
		return hwinfoLastRead
	}
	defer unmapView.Call(addr)

	hdr := (*hwinfoHeader)(unsafe.Pointer(addr))
	if hdr.Signature != hwinfoSig || hdr.NumReadingElements == 0 || hdr.SizeOfReadingElement == 0 {
		hwinfoLastRead = externalTempReading{}
		return hwinfoLastRead
	}
	base := uintptr(addr)
	off := base + uintptr(hdr.OffsetOfReadingSection)
	size := uintptr(hdr.SizeOfReadingElement)
	n := int(hdr.NumReadingElements)
	if n > 4096 {
		n = 4096
	}

	type cand struct {
		score int
		temp  float64
		name  string
	}
	var best cand
	for i := 0; i < n; i++ {
		p := unsafe.Pointer(off + uintptr(i)*size)
		// Safe only if element size >= sizeof(hwinfoReading); older layouts still have Value at same offset after labels.
		if size < unsafe.Sizeof(hwinfoReading{}) {
			continue
		}
		rd := (*hwinfoReading)(p)
		if rd.ReadingType != hwinfoTypeTemp {
			continue
		}
		t := rd.Value
		if t < 10 || t > 125 {
			continue
		}
		label := cString128(rd.LabelUser)
		if label == "" {
			label = cString128(rd.LabelOrig)
		}
		l := strings.ToLower(label)
		score := 0
		switch {
		case strings.Contains(l, "cpu package"):
			score = 100
		case strings.Contains(l, "tctl") || strings.Contains(l, "tdie"):
			score = 95
		case strings.Contains(l, "cpu average") || strings.Contains(l, "cpu (tctl"):
			score = 90
		case strings.Contains(l, "core max") || strings.Contains(l, "cpu max"):
			score = 85
		case strings.HasPrefix(l, "cpu") && strings.Contains(l, "temp"):
			score = 70
		case strings.Contains(l, "cpu") && !strings.Contains(l, "gpu"):
			score = 50
		default:
			continue
		}
		if score > best.score {
			best = cand{score: score, temp: t, name: label}
		}
	}
	if best.score == 0 {
		hwinfoLastRead = externalTempReading{}
		return hwinfoLastRead
	}
	hwinfoLastRead = externalTempReading{
		TempC: best.temp, OK: true, Source: "hwinfo",
		LabelRu: "температура из HWiNFO", LabelEn: "temperature from HWiNFO",
		SensorName: best.name,
	}
	return hwinfoLastRead
}
