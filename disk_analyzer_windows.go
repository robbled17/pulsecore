//go:build windows

package main

import (
	"math"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modKernel32Disk             = windows.NewLazySystemDLL("kernel32.dll")
	procGetLogicalDriveStringsW = modKernel32Disk.NewProc("GetLogicalDriveStringsW")
	procGetDriveTypeW           = modKernel32Disk.NewProc("GetDriveTypeW")
	procGetVolumeInformationW   = modKernel32Disk.NewProc("GetVolumeInformationW")
	procGetDiskFreeSpaceExWDisk = modKernel32Disk.NewProc("GetDiskFreeSpaceExW")
)

const (
	driveFixed     = 3
	driveRemovable = 2
)

func getLocalDrivesNative() []LocalDriveInfo {
	var buf [512]uint16
	r, _, _ := procGetLogicalDriveStringsW.Call(uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	if r == 0 {
		return defaultDriveC()
	}

	var roots []string
	start := 0
	for i := 0; i < int(r); i++ {
		if buf[i] == 0 {
			if i > start {
				roots = append(roots, windows.UTF16ToString(buf[start:i]))
			}
			start = i + 1
		}
	}

	var out []LocalDriveInfo
	for _, root := range roots {
		rootPtr, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		dt, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(rootPtr)))
		if dt != driveFixed && dt != driveRemovable {
			continue
		}

		var freeBytes, totalBytes, totalFree uint64
		r1, _, _ := procGetDiskFreeSpaceExWDisk.Call(
			uintptr(unsafe.Pointer(rootPtr)),
			uintptr(unsafe.Pointer(&freeBytes)),
			uintptr(unsafe.Pointer(&totalBytes)),
			uintptr(unsafe.Pointer(&totalFree)),
		)
		if r1 == 0 || totalBytes == 0 {
			continue
		}

		var volName [256]uint16
		_, _, _ = procGetVolumeInformationW.Call(
			uintptr(unsafe.Pointer(rootPtr)),
			uintptr(unsafe.Pointer(&volName[0])),
			256,
			0, 0, 0, 0, 0,
		)
		label := windows.UTF16ToString(volName[:])
		if label == "" {
			label = "Локальный диск"
		}

		usedBytes := totalBytes - freeBytes
		totalGB := math.Round(float64(totalBytes)/(1024*1024*1024)*10) / 10
		freeGB := math.Round(float64(freeBytes)/(1024*1024*1024)*10) / 10
		usedGB := math.Round(float64(usedBytes)/(1024*1024*1024)*10) / 10
		usedPercent := math.Round(float64(usedBytes)/float64(totalBytes)*1000) / 10

		letter := strings.ToUpper(strings.TrimSuffix(strings.TrimSuffix(root, `\`), `:`))

		out = append(out, LocalDriveInfo{
			Letter:      letter,
			Mount:       root,
			Label:       label,
			DriveType:   "Fixed",
			TotalBytes:  totalBytes,
			FreeBytes:   freeBytes,
			UsedBytes:   usedBytes,
			TotalGB:     totalGB,
			FreeGB:      freeGB,
			UsedGB:      usedGB,
			UsedPercent: usedPercent,
		})
	}

	if len(out) == 0 {
		return defaultDriveC()
	}
	return out
}

func defaultDriveC() []LocalDriveInfo {
	return []LocalDriveInfo{
		{
			Letter:      "C",
			Mount:       `C:\`,
			Label:       "Локальный диск",
			DriveType:   "Fixed",
			TotalBytes:  500 * 1024 * 1024 * 1024,
			FreeBytes:   200 * 1024 * 1024 * 1024,
			UsedBytes:   300 * 1024 * 1024 * 1024,
			TotalGB:     500,
			FreeGB:      200,
			UsedGB:      300,
			UsedPercent: 60,
		},
	}
}
