package main

import (
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DeepCleanupItem is one large system reclaim option.
type DeepCleanupItem struct {
	ID           string `json:"id"`
	SizeBytes    int64  `json:"sizeBytes"`
	Available    bool   `json:"available"`
	NeedsAdmin   bool   `json:"needsAdmin"`
	Irreversible bool   `json:"irreversible"`
	Enabled      bool   `json:"enabled"` // current state (e.g. hibernation on)
}

// DeepCleanupPreview aggregates heavy system reclaim targets.
type DeepCleanupPreview struct {
	Items              []DeepCleanupItem `json:"items"`
	HibernationOn      bool              `json:"hibernationOn"`
	HibernationBytes   int64             `json:"hibernationBytes"`
	ShadowBytes        int64             `json:"shadowBytes"`
	DeliveryOptBytes   int64             `json:"deliveryOptBytes"`
	SystemDriveUsedPct float64           `json:"systemDriveUsedPct"`
	SystemDriveFreeGB  float64           `json:"systemDriveFreeGB"`
}

// DeepCleanupResult reports bytes actually reclaimed.
type DeepCleanupResult struct {
	OK                 bool     `json:"ok"`
	FreedBytes         int64    `json:"freedBytes"`
	ActualCleanedBytes int64    `json:"actualCleanedBytes"`
	Actions            []string `json:"actions"`
	Message            string   `json:"message"`
}

var (
	modKernel32Deep           = windows.NewLazySystemDLL("kernel32.dll")
	procGetDiskFreeSpaceExW   = modKernel32Deep.NewProc("GetDiskFreeSpaceExW")
)

// PreviewDeepCleanup measures hibernation / VSS / Delivery Optimization.
func (a *App) PreviewDeepCleanup() DeepCleanupPreview {
	hibOn, hibBytes := hibernationStatus()
	shadow := estimateShadowStorageBytes()
	delivery := deliveryOptimizationSize()
	usedPct, freeGB := systemDriveSpaceEx()
	items := []DeepCleanupItem{
		{
			ID: "hibernation", SizeBytes: hibBytes, Available: hibOn && hibBytes > 0,
			NeedsAdmin: true, Irreversible: false, Enabled: hibOn,
		},
		{
			ID: "vss_oldest", SizeBytes: shadow, Available: shadow > 64*1024*1024,
			NeedsAdmin: true, Irreversible: true, Enabled: shadow > 0,
		},
		{
			ID: "delivery_opt", SizeBytes: delivery, Available: delivery > 0,
			NeedsAdmin: true, Irreversible: false, Enabled: delivery > 0,
		},
	}
	return DeepCleanupPreview{
		Items: items, HibernationOn: hibOn, HibernationBytes: hibBytes,
		ShadowBytes: shadow, DeliveryOptBytes: delivery,
		SystemDriveUsedPct: usedPct, SystemDriveFreeGB: freeGB,
	}
}

// EnableHibernation runs `powercfg -h on` and refreshes disk metrics.
func (a *App) EnableHibernation() DeepCleanupResult {
	_, rec := a.MeasureAndRecordClean("C:", "HibernationOn", func() int64 {
		_ = runHiddenCommand("powercfg", "-h", "on")
		time.Sleep(500 * time.Millisecond)
		return 0
	})
	a.RefreshDiskMetricsEx()
	on, _ := hibernationStatus()
	return DeepCleanupResult{
		OK: on, FreedBytes: int64(rec.BytesFreed), ActualCleanedBytes: 0,
		Actions: []string{"hibernation_on"},
	}
}

// DisableHibernation runs `powercfg -h off` and refreshes disk metrics.
func (a *App) DisableHibernation() DeepCleanupResult {
	beforeOn, beforeBytes := hibernationStatus()
	if sz := hiberfilSize(); sz > beforeBytes {
		beforeBytes = sz
	}
	var reported int64
	_, rec := a.MeasureAndRecordClean("C:", "Hibernation", func() int64 {
		err := runHiddenCommand("powercfg", "-h", "off")
		time.Sleep(500 * time.Millisecond)
		on, afterBytes := hibernationStatus()
		freed := beforeBytes - afterBytes
		if freed < 0 {
			freed = 0
		}
		if beforeOn && !on && freed == 0 && beforeBytes > 0 {
			freed = beforeBytes
		}
		if err != nil && freed == 0 {
			return 0
		}
		reported = freed
		return freed
	})
	a.RefreshDiskMetricsEx()
	on, _ := hibernationStatus()
	freed := int64(rec.BytesFreed)
	if freed <= 0 {
		freed = reported
	}
	return DeepCleanupResult{
		OK: !on || freed > 0, FreedBytes: freed, ActualCleanedBytes: freed,
		Actions: []string{"hibernation"},
	}
}

// CleanOldestShadow deletes the oldest VSS shadow on C: (keeps newer points).
func (a *App) CleanOldestShadow() DeepCleanupResult {
	before := estimateShadowStorageBytes()
	var reported int64
	_, rec := a.MeasureAndRecordClean("C:", "VSS", func() int64 {
		_ = runHiddenCommand("vssadmin", "delete", "shadows", "/for=c:", "/oldest")
		time.Sleep(800 * time.Millisecond)
		after := estimateShadowStorageBytes()
		freed := before - after
		if freed < 0 {
			freed = 0
		}
		reported = freed
		return freed
	})
	a.RefreshDiskMetricsEx()
	freed := int64(rec.BytesFreed)
	if freed <= 0 {
		freed = reported
	}
	return DeepCleanupResult{
		OK: freed > 0, FreedBytes: freed, ActualCleanedBytes: freed,
		Actions: []string{"vss_oldest"},
	}
}

// CleanDeliveryOptimization clears SoftwareDistribution\DeliveryOptimization cache.
func (a *App) CleanDeliveryOptimization() DeepCleanupResult {
	dir := `C:\Windows\SoftwareDistribution\DeliveryOptimization`
	before := dirSizeBudget(dir, time.Now().Add(1200*time.Millisecond), 5)
	var reported int64
	_, rec := a.MeasureAndRecordClean("C:", "DeliveryOpt", func() int64 {
		freed := cleanDeliveryOptimizationDir(dir)
		if freed <= 0 && before > 0 {
			after := dirSizeBudget(dir, time.Now().Add(600*time.Millisecond), 4)
			if before > after {
				freed = before - after
			}
		}
		reported = freed
		return freed
	})
	a.RefreshDiskMetricsEx()
	freed := int64(rec.BytesFreed)
	if freed <= 0 {
		freed = reported
	}
	return DeepCleanupResult{
		OK: freed > 0, FreedBytes: freed, ActualCleanedBytes: freed,
		Actions: []string{"delivery_opt"},
	}
}

func cleanDeliveryOptimizationDir(dir string) int64 {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return 0
	}
	return walkClean(dir, 6, time.Time{}, true, nil)
}

