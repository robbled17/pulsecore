package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/windows/registry"
)

// StartupItem is one autostart entry from registry or Startup folder.
type StartupItem struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Command          string  `json:"command"`
	Path             string  `json:"path"`
	Publisher        string  `json:"publisher"`
	Location         string  `json:"location"` // hkcu_run|hklm_run|startup_user|startup_common
	LocationRu       string  `json:"locationRu"`
	LocationEn       string  `json:"locationEn"`
	Enabled          bool    `json:"enabled"`
	CanDisable       bool    `json:"canDisable"`
	Critical         bool    `json:"critical"`
	Impact           string  `json:"impact,omitempty"` // high|medium|low — omit when unknown
	ImpactRu         string  `json:"impactRu,omitempty"`
	ImpactEn         string  `json:"impactEn,omitempty"`
	DetailRu         string  `json:"detailRu,omitempty"`
	DetailEn         string  `json:"detailEn,omitempty"`
	Advise           string  `json:"advise"` // startupAdviseDisable|startupAdviseKeep
	Category         string  `json:"category,omitempty"` // launcher|cloud|messenger|antivirus|driver
	CategoryKey      string  `json:"categoryKey,omitempty"`
	CategoryEstimate bool    `json:"categoryEstimate,omitempty"`
	Running          bool    `json:"running,omitempty"`
	CPUPercent       float64 `json:"cpuPercent,omitempty"`
	RAMMB            float64 `json:"ramMB,omitempty"`
	ParseConfident   bool    `json:"-"`
	RegValue         string  `json:"-"`
	FolderFile       string  `json:"-"`
}

// StartupDisableResult reports a confirmed disable/enable attempt.
type StartupDisableResult struct {
	OK        bool   `json:"ok"`
	MessageRu string `json:"messageRu"`
	MessageEn string `json:"messageEn"`
}

type startupCache struct {
	mu          sync.Mutex
	items       []StartupItem
	broken      []StartupItem
	brokenCount int
	at          time.Time
}

// Local category table — estimate only (not measured by file size).
var startupCategoryTable = []struct {
	Hints    []string
	Category string
	Key      string
	Impact   string
}{
	{[]string{"steam", "epicgameslauncher", "epicgames", "origin", "eadesktop", "ubisoftconnect", "battlenet", "gog galaxy", "riotclient"}, "launcher", "startupCatLauncher", "high"},
	{[]string{"dropbox", "googledrive", "google drive", "onedrive", "box.exe", "megasync", "icloud"}, "cloud", "startupCatCloud", "medium"},
	{[]string{"discord", "telegram", "skype", "slack", "teams", "zoom", "whatsapp", "viber"}, "messenger", "startupCatMessenger", "medium"},
	{[]string{"avast", "avg", "kaspersky", "norton", "mcafee", "bitdefender", "eset", "malwarebytes"}, "antivirus", "startupCatAntivirus", "high"},
	{[]string{"nvidia", "amd", "razer", "logitech", "corsair", "icue", "nzxt", "realtek", "intel"}, "driver", "startupCatDriver", "low"},
}

func (a *App) ensureStartupCache() *startupCache {
	if a.startupScan == nil {
		a.startupScan = &startupCache{}
	}
	return a.startupScan
}

// Protected system entries — hidden from the UI (Task Manager also locks these).
var criticalStartupNames = map[string]bool{
	"securityhealth": true, "windows defender": true, "windows security": true,
	"securityhealthsystray": true, "ctfmon": true,
}

var criticalPathHints = []string{
	`securityhealthsystray`, `msmpeng`, `smartscreen`,
}

var highImpactHints = []string{
	"steam", "epicgames", "discord", "spotify", "adobe", "ccxprocess",
	"teams", "skype", "zoom", "dropbox", "googledrive", "itunes",
	"utorrent", "qbittorrent", "corsair", "icue", "nzxt", "razer",
	"logitech", "nvidia", "amd", "overwolf", "wallpaper",
}

