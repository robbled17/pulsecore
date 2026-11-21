package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// InstalledApp is one entry from Windows Uninstall registry.
type InstalledApp struct {
	ID              string `json:"id"`
	DisplayName     string `json:"displayName"`
	Publisher       string `json:"publisher"`
	InstallLocation string `json:"installLocation"`
	UninstallString string `json:"uninstallString"`
	DisplayVersion  string `json:"displayVersion"`
	InstallDate     string `json:"installDate"`
	EstimatedMB     int64  `json:"estimatedMB"`
	IsMSI           bool   `json:"isMsi"`
}

// RunningAppProcess is a process whose image lives under InstallLocation.
type RunningAppProcess struct {
	PID       uint32 `json:"pid"`
	Name      string `json:"name"`
	ImagePath string `json:"imagePath"`
}

// UninstallPreCheck is returned before uninstall.
type UninstallPreCheck struct {
	App       InstalledApp         `json:"app"`
	Running   []RunningAppProcess  `json:"running"`
	CanKill   bool                 `json:"canKill"`
	Warning   string               `json:"warning"`
	OK        bool                 `json:"ok"`
}

// ResidualItem is one leftover after uninstall.
type ResidualItem struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"` // folder | file | registry
	Path     string `json:"path"`
	Label    string `json:"label"`
	Safe     bool   `json:"safe"` // false = blocked by whitelist
	Selected bool   `json:"selected"`
}

// ResidualScanResult is the post-uninstall checklist.
type ResidualScanResult struct {
	AppName string         `json:"appName"`
	Items   []ResidualItem `json:"items"`
	OK      bool           `json:"ok"`
	Message string         `json:"message"`
}

// InstallTrace is a before/after snapshot for precise cleanup.
type InstallTrace struct {
	AppID       string            `json:"appId"`
	AppName     string            `json:"appName"`
	Installer   string            `json:"installer"`
	CreatedAt   string            `json:"createdAt"`
	FinishedAt  string            `json:"finishedAt,omitempty"`
	Before      TraceSnapshot     `json:"before"`
	After       TraceSnapshot     `json:"after,omitempty"`
	NewKeys     []string          `json:"newKeys,omitempty"`
	NewDirs     []string          `json:"newDirs,omitempty"`
	NewServices []string          `json:"newServices,omitempty"`
	Extra       map[string]string `json:"extra,omitempty"`
}

// TraceSnapshot captures registry + install dirs.
type TraceSnapshot struct {
	UninstallKeys []string `json:"uninstallKeys"`
	RunKeys       []string `json:"runKeys"`
	Services      []string `json:"services"`
	Dirs          []string `json:"dirs"`
}

// TraceCleanupResult is cleanup driven by a saved trace.
type TraceCleanupResult struct {
	AppID   string         `json:"appId"`
	Removed []ResidualItem `json:"removed"`
	Failed  []ResidualItem `json:"failed"`
	OK      bool           `json:"ok"`
	Message string         `json:"message"`
}

var (
	installTraceMu   sync.Mutex
	activeInstallTrace *InstallTrace
)

// Hard whitelist: never propose deleting these residual roots/names.
var residualWhitelist = []string{
	`microsoft`, `windows`, `system32`, `syswow64`, `winsxs`,
	`pulsecore`, `zeropulse`,
	`nvidia`, `amd`, `intel`, `realtek`,
	`defender`, `security health`,
	`.net`, `visual c++`, `vcredist`,
	`edgeupdate`, `google\\update`,
}

