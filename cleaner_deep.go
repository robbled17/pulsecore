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
	modKernel32Deep         = windows.NewLazySystemDLL("kernel32.dll")
	procGetDiskFreeSpaceExW = modKernel32Deep.NewProc("GetDiskFreeSpaceExW")
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
		_, _ = runElevatedPowerShell("vssadmin delete shadows /for=c: /oldest /quiet")
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
		OK: freed > 0 || before > 0, FreedBytes: freed, ActualCleanedBytes: freed,
		Actions: []string{"vss_oldest"},
	}
}

// RecycleBinStatus is the live Windows recycle bin size from SHQueryRecycleBinW.
type RecycleBinStatus struct {
	Bytes int64 `json:"bytes"`
	Items int64 `json:"items"`
}

// GetRecycleBinInfo returns the exact recycle-bin size for the leftovers tab.
func (a *App) GetRecycleBinInfo() RecycleBinStatus {
	b, items, err := queryRecycleBinInfo()
	if err != nil || b < 0 {
		b = recycleBinSizeApprox()
		if b < 0 {
			b = 0
		}
		items = 0
	}
	return RecycleBinStatus{Bytes: b, Items: items}
}

// EmptyRecycleBin clears every drive bin and records the pre-delete size.
func (a *App) EmptyRecycleBin() DeepCleanupResult {
	size, items, _ := queryRecycleBinInfo()
	if size <= 0 || items <= 0 {
		return DeepCleanupResult{
			OK: true, FreedBytes: 0, ActualCleanedBytes: 0,
			Actions: []string{"recycle"}, Message: "ok",
		}
	}
	before := size
	var reported int64
	_, rec := a.MeasureAndRecordClean("C:", "Recycle", func() int64 {
		_ = emptyRecycleBinNative()
		reported = before
		return reported
	})
	freed := int64(rec.DeletedBytes)
	if freed <= 0 {
		freed = int64(rec.BytesFreed)
	}
	if freed <= 0 {
		freed = reported
	}
	return DeepCleanupResult{
		OK: true, FreedBytes: freed, ActualCleanedBytes: freed,
		Actions: []string{"recycle"}, Message: "ok",
	}
}

// CreateRestorePoint runs Checkpoint-Computer with a hidden window and elevation.
func (a *App) CreateRestorePoint() DeepCleanupResult {
	script := `Enable-ComputerRestore -Drive "C:\" -ErrorAction SilentlyContinue
Checkpoint-Computer -Description "PulseCore Backup" -RestorePointType "MODIFY_SETTINGS" -ErrorAction Stop`

	text, err := runElevatedPowerShell(script)
	text = strings.TrimSpace(text)
	low := strings.ToLower(text)
	if err != nil && text == "" {
		text = err.Error()
		low = strings.ToLower(text)
	}
	failed := err != nil || strings.Contains(low, "failed") || strings.Contains(low, "ошиб") || strings.Contains(low, "exception") || strings.Contains(low, "cannot")
	if !failed {
		return DeepCleanupResult{OK: true, Message: "ok", Actions: []string{"restore_point"}}
	}
	if strings.Contains(low, "1440") || strings.Contains(low, "24 hour") || strings.Contains(low, "24 час") ||
		strings.Contains(low, "already been created") || strings.Contains(low, "0x80042306") ||
		strings.Contains(low, "frequency") || strings.Contains(low, "не чаще") ||
		strings.Contains(low, "eventid 101") || strings.Contains(low, "restorepoint") {
		return DeepCleanupResult{OK: false, Message: "restore_point_24h", Actions: []string{"restore_point"}}
	}
	if strings.Contains(low, "access") || strings.Contains(low, "denied") || strings.Contains(low, "0x80070005") ||
		strings.Contains(low, "administrator") || strings.Contains(low, "администрат") || strings.Contains(low, "0x80041003") ||
		strings.Contains(low, "canceled") || strings.Contains(low, "cancelled") {
		return DeepCleanupResult{OK: false, Message: "restore_point_admin", Actions: []string{"restore_point"}}
	}
	if text == "" {
		text = "restore_point_failed"
	}
	return DeepCleanupResult{OK: false, Message: text, Actions: []string{"restore_point"}}
}

// CleanDeliveryOptimization clears SoftwareDistribution\DeliveryOptimization cache.
func (a *App) CleanDeliveryOptimization() DeepCleanupResult {
	dir := `C:\Windows\SoftwareDistribution\DeliveryOptimization`
	before := dirSizeBudget(dir, time.Now().Add(1200*time.Millisecond), 5)
	var reported int64
	_, rec := a.MeasureAndRecordClean("C:", "DeliveryOpt", func() int64 {
		psScript := `Stop-Service -Name dosvc -Force -ErrorAction SilentlyContinue
Remove-Item -Path "$env:SystemRoot\SoftwareDistribution\DeliveryOptimization\Cache\*" -Recurse -Force -ErrorAction SilentlyContinue
Delete-DeliveryOptimizationCache -Force -ErrorAction SilentlyContinue
Start-Service -Name dosvc -ErrorAction SilentlyContinue`
		_, _ = runElevatedPowerShell(psScript)
		time.Sleep(500 * time.Millisecond)
		after := dirSizeBudget(dir, time.Now().Add(600*time.Millisecond), 4)
		freed := before - after
		if freed <= 0 && before > 0 {
			freed = cleanDeliveryOptimizationDir(dir)
		}
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
		OK: freed > 0 || before > 0, FreedBytes: freed, ActualCleanedBytes: freed,
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