var mediumImpactHints = []string{
	"chrome", "msedge", "firefox", "opera", "brave", "yandex",
	"telegram", "slack", "notion", "obs64", "obs32",
}

// GetStartupItems scans Run keys and Startup folders (no admin required for HKCU / user folder).
func (a *App) GetStartupItems() []StartupItem {
	raw := make([]StartupItem, 0, 32)
	raw = append(raw, scanRunKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, "hkcu_run", true)...)
	raw = append(raw, scanDisabledHKCURun()...)
	raw = append(raw, scanRunKey(registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Run`, "hklm_run", false)...)
	raw = append(raw, scanRunKey(registry.LOCAL_MACHINE, `Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`, "hklm_run", false)...)

	if userStart := startupFolder(false); userStart != "" {
		raw = append(raw, scanStartupFolder(userStart, "startup_user", true)...)
	}
	if commonStart := startupFolder(true); commonStart != "" {
		raw = append(raw, scanStartupFolder(commonStart, "startup_common", false)...)
	}

	broken := make([]StartupItem, 0)
	kept := make([]StartupItem, 0, len(raw))
	for i := range raw {
		it := raw[i]
		resolveStartupPath(&it)
		applyStartupApproved(&it)
		enrichStartupItem(&it)
		attachStartupLiveMetrics(&it)
		if it.Critical {
			continue // hide protected system entries entirely
		}
		if it.ParseConfident && it.Path != "" {
			if _, err := os.Stat(it.Path); err != nil {
				broken = append(broken, it)
				continue
			}
		}
		kept = append(kept, it)
	}
	kept = mergeStartupDuplicates(kept)

	c := a.ensureStartupCache()
	c.mu.Lock()
	c.items = kept
	c.broken = broken
	c.brokenCount = len(broken)
	c.at = time.Now()
	c.mu.Unlock()
	return kept
}

// GetStartupBrokenCount returns how many confidently-broken entries were filtered out.
func (a *App) GetStartupBrokenCount() int {
	c := a.ensureStartupCache()
	c.mu.Lock()
	empty := c.at.IsZero()
	c.mu.Unlock()
	if empty {
		_ = a.GetStartupItems()
	}
	c.mu.Lock()
	n := c.brokenCount
	c.mu.Unlock()
	return n
}

// RemoveBrokenStartupEntries deletes broken HKCU Run / user-folder entries (values stashed for undo).
func (a *App) RemoveBrokenStartupEntries() StartupDisableResult {
	_ = a.GetStartupItems()
	c := a.ensureStartupCache()
	c.mu.Lock()
	broken := append([]StartupItem(nil), c.broken...)
	c.mu.Unlock()
	if len(broken) == 0 {
		return StartupDisableResult{OK: true, MessageRu: "Битых записей нет.", MessageEn: "No broken entries."}
	}
	removed := 0
	for _, it := range broken {
		switch it.Location {
		case "hkcu_run":
			if err := setHKCURunEnabled(it.RegValue, it.Command, false); err == nil {
				removed++
			}
		case "startup_user":
			if it.FolderFile != "" {
				if err := os.Remove(it.FolderFile); err == nil {
					removed++
				}
			}
		}
	}
	a.GetStartupItems()
	return StartupDisableResult{
		OK:        removed > 0,
		MessageRu: fmt.Sprintf("Удалено битых записей: %d.", removed),
		MessageEn: fmt.Sprintf("Removed broken entries: %d.", removed),
	}
}

func scanDisabledHKCURun() []StartupItem {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\PulseCore\DisabledRun`, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	names, err := k.ReadValueNames(0)
	if err != nil {
		return nil
	}
	out := make([]StartupItem, 0, len(names))
	for _, name := range names {
		val, _, err := k.GetStringValue(name)
		if err != nil || strings.TrimSpace(val) == "" {
			continue
		}
		out = append(out, StartupItem{
			ID: "hkcu_run|" + strings.ToLower(name), Name: name, Command: val, Path: extractExePath(val),
			Location: "hkcu_run", Enabled: false, CanDisable: true, RegValue: name,
		})
	}
	return out
}

func scanRunKey(root registry.Key, path, loc string, canDisable bool) []StartupItem {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	names, err := k.ReadValueNames(0)
	if err != nil {
		return nil
	}
	out := make([]StartupItem, 0, len(names))
	for _, name := range names {
		val, _, err := k.GetStringValue(name)
		if err != nil || strings.TrimSpace(val) == "" {
			continue
		}
		id := loc + "|" + strings.ToLower(name)
		out = append(out, StartupItem{
			ID: id, Name: name, Command: val, Path: extractExePath(val),
			Location: loc, Enabled: true, CanDisable: canDisable, RegValue: name,
		})
	}
	return out
}

func scanStartupFolder(dir, loc string, canDisable bool) []StartupItem {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]StartupItem, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if lower == "desktop.ini" {
			continue
		}
		enabled := true
		if strings.HasSuffix(lower, ".disabled") {
			enabled = false
		}
		if !strings.HasSuffix(lower, ".lnk") && !strings.HasSuffix(lower, ".lnk.disabled") &&
			!strings.HasSuffix(lower, ".url") && !strings.HasSuffix(lower, ".exe") {
			continue
		}
		full := filepath.Join(dir, name)
		display := strings.TrimSuffix(name, ".disabled")
		display = strings.TrimSuffix(display, filepath.Ext(display))
		id := loc + "|" + strings.ToLower(name)
		out = append(out, StartupItem{
			ID: id, Name: display, Command: full, Path: full,
			Location: loc, Enabled: enabled, CanDisable: canDisable && enabled || (canDisable && !enabled),
			FolderFile: full,
		})
		_ = canDisable
	}
	// Fix CanDisable for folder items: user folder always (enable/disable), common never
	for i := range out {
		out[i].CanDisable = canDisable
	}
	return out
}