func (a *App) tracesDir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "PulseCore", "traces")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// ListInstalledApps enumerates HKLM/HKCU Uninstall entries.
func (a *App) ListInstalledApps() []InstalledApp {
	seen := map[string]bool{}
	out := make([]InstalledApp, 0, 128)
	roots := []struct {
		key  registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	}
	for _, r := range roots {
		k, err := registry.OpenKey(r.key, r.path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		names, _ := k.ReadSubKeyNames(-1)
		for _, name := range names {
			sub, err := registry.OpenKey(k, name, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			app := readUninstallEntry(name, sub)
			sub.Close()
			if app.DisplayName == "" || app.UninstallString == "" {
				continue
			}
			if seen[app.ID] {
				continue
			}
			seen[app.ID] = true
			out = append(out, app)
		}
		k.Close()
	}
	return out
}

func readUninstallEntry(keyName string, k registry.Key) InstalledApp {
	get := func(name string) string {
		v, _, err := k.GetStringValue(name)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(v)
	}
	display := get("DisplayName")
	uninst := get("UninstallString")
	loc := get("InstallLocation")
	pub := get("Publisher")
	ver := get("DisplayVersion")
	date := get("InstallDate")
	var sizeMB int64
	if sz, _, err := k.GetIntegerValue("EstimatedSize"); err == nil {
		sizeMB = int64(sz) / 1024
	}
	id := keyName
	if display != "" {
		sum := sha1.Sum([]byte(strings.ToLower(display + "|" + uninst)))
		id = hex.EncodeToString(sum[:8])
	}
	isMSI := strings.Contains(strings.ToLower(uninst), "msiexec")
	return InstalledApp{
		ID:              id,
		DisplayName:     display,
		Publisher:       pub,
		InstallLocation: loc,
		UninstallString: uninst,
		DisplayVersion:  ver,
		InstallDate:     date,
		EstimatedMB:     sizeMB,
		IsMSI:           isMSI,
	}
}

// PrecheckUninstall finds processes under InstallLocation.
func (a *App) PrecheckUninstall(appID string) UninstallPreCheck {
	app, ok := findInstalledApp(a, appID)
	if !ok {
		return UninstallPreCheck{OK: false, Warning: "Приложение не найдено в списке удаления"}
	}
	running := findProcessesUnder(app.InstallLocation)
	warn := ""
	if len(running) > 0 {
		warn = fmt.Sprintf("Найдено запущенных процессов: %d. Завершите их перед удалением или нажмите «Завершить и удалить».", len(running))
	}
	return UninstallPreCheck{
		App:     app,
		Running: running,
		CanKill: len(running) > 0,
		Warning: warn,
		OK:      true,
	}
}

func findInstalledApp(a *App, appID string) (InstalledApp, bool) {
	for _, app := range a.ListInstalledApps() {
		if app.ID == appID {
			return app, true
		}
	}
	return InstalledApp{}, false
}

func findProcessesUnder(installLocation string) []RunningAppProcess {
	root := strings.TrimSpace(installLocation)
	if root == "" {
		return nil
	}
	root = filepath.Clean(root)
	rootLower := strings.ToLower(root)

	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	err = windows.Process32First(snap, &pe)
	out := make([]RunningAppProcess, 0, 4)
	for err == nil {
		pid := pe.ProcessID
		name := windows.UTF16ToString(pe.ExeFile[:])
		if pid > 4 {
			if path := queryProcessImagePath(pid); path != "" {
				pl := strings.ToLower(filepath.Clean(path))
				if strings.HasPrefix(pl, rootLower+string(os.PathSeparator)) || pl == rootLower {
					out = append(out, RunningAppProcess{PID: pid, Name: name, ImagePath: path})
				}
			}
		}
		err = windows.Process32Next(snap, &pe)
	}
	return out
}

var (
	procQueryFullProcessImageNameW = modKernel32.NewProc("QueryFullProcessImageNameW")
)

func queryProcessImagePath(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	r, _, _ := procQueryFullProcessImageNameW.Call(
		uintptr(h),
		uintptr(0),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:size])
}

// KillAppProcesses terminates processes under InstallLocation.
func (a *App) KillAppProcesses(appID string) map[string]interface{} {
	app, ok := findInstalledApp(a, appID)
	if !ok {
		return map[string]interface{}{"ok": false, "message": "Приложение не найдено"}
	}
	running := findProcessesUnder(app.InstallLocation)
	killed := 0
	failed := 0
	for _, p := range running {
		if err := terminatePID(p.PID); err != nil {
			failed++
		} else {
			killed++
		}
	}
	return map[string]interface{}{
		"ok":      failed == 0,
		"killed":  killed,
		"failed":  failed,
		"message": fmt.Sprintf("Завершено: %d, ошибок: %d", killed, failed),
	}
}

func terminatePID(pid uint32) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.TerminateProcess(h, 1)
}