func hibernationStatus() (on bool, sizeBytes int64) {
	sz := hiberfilSize()
	if sz > 0 {
		return true, sz
	}
	// powercfg /a lists available sleep states; absence of hibernate file ≈ off.
	out, err := runHiddenCombinedOutput("powercfg", "/a")
	if err == nil {
		s := strings.ToLower(string(out))
		if strings.Contains(s, "hibernate has not been enabled") ||
			strings.Contains(s, "гибернация не включена") ||
			strings.Contains(s, "гибернация недоступна") {
			return false, 0
		}
	}
	return false, 0
}

func hiberfilSize() int64 {
	p := `C:\hiberfil.sys`
	info, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return info.Size()
}

func estimateShadowStorageBytes() int64 {
	out, err := runHiddenCombinedOutput("vssadmin", "list", "shadowstorage")
	if err != nil {
		return 0
	}
	return parseVSSUsedBytes(string(out))
}

func parseVSSUsedBytes(text string) int64 {
	// English: "Used Shadow Copy Storage space: 1.23 GB (1,234,567,890 bytes)"
	// Russian locales may vary; prefer parenthesized byte count.
	var total int64
	lower := strings.ToLower(text)
	lines := strings.Split(lower, "\n")
	for _, line := range lines {
		if !strings.Contains(line, "used") && !strings.Contains(line, "использу") {
			continue
		}
		if i := strings.LastIndex(line, "("); i >= 0 {
			chunk := line[i+1:]
			if j := strings.Index(chunk, "byte"); j >= 0 {
				chunk = chunk[:j]
			}
			if j := strings.Index(chunk, "байт"); j >= 0 {
				chunk = chunk[:j]
			}
			chunk = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(chunk), ")"))
			chunk = strings.ReplaceAll(chunk, ",", "")
			chunk = strings.ReplaceAll(chunk, " ", "")
			if n, err := strconv.ParseInt(chunk, 10, 64); err == nil && n > 0 {
				total += n
			}
		}
	}
	return total
}

func deliveryOptimizationSize() int64 {
	return dirSizeBudget(`C:\Windows\SoftwareDistribution\DeliveryOptimization`, time.Now().Add(900*time.Millisecond), 4)
}

// systemDriveSpaceEx uses GetDiskFreeSpaceExW for C:\ and returns used% + free GB.
func systemDriveSpaceEx() (usedPct float64, freeGB float64) {
	free, total, ok := getDiskFreeSpaceEx(`C:\`)
	if !ok || total == 0 {
		return 0, 0
	}
	used := total - free
	usedPct = float64(used) * 100 / float64(total)
	freeGB = float64(free) / (1024 * 1024 * 1024)
	return usedPct, freeGB
}

func getDiskFreeSpaceEx(root string) (freeBytes, totalBytes uint64, ok bool) {
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return 0, 0, false
	}
	var free, total, totalFree uint64
	r1, _, _ := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&free)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if r1 == 0 {
		return 0, 0, false
	}
	return free, total, true
}

// RefreshDiskMetricsEx forces volume cache refresh using GetDiskFreeSpaceEx for C:.
func (a *App) RefreshDiskMetricsEx() []DiskMetrics {
	disks := a.RefreshDiskMetrics()
	free, total, ok := getDiskFreeSpaceEx(`C:\`)
	if !ok || total == 0 {
		return disks
	}
	usedPct := float64(total-free) * 100 / float64(total)
	freeGB := float64(free) / (1024 * 1024 * 1024)
	usedGB := float64(total-free) / (1024 * 1024 * 1024)
	totalGB := float64(total) / (1024 * 1024 * 1024)
	a.mu.Lock()
	defer a.mu.Unlock()
	patch := func(list []DiskMetrics) []DiskMetrics {
		out := append([]DiskMetrics(nil), list...)
		for i := range out {
			m := strings.ToUpper(out[i].Mount)
			if strings.HasPrefix(m, "C:") {
				out[i].Percent = usedPct
				out[i].FreeGB = freeGB
				out[i].UsedGB = usedGB
				out[i].TotalGB = totalGB
			}
		}
		return out
	}
	a.cachedDisks = patch(a.cachedDisks)
	a.metrics.Disks = patch(a.metrics.Disks)
	return a.metrics.Disks
}

func deepErrString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