func startupFolder(common bool) string {
	if common {
		return filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs\Startup`)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, `AppData\Roaming\Microsoft\Windows\Start Menu\Programs\Startup`)
}

func extractExePath(cmd string) string {
	path, _ := parseStartupCommand(cmd)
	return path
}

// parseStartupCommand extracts the executable path and whether parsing was confident.
func parseStartupCommand(cmd string) (path string, confident bool) {
	s := strings.TrimSpace(cmd)
	if s == "" {
		return "", false
	}
	s = expandEnvBestEffort(s)
	if strings.HasPrefix(s, `"`) {
		if i := strings.Index(s[1:], `"`); i >= 0 {
			return s[1 : i+1], true
		}
		return "", false // unbalanced quotes — uncertain
	}
	// Path with spaces before first switch: take until " /" or " -" after .exe
	lower := strings.ToLower(s)
	if idx := strings.Index(lower, ".exe"); idx >= 0 {
		return s[:idx+4], true
	}
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return s, false
	}
	// Bare relative name — may need App Paths; mark uncertain until resolved.
	if !strings.Contains(parts[0], `\`) && !strings.Contains(parts[0], `/`) {
		return parts[0], false
	}
	return parts[0], true
}

func expandEnvBestEffort(s string) string {
	return os.Expand(s, func(key string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return "%" + key + "%"
	})
}

func resolveAppPaths(name string) string {
	base := filepath.Base(name)
	if base == "" {
		return ""
	}
	keys := []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\` + base,
		`SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths\` + base,
	}
	for _, path := range keys {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		val, _, err := k.GetStringValue("")
		k.Close()
		if err == nil && strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

func resolveStartupPath(it *StartupItem) {
	path, ok := parseStartupCommand(it.Command)
	it.ParseConfident = ok
	if path == "" {
		path = it.Path
	}
	path = expandEnvBestEffort(path)
	if path != "" && !filepath.IsAbs(path) {
		if ap := resolveAppPaths(path); ap != "" {
			path = ap
			it.ParseConfident = true
		}
	}
	if path != "" {
		it.Path = path
	}
}

func applyStartupApproved(it *StartupItem) {
	if it.Location != "hkcu_run" && it.Location != "hklm_run" {
		return
	}
	name := it.RegValue
	if name == "" {
		name = it.Name
	}
	hive := registry.CURRENT_USER
	if it.Location == "hklm_run" {
		hive = registry.LOCAL_MACHINE
	}
	k, err := registry.OpenKey(hive, `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	raw, _, err := k.GetBinaryValue(name)
	if err != nil || len(raw) < 1 {
		return
	}
	// 0x02 = enabled, 0x03 = disabled (Task Manager convention).
	switch raw[0] {
	case 0x03:
		it.Enabled = false
	case 0x02:
		it.Enabled = true
	}
}

