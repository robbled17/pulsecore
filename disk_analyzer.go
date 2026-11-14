package main

// LocalDriveInfo represents a local drive partition.
type LocalDriveInfo struct {
	Letter      string  `json:"letter"`      // e.g. "C"
	Mount       string  `json:"mount"`       // e.g. "C:\"
	Label       string  `json:"label"`       // Volume label
	DriveType   string  `json:"driveType"`   // e.g. "SSD", "HDD", "Fixed"
	TotalBytes  uint64  `json:"totalBytes"`
	FreeBytes   uint64  `json:"freeBytes"`
	UsedBytes   uint64  `json:"usedBytes"`
	TotalGB     float64 `json:"totalGB"`
	FreeGB      float64 `json:"freeGB"`
	UsedGB      float64 `json:"usedGB"`
	UsedPercent float64 `json:"usedPercent"`
}

// GetLocalDrives returns all active fixed/local storage drives.
func (a *App) GetLocalDrives() []LocalDriveInfo {
	return getLocalDrivesNative()
}