// UninstallApp launches the official uninstaller. If killFirst, terminates matching processes.
func (a *App) UninstallApp(appID string, killFirst bool) map[string]interface{} {
	app, ok := findInstalledApp(a, appID)
	if !ok {
		return map[string]interface{}{"ok": false, "message": "Приложение не найдено"}
	}
	if killFirst {
		_ = a.KillAppProcesses(appID)
		time.Sleep(400 * time.Millisecond)
	}
	if err := launchUninstaller(app.UninstallString); err != nil {
		return map[string]interface{}{"ok": false, "message": err.Error()}
	}
	return map[string]interface{}{
		"ok":      true,
		"message": "Запущен официальный деинсталлятор",
		"appName": app.DisplayName,
	}
}

func launchUninstaller(cmdLine string) error {
	cmdLine = strings.TrimSpace(cmdLine)
	if cmdLine == "" {
		return fmt.Errorf("пустая команда удаления")
	}
	// Prefer msiexec quiet-ish interactive: leave UI to the vendor uninstaller.
	exe, args := splitCommandLine(cmdLine)
	if exe == "" {
		return fmt.Errorf("не удалось разобрать команду удаления")
	}
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: false}
	return cmd.Start()
}

func splitCommandLine(s string) (string, []string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	// Use Windows CommandLineToArgv-like simple parse.
	var parts []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuote = !inQuote
		case c == ' ' && !inQuote:
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

// ScanResiduals looks for leftovers by publisher/app name with a hard whitelist.
func (a *App) ScanResiduals(appName, publisher string) ResidualScanResult {
	appName = strings.TrimSpace(appName)
	publisher = strings.TrimSpace(publisher)
	if appName == "" {
		return ResidualScanResult{OK: false, Message: "Не указано имя приложения"}
	}
	needles := uniqueNonEmpty([]string{
		sanitizeNameToken(appName),
		sanitizeNameToken(publisher),
		firstToken(appName),
	})
	items := make([]ResidualItem, 0, 24)

	// Folders
	folderRoots := residualFolderRoots()
	for _, root := range folderRoots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if !nameMatchesAny(name, needles) {
				continue
			}
			full := filepath.Join(root, name)
			safe := !isWhitelistedResidual(full, name)
			items = append(items, ResidualItem{
				ID:       "fs:" + full,
				Kind:     dirOrFile(e),
				Path:     full,
				Label:    filepath.Base(root) + " · " + name,
				Safe:     safe,
				Selected: safe,
			})
		}
	}

	// Registry Software + Run
	regItems := scanResidualRegistry(needles)
	items = append(items, regItems...)

	msg := fmt.Sprintf("Найдено остатков: %d", len(items))
	if len(items) == 0 {
		msg = "Остатков не найдено"
	}
	return ResidualScanResult{
		AppName: appName,
		Items:   items,
		OK:      true,
		Message: msg,
	}
}

func residualFolderRoots() []string {
	local := os.Getenv("LOCALAPPDATA")
	roaming := os.Getenv("APPDATA")
	programData := os.Getenv("ProgramData")
	roots := []string{}
	for _, r := range []string{local, roaming, programData} {
		if r != "" {
			roots = append(roots, r)
		}
	}
	return roots
}

func dirOrFile(e fs.DirEntry) string {
	if e.IsDir() {
		return "folder"
	}
	return "file"
}

func sanitizeNameToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	repl := strings.NewReplacer("®", "", "™", "", "(tm)", "", "(r)", "")
	s = repl.Replace(s)
	return strings.TrimSpace(s)
}

func firstToken(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return sanitizeNameToken(fields[0])
}

func uniqueNonEmpty(xs []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x == "" || len(x) < 3 {
			continue
		}
		if seen[x] {
			continue
		}
		seen[x] = true
		out = append(out, x)
	}
	return out
}

func nameMatchesAny(name string, needles []string) bool {
	n := strings.ToLower(name)
	for _, needle := range needles {
		if needle != "" && strings.Contains(n, needle) {
			return true
		}
	}
	return false
}

func isWhitelistedResidual(fullPath, name string) bool {
	blob := strings.ToLower(fullPath + " " + name)
	for _, w := range residualWhitelist {
		if strings.Contains(blob, w) {
			return true
		}
	}
	// Protect Windows roots themselves
	win := strings.ToLower(os.Getenv("WINDIR"))
	if win != "" && strings.HasPrefix(strings.ToLower(fullPath), win) {
		return true
	}
	return false
}

