package main

import (
	_ "embed"
	"encoding/json"
	"sync"
)

//go:embed hardware_db.json
var embeddedHardwareDBJSON []byte

type GPUEntry struct {
	Name   string  `json:"name"`
	Score  float64 `json:"score"`
	VRAMGB float64 `json:"vramGB"`
	Type   string  `json:"type"` // "desktop", "laptop", "igpu"
}

type CPUEntry struct {
	Name    string  `json:"name"`
	Score   float64 `json:"score"`
	Cores   int     `json:"cores"`
	Threads int     `json:"threads"`
}

type HardwareDatabase struct {
	GPUs map[string]GPUEntry `json:"gpus"`
	CPUs map[string]CPUEntry `json:"cpus"`
}

var (
	hwDBInstance *HardwareDatabase
	hwDBOnce     sync.Once
)

// GetHardwareDB returns the singleton reference hardware database loaded from embedded JSON.
func GetHardwareDB() *HardwareDatabase {
	hwDBOnce.Do(func() {
		hwDBInstance = &HardwareDatabase{
			GPUs: make(map[string]GPUEntry),
			CPUs: make(map[string]CPUEntry),
		}
		if len(embeddedHardwareDBJSON) > 0 {
			_ = json.Unmarshal(embeddedHardwareDBJSON, hwDBInstance)
		}
	})
	return hwDBInstance
}
