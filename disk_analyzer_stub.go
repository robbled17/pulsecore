//go:build !windows

package main

func getLocalDrivesNative() []LocalDriveInfo {
	return []LocalDriveInfo{
		{
			Letter:      "C",
			Mount:       "/",
			Label:       "Root",
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