func scanResidualRegistry(needles []string) []ResidualItem {
	items := make([]ResidualItem, 0, 16)
	type rootSpec struct {
		key  registry.Key
		path string
		label string
	}
	specs := []rootSpec{
		{registry.CURRENT_USER, `SOFTWARE`, "HKCU\\Software"},
		{registry.LOCAL_MACHINE, `SOFTWARE`, "HKLM\\Software"},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node`, "HKLM\\Software\\WOW6432Node"},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, "HKCU\\Run"},
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, "HKLM\\Run"},
	}
	for _, sp := range specs {
		if strings.HasSuffix(strings.ToLower(sp.path), `\run`) {
			items = append(items, scanRunValues(sp.key, sp.path, sp.label, needles)...)
			continue
		}
		k, err := registry.OpenKey(sp.key, sp.path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		names, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, name := range names {
			if !nameMatchesAny(name, needles) {
				continue
			}
			full := sp.label + "\\" + name
			safe := !isWhitelistedResidual(full, name)
			items = append(items, ResidualItem{
				ID:       "reg:" + full,
				Kind:     "registry",
				Path:     full,
				Label:    full,
				Safe:     safe,
				Selected: safe,
			})
		}
	}
	return items
}

func scanRunValues(root registry.Key, path, label string, needles []string) []ResidualItem {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	names, _ := k.ReadValueNames(-1)
	out := make([]ResidualItem, 0)
	for _, name := range names {
		val, _, _ := k.GetStringValue(name)
		blob := name + " " + val
		if !nameMatchesAny(blob, needles) {
			continue
		}
		full := label + " · " + name
		safe := !isWhitelistedResidual(full+" "+val, name)
		out = append(out, ResidualItem{
			ID:       "run:" + label + ":" + name,
			Kind:     "registry",
			Path:     full,
			Label:    full,
			Safe:     safe,
			Selected: safe,
		})
	}
	return out
}

// DeleteResiduals removes selected safe leftover items.
func (a *App) DeleteResiduals(items []ResidualItem) map[string]interface{} {
	removed := 0
	failed := 0
	skipped := 0
	for _, it := range items {
		if !it.Selected {
			continue
		}
		if !it.Safe || isWhitelistedResidual(it.Path, it.Label) {
			skipped++
			continue
		}
		switch it.Kind {
		case "folder", "file":
			if err := os.RemoveAll(it.Path); err != nil {
				failed++
			} else {
				removed++
			}
		case "registry":
			if err := deleteResidualRegistry(it); err != nil {
				failed++
			} else {
				removed++
			}
		default:
			skipped++
		}
	}
	return map[string]interface{}{
		"ok":      failed == 0,
		"removed": removed,
		"failed":  failed,
		"skipped": skipped,
		"message": fmt.Sprintf("Удалено: %d, ошибок: %d, пропущено: %d", removed, failed, skipped),
	}
}

func deleteResidualRegistry(it ResidualItem) error {
	// Only delete leaf Software subkeys we discovered, never Run wholesale.
	path := it.Path
	if strings.Contains(it.ID, "run:") {
		// run:HKCU\Run:ValueName
		parts := strings.SplitN(strings.TrimPrefix(it.ID, "run:"), ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("bad run id")
		}
		label, valueName := parts[0], parts[1]
		var root registry.Key
		var keyPath string
		switch {
		case strings.HasPrefix(label, "HKCU"):
			root = registry.CURRENT_USER
			keyPath = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
		default:
			root = registry.LOCAL_MACHINE
			keyPath = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
		}
		k, err := registry.OpenKey(root, keyPath, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer k.Close()
		return k.DeleteValue(valueName)
	}
	// reg:HKCU\Software\Name
	clean := strings.TrimPrefix(path, `HKCU\Software\`)
	root := registry.CURRENT_USER
	base := `SOFTWARE`
	if strings.HasPrefix(path, `HKLM\Software\WOW6432Node\`) {
		clean = strings.TrimPrefix(path, `HKLM\Software\WOW6432Node\`)
		root = registry.LOCAL_MACHINE
		base = `SOFTWARE\WOW6432Node`
	} else if strings.HasPrefix(path, `HKLM\Software\`) {
		clean = strings.TrimPrefix(path, `HKLM\Software\`)
		root = registry.LOCAL_MACHINE
		base = `SOFTWARE`
	} else if strings.HasPrefix(path, `HKCU\Software\`) {
		clean = strings.TrimPrefix(path, `HKCU\Software\`)
		root = registry.CURRENT_USER
		base = `SOFTWARE`
	} else {
		return fmt.Errorf("unsupported registry path")
	}
	// Only delete a single subkey under Software, never nested deep paths with backslashes beyond one level.
	if strings.Contains(clean, `\`) || clean == "" {
		return fmt.Errorf("refusing nested registry delete")
	}
	parent, err := registry.OpenKey(root, base, registry.WRITE)
	if err != nil {
		return err
	}
	defer parent.Close()
	return registry.DeleteKey(parent, clean)
}

// StartTracedInstall opens a file picker, snapshots state, launches installer.
func (a *App) StartTracedInstall() map[string]interface{} {
	if a.ctx == nil {
		return map[string]interface{}{"ok": false, "message": "Приложение ещё не готово"}
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Выберите установщик (.exe или .msi)",
		Filters: []runtime.FileFilter{
			{DisplayName: "Installers", Pattern: "*.exe;*.msi"},
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
	if err != nil || strings.TrimSpace(path) == "" {
		return map[string]interface{}{"ok": false, "message": "Установка отменена"}
	}
	before := captureTraceSnapshot()
	sum := sha1.Sum([]byte(strings.ToLower(path) + time.Now().Format(time.RFC3339Nano)))
	appID := hex.EncodeToString(sum[:8])
	trace := &InstallTrace{
		AppID:     appID,
		AppName:   filepath.Base(path),
		Installer: path,
		CreatedAt: time.Now().Format(time.RFC3339),
		Before:    before,
	}
	installTraceMu.Lock()
	activeInstallTrace = trace
	installTraceMu.Unlock()

	cmd := exec.Command(path)
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".msi" {
		cmd = exec.Command("msiexec.exe", "/i", path)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: false}
	if err := cmd.Start(); err != nil {
		return map[string]interface{}{"ok": false, "message": err.Error()}
	}
	return map[string]interface{}{
		"ok":      true,
		"appId":   appID,
		"appName": trace.AppName,
		"message": "Установщик запущен. После установки нажмите «Завершить трассировку».",
	}
}

// FinishTracedInstall diffs snapshots and saves traces/<app_id>.json.
func (a *App) FinishTracedInstall() map[string]interface{} {
	installTraceMu.Lock()
	trace := activeInstallTrace
	installTraceMu.Unlock()
	if trace == nil {
		return map[string]interface{}{"ok": false, "message": "Нет активной трассировки"}
	}
	after := captureTraceSnapshot()
	trace.After = after
	trace.FinishedAt = time.Now().Format(time.RFC3339)
	trace.NewKeys = diffStrings(trace.Before.UninstallKeys, after.UninstallKeys)
	trace.NewKeys = append(trace.NewKeys, diffStrings(trace.Before.RunKeys, after.RunKeys)...)
	trace.NewServices = diffStrings(trace.Before.Services, after.Services)
	trace.NewDirs = diffStrings(trace.Before.Dirs, after.Dirs)

	path := filepath.Join(a.tracesDir(), trace.AppID+".json")
	raw, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return map[string]interface{}{"ok": false, "message": err.Error()}
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return map[string]interface{}{"ok": false, "message": err.Error()}
	}
	installTraceMu.Lock()
	activeInstallTrace = nil
	installTraceMu.Unlock()
	return map[string]interface{}{
		"ok":       true,
		"appId":    trace.AppID,
		"path":     path,
		"newKeys":  len(trace.NewKeys),
		"newDirs":  len(trace.NewDirs),
		"message":  fmt.Sprintf("Трассировка сохранена: %s", filepath.Base(path)),
	}
}

// ListInstallTraces returns saved trace metadata.
func (a *App) ListInstallTraces() []map[string]interface{} {
	dir := a.tracesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]map[string]interface{}, 0)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var tr InstallTrace
		if json.Unmarshal(raw, &tr) != nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"appId":      tr.AppID,
			"appName":    tr.AppName,
			"createdAt":  tr.CreatedAt,
			"finishedAt": tr.FinishedAt,
			"newKeys":    len(tr.NewKeys),
			"newDirs":    len(tr.NewDirs),
			"file":       e.Name(),
		})
	}
	return out
}

// CleanupFromTrace removes new dirs/keys recorded in a trace (whitelist enforced).
func (a *App) CleanupFromTrace(appID string) TraceCleanupResult {
	path := filepath.Join(a.tracesDir(), appID+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return TraceCleanupResult{AppID: appID, OK: false, Message: "Трассировка не найдена"}
	}
	var tr InstallTrace
	if err := json.Unmarshal(raw, &tr); err != nil {
		return TraceCleanupResult{AppID: appID, OK: false, Message: "Повреждённый файл трассировки"}
	}
	removed := make([]ResidualItem, 0)
	failed := make([]ResidualItem, 0)
	for _, dir := range tr.NewDirs {
		it := ResidualItem{ID: "fs:" + dir, Kind: "folder", Path: dir, Label: dir, Safe: !isWhitelistedResidual(dir, filepath.Base(dir)), Selected: true}
		if !it.Safe {
			failed = append(failed, it)
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			failed = append(failed, it)
		} else {
			removed = append(removed, it)
		}
	}
	for _, key := range tr.NewKeys {
		it := ResidualItem{ID: "reg:" + key, Kind: "registry", Path: key, Label: key, Safe: !isWhitelistedResidual(key, key), Selected: true}
		if !it.Safe {
			failed = append(failed, it)
			continue
		}
		if err := deleteTraceRegistryKey(key); err != nil {
			failed = append(failed, it)
		} else {
			removed = append(removed, it)
		}
	}
	return TraceCleanupResult{
		AppID:   appID,
		Removed: removed,
		Failed:  failed,
		OK:      len(failed) == 0,
		Message: fmt.Sprintf("Удалено: %d, ошибок: %d", len(removed), len(failed)),
	}
}

func deleteTraceRegistryKey(key string) error {
	// Stored as "HKCU\\Software\\Name" or "HKLM\\Run\\Value=..."
	if strings.Contains(key, `\Run\`) || strings.HasPrefix(key, "RUN:") {
		return fmt.Errorf("run key cleanup via residual path")
	}
	it := ResidualItem{Path: key, Kind: "registry", ID: "reg:" + key, Safe: true, Selected: true}
	return deleteResidualRegistry(it)
}

func captureTraceSnapshot() TraceSnapshot {
	return TraceSnapshot{
		UninstallKeys: listUninstallKeyNames(),
		RunKeys:       listRunValueNames(),
		Services:      listServiceNames(),
		Dirs:          listCommonInstallDirs(),
	}
}

func listUninstallKeyNames() []string {
	out := make([]string, 0, 128)
	roots := []struct {
		key  registry.Key
		path string
		pref string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, "HKLM"},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`, "HKLM32"},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, "HKCU"},
	}
	for _, r := range roots {
		k, err := registry.OpenKey(r.key, r.path, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		names, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, n := range names {
			out = append(out, r.pref+`\`+n)
		}
	}
	return out
}

func listRunValueNames() []string {
	out := make([]string, 0, 32)
	specs := []struct {
		key  registry.Key
		path string
		pref string
	}{
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, "HKCU"},
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, "HKLM"},
	}
	for _, sp := range specs {
		k, err := registry.OpenKey(sp.key, sp.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		names, _ := k.ReadValueNames(-1)
		k.Close()
		for _, n := range names {
			out = append(out, sp.pref+`\Run\`+n)
		}
	}
	return out
}

func listServiceNames() []string {
	out := make([]string, 0, 64)
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services`, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return out
	}
	names, _ := k.ReadSubKeyNames(-1)
	k.Close()
	return names
}

func listCommonInstallDirs() []string {
	out := make([]string, 0, 64)
	roots := []string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("LOCALAPPDATA"),
	}
	for _, root := range roots {
		if root == "" {
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				out = append(out, filepath.Join(root, e.Name()))
			}
		}
	}
	return out
}

func diffStrings(before, after []string) []string {
	set := map[string]bool{}
	for _, b := range before {
		set[b] = true
	}
	out := make([]string, 0)
	for _, a := range after {
		if !set[a] {
			out = append(out, a)
		}
	}
	return out
}