func setStartupApproved(hive registry.Key, valueName string, enabled bool) error {
	k, _, err := registry.CreateKey(hive, `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	buf := make([]byte, 12)
	if enabled {
		buf[0] = 0x02
	} else {
		buf[0] = 0x03
	}
	return k.SetBinaryValue(valueName, buf)
}

func enrichStartupItem(it *StartupItem) {
	switch it.Location {
	case "hkcu_run":
		it.LocationRu, it.LocationEn = "Реестр (текущий пользователь)", "Registry (current user)"
	case "hklm_run":
		it.LocationRu, it.LocationEn = "Реестр (все пользователи)", "Registry (all users)"
		it.CanDisable = false
		it.DetailRu = "Изменение HKLM требует прав администратора - PulseCore только показывает запись."
		it.DetailEn = "Changing HKLM needs admin rights - PulseCore only lists the entry."
	case "startup_user":
		it.LocationRu, it.LocationEn = "Папка автозагрузки (пользователь)", "Startup folder (user)"
	case "startup_common":
		it.LocationRu, it.LocationEn = "Папка автозагрузки (общая)", "Startup folder (common)"
		it.CanDisable = false
	}

	base := strings.ToLower(filepath.Base(it.Path))
	nameL := strings.ToLower(it.Name)
	cmdL := strings.ToLower(it.Command)

	it.Critical = isCriticalStartup(nameL, base, cmdL)
	if it.Critical {
		it.CanDisable = false
		it.Advise = "startupAdviseKeep"
		return
	}

	cat, key, impact := classifyStartupCategory(nameL + " " + base + " " + cmdL)
	if cat != "" {
		it.Category = cat
		it.CategoryKey = key
		it.CategoryEstimate = true
		it.Impact = impact
		switch impact {
		case "high":
			it.ImpactRu, it.ImpactEn = "Высокое", "High"
			it.Advise = "startupAdviseDisable"
		case "medium":
			it.ImpactRu, it.ImpactEn = "Среднее", "Medium"
			it.Advise = "startupAdviseDisable"
		default:
			it.ImpactRu, it.ImpactEn = "Низкое", "Low"
			it.Advise = "startupAdviseKeep"
		}
	} else {
		// Unknown: omit impact fields (no "insufficient data" copy).
		it.Impact = ""
		it.ImpactRu, it.ImpactEn = "", ""
		it.Advise = "startupAdviseKeep"
	}

	if pub := guessPublisher(it.Path, base); pub != "" {
		it.Publisher = pub
	}
}

func classifyStartupCategory(blob string) (category, key, impact string) {
	blob = strings.ToLower(blob)
	for _, row := range startupCategoryTable {
		for _, h := range row.Hints {
			if strings.Contains(blob, h) {
				return row.Category, row.Key, row.Impact
			}
		}
	}
	return "", "", ""
}

func mergeStartupDuplicates(items []StartupItem) []StartupItem {
	index := map[string]int{}
	out := make([]StartupItem, 0, len(items))
	for _, it := range items {
		key := strings.ToLower(filepath.Base(it.Path))
		if key == "" || key == "." {
			key = strings.ToLower(it.Name)
		}
		if i, ok := index[key]; ok {
			cur := out[i]
			// Prefer enabled + disableable user entry.
			if it.Enabled && !cur.Enabled {
				cur.Enabled = true
			}
			if it.CanDisable && !cur.CanDisable {
				cur.CanDisable = true
				cur.Location = it.Location
				cur.RegValue = it.RegValue
				cur.FolderFile = it.FolderFile
				cur.ID = it.ID
			}
			if cur.Category == "" && it.Category != "" {
				cur.Category = it.Category
				cur.CategoryKey = it.CategoryKey
				cur.CategoryEstimate = it.CategoryEstimate
				cur.Impact = it.Impact
				cur.ImpactRu, cur.ImpactEn = it.ImpactRu, it.ImpactEn
			}
			out[i] = cur
			continue
		}
		index[key] = len(out)
		out = append(out, it)
	}
	return out
}

func attachStartupLiveMetrics(it *StartupItem) {
	base := strings.ToLower(filepath.Base(it.Path))
	if base == "" {
		return
	}
	// Best-effort: match running process by exe name via existing tops if available.
	// Keep silent on failure — field simply omitted.
	type procLite interface{}
	_ = procLite(nil)
	if cpu, ram, ok := lookupProcessLoadByExe(base); ok {
		it.Running = true
		it.CPUPercent = cpu
		it.RAMMB = ram
	}
}

func isCriticalStartup(name, base, cmd string) bool {
	for hint := range criticalStartupNames {
		if criticalStartupNames[hint] && (strings.Contains(name, hint) || strings.Contains(base, hint)) {
			return true
		}
	}
	for _, h := range criticalPathHints {
		if strings.Contains(cmd, h) || strings.Contains(base, h) {
			return true
		}
	}
	return false
}

func guessStartupImpact(blob string) string {
	_, _, impact := classifyStartupCategory(blob)
	if impact != "" {
		return impact
	}
	blob = strings.ToLower(blob)
	for _, h := range highImpactHints {
		if strings.Contains(blob, h) {
			return "high"
		}
	}
	for _, h := range mediumImpactHints {
		if strings.Contains(blob, h) {
			return "medium"
		}
	}
	return ""
}

func lookupProcessLoadByExe(exeBase string) (cpuPct, ramMB float64, ok bool) {
	exeBase = strings.ToLower(strings.TrimSpace(exeBase))
	if exeBase == "" {
		return 0, 0, false
	}
	procs, err := listProcessSnapsLite()
	if err != nil || len(procs) == 0 {
		return 0, 0, false
	}
	for _, p := range procs {
		if strings.ToLower(p.Name) == exeBase || strings.HasSuffix(strings.ToLower(p.Name), exeBase) {
			return p.CPU, p.RAMMB, true
		}
	}
	return 0, 0, false
}

type procSnapLite struct {
	Name  string
	CPU   float64
	RAMMB float64
}

func listProcessSnapsLite() ([]procSnapLite, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}
	out := make([]procSnapLite, 0, 32)
	for _, p := range procs {
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}
		cpu, _ := p.CPUPercent()
		var ramMB float64
		if mi, err := p.MemoryInfo(); err == nil && mi != nil {
			ramMB = float64(mi.RSS) / (1024 * 1024)
		}
		out = append(out, procSnapLite{Name: name, CPU: cpu, RAMMB: ramMB})
	}
	return out, nil
}

func stashDisabledRun(valueName, command string) error {
	if valueName == "" || command == "" {
		return nil
	}
	disabledKey, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\PulseCore\DisabledRun`, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer disabledKey.Close()
	return disabledKey.SetStringValue(valueName, command)
}

func guessPublisher(path, base string) string {
	switch {
	case strings.Contains(base, "discord"):
		return "Discord Inc."
	case strings.Contains(base, "steam"):
		return "Valve"
	case strings.Contains(base, "spotify"):
		return "Spotify"
	case strings.Contains(base, "chrome"):
		return "Google"
	case strings.Contains(base, "msedge"):
		return "Microsoft"
	case strings.Contains(base, "teams"):
		return "Microsoft"
	case strings.Contains(base, "adobe"), strings.Contains(base, "ccx"):
		return "Adobe"
	case strings.Contains(base, "nvidia"):
		return "NVIDIA"
	case path != "":
		if _, err := os.Stat(path); err == nil {
			return ""
		}
	}
	return ""
}

// SetStartupItemEnabled enables or disables a non-critical HKCU / user-folder entry after UI confirm.
func (a *App) SetStartupItemEnabled(id string, enabled bool) StartupDisableResult {
	id = strings.TrimSpace(id)
	if id == "" {
		return StartupDisableResult{OK: false, MessageRu: "Не указана запись.", MessageEn: "No entry specified."}
	}
	c := a.ensureStartupCache()
	c.mu.Lock()
	items := append([]StartupItem(nil), c.items...)
	c.mu.Unlock()
	if len(items) == 0 {
		items = a.GetStartupItems()
	}
	var target *StartupItem
	for i := range items {
		if items[i].ID == id {
			target = &items[i]
			break
		}
	}
	if target == nil {
		return StartupDisableResult{OK: false, MessageRu: "Запись не найдена - обновите список.", MessageEn: "Entry not found - refresh the list."}
	}
	if target.Critical || !target.CanDisable {
		return StartupDisableResult{
			OK: false,
			MessageRu: "Эту запись нельзя менять из PulseCore.",
			MessageEn: "This entry cannot be changed from PulseCore.",
		}
	}
	if target.Location == "hkcu_run" {
		// Prefer Task Manager-style StartupApproved (reversible); keep value in Run.
		if err := setStartupApproved(registry.CURRENT_USER, target.RegValue, enabled); err != nil {
			// Fallback: move value to DisabledRun stash.
			if err2 := setHKCURunEnabled(target.RegValue, target.Command, enabled); err2 != nil {
				return StartupDisableResult{OK: false, MessageRu: err2.Error(), MessageEn: err2.Error()}
			}
		} else if !enabled {
			// Approved-disable leaves Run value; also mirror to stash for recovery tools.
			_ = stashDisabledRun(target.RegValue, target.Command)
		}
	} else if target.Location == "startup_user" {
		if err := setStartupFolderEnabled(target.FolderFile, enabled); err != nil {
			return StartupDisableResult{OK: false, MessageRu: err.Error(), MessageEn: err.Error()}
		}
	} else {
		return StartupDisableResult{
			OK: false,
			MessageRu: "Поддерживается только автозапуск текущего пользователя.",
			MessageEn: "Only current-user startup entries are supported.",
		}
	}
	a.GetStartupItems() // refresh cache
	if enabled {
		return StartupDisableResult{OK: true, MessageRu: "Автозапуск включён.", MessageEn: "Startup enabled."}
	}
	return StartupDisableResult{OK: true, MessageRu: "Автозапуск отключён.", MessageEn: "Startup disabled."}
}

func setHKCURunEnabled(valueName, command string, enabled bool) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	disabledKey, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\PulseCore\DisabledRun`, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer disabledKey.Close()

	if !enabled {
		// stash command then delete from Run
		if command == "" {
			if v, _, e := key.GetStringValue(valueName); e == nil {
				command = v
			}
		}
		if command != "" {
			_ = disabledKey.SetStringValue(valueName, command)
		}
		if err := key.DeleteValue(valueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	// re-enable from stash or provided command
	cmd := command
	if v, _, e := disabledKey.GetStringValue(valueName); e == nil && v != "" {
		cmd = v
	}
	if cmd == "" {
		return fmt.Errorf("no saved command to restore")
	}
	if err := key.SetStringValue(valueName, cmd); err != nil {
		return err
	}
	_ = disabledKey.DeleteValue(valueName)
	return nil
}

func setStartupFolderEnabled(file string, enabled bool) error {
	if file == "" {
		return fmt.Errorf("empty path")
	}
	if enabled {
		if strings.HasSuffix(strings.ToLower(file), ".disabled") {
			newPath := strings.TrimSuffix(file, ".disabled")
			newPath = strings.TrimSuffix(newPath, ".DISABLED")
			return os.Rename(file, newPath)
		}
		return nil
	}
	if strings.HasSuffix(strings.ToLower(file), ".disabled") {
		return nil
	}
	return os.Rename(file, file+".disabled")
}
