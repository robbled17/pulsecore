//go:build windows

package main

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	modKernel32Boot      = windows.NewLazySystemDLL("kernel32.dll")
	procGetTickCount64BA = modKernel32Boot.NewProc("GetTickCount64")
)

func getSystemUptimeMs() uint64 {
	r, _, _ := procGetTickCount64BA.Call()
	return uint64(r)
}

// BootDiagnostics contains startup time analysis, degraders, and startup items.
type BootDiagnostics struct {
	EventLogAvailable  bool                  `json:"eventLogAvailable"`
	BootTimeSec        float64               `json:"bootTimeSec"`
	KernelSec          float64               `json:"kernelSec"`
	DesktopSec         float64               `json:"desktopSec"`
	BootRating         string                `json:"bootRating"` // "fast", "moderate", "slow"
	FastStartupWarning bool                  `json:"fastStartupWarning"`
	FastStartupDays    int                   `json:"fastStartupDays"`
	HiberbootEnabled   bool                  `json:"hiberbootEnabled"`
	UptimeHours        float64               `json:"uptimeHours"`
	Degraders          []BootDegradationItem `json:"degraders"`
	StartupItems       []BootStartupItem     `json:"startupItems"`
}

// BootDegradationItem describes a specific executable that slowed down boot (Event ID 101).
type BootDegradationItem struct {
	Name     string  `json:"name"`
	TotalSec float64 `json:"totalSec"`
}

// BootStartupItem represents an autorun program or scheduled task.
type BootStartupItem struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Command     string  `json:"command"`
	Location    string  `json:"location"` // "registry_user", "registry_machine", "startup_folder", "task_scheduler", "delayed_task"
	Status      string  `json:"status"`   // "enabled", "delayed", "disabled"
	IsProtected bool    `json:"isProtected"`
	DelaySec    int     `json:"delaySec"`
	DegradeSec  float64 `json:"degradeSec"`
}

// Event 100 XML structure
type eventXML struct {
	EventData struct {
		Data []struct {
			Name  string `xml:"Name,attr"`
			Value string `xml:",chardata"`
		} `xml:"Data"`
	} `xml:"EventData"`
}

// Events wrapper
type eventsXML struct {
	Events []eventXML `xml:"Event"`
}

// isProtectedStartupItem returns true for essential drivers and PulseCore itself.
func isProtectedStartupItem(name, command string) bool {
	blob := strings.ToLower(name + " " + command)
	protectedTerms := []string{
		"realtek", "rtkauduservice", "rtkngui", "ravcpl64",
		"nvidia", "nvcontainer", "nvcpl", "nvdisplay",
		"amd", "radeon", "amddvr", "amdow",
		"intel graphics", "igfx", "igfxtray",
		"securityhealthsystray", "securityhealthhost",
		"ctfmon", "pulsecore",
	}
	for _, term := range protectedTerms {
		if strings.Contains(blob, term) {
			return true
		}
	}
	return false
}

// parseEvent100XML extracts boot timing metrics from Event 100 XML.
func parseEvent100XML(xmlData []byte) (bootSec, kernelSec, desktopSec float64, ok bool) {
	var ev eventXML
	if err := xml.Unmarshal(xmlData, &ev); err != nil {
		var evs eventsXML
		if err2 := xml.Unmarshal(xmlData, &evs); err2 != nil || len(evs.Events) == 0 {
			return 0, 0, 0, false
		}
		ev = evs.Events[0]
	}

	var bootMs, mainPathMs float64
	foundBoot, foundMain := false, false

	for _, d := range ev.EventData.Data {
		switch d.Name {
		case "BootTime":
			if v, err := strconv.ParseFloat(strings.TrimSpace(d.Value), 64); err == nil && v > 0 {
				bootMs = v
				foundBoot = true
			}
		case "MainPathBootTime":
			if v, err := strconv.ParseFloat(strings.TrimSpace(d.Value), 64); err == nil && v > 0 {
				mainPathMs = v
				foundMain = true
			}
		}
	}

	if !foundBoot {
		return 0, 0, 0, false
	}

	bootSec = bootMs / 1000.0
	if foundMain && mainPathMs <= bootMs {
		kernelSec = mainPathMs / 1000.0
		desktopSec = bootSec - kernelSec
	} else {
		kernelSec = bootSec * 0.4
		desktopSec = bootSec * 0.6
	}

	bootSec = float64(int(bootSec*10)) / 10
	kernelSec = float64(int(kernelSec*10)) / 10
	desktopSec = float64(int(desktopSec*10)) / 10

	return bootSec, kernelSec, desktopSec, true
}

// parseEvent101XML extracts boot degraders from Event 101 XML.
func parseEvent101XML(xmlData []byte) []BootDegradationItem {
	var evs eventsXML
	wrapped := []byte("<Events>" + string(xmlData) + "</Events>")
	if err := xml.Unmarshal(wrapped, &evs); err != nil {
		var single eventXML
		if err2 := xml.Unmarshal(xmlData, &single); err2 == nil {
			evs.Events = []eventXML{single}
		} else {
			return nil
		}
	}

	var items []BootDegradationItem
	seen := map[string]bool{}

	for _, ev := range evs.Events {
		var name string
		var totalMs float64

		for _, d := range ev.EventData.Data {
			switch d.Name {
			case "Name":
				name = strings.TrimSpace(d.Value)
			case "TotalTime":
				if v, err := strconv.ParseFloat(strings.TrimSpace(d.Value), 64); err == nil {
					totalMs = v
				}
			}
		}

		if name != "" && totalMs > 500 {
			baseName := filepath.Base(name)
			low := strings.ToLower(baseName)
			if !seen[low] {
				seen[low] = true
				sec := float64(int(totalMs/100)) / 10.0
				items = append(items, BootDegradationItem{
					Name:     baseName,
					TotalSec: sec,
				})
			}
		}
	}

	return items
}

// readBootEventsFromLog executes wevtutil to read Events 100 and 101.
func readBootEventsFromLog() (bootSec, kernelSec, desktopSec float64, degraders []BootDegradationItem, hasLog bool) {
	cmd100 := exec.Command("wevtutil", "qe", "Microsoft-Windows-Diagnostics-Performance/Operational",
		`/q:*[System[(EventID=100)]]`, "/c:1", "/rd:true", "/f:xml")
	cmd100.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out100, err100 := cmd100.Output()

	if err100 != nil || len(out100) == 0 {
		return 0, 0, 0, nil, false
	}

	bootSec, kernelSec, desktopSec, ok := parseEvent100XML(out100)
	if !ok {
		return 0, 0, 0, nil, false
	}

	cmd101 := exec.Command("wevtutil", "qe", "Microsoft-Windows-Diagnostics-Performance/Operational",
		`/q:*[System[(EventID=101)]]`, "/c:8", "/rd:true", "/f:xml")
	cmd101.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out101, _ := cmd101.Output()

	if len(out101) > 0 {
		degraders = parseEvent101XML(out101)
	}

	return bootSec, kernelSec, desktopSec, degraders, true
}

// readStartupApprovedBytes reads the binary value from StartupApproved key.
func readStartupApprovedBytes(root registry.Key, subKey, valName string) []byte {
	k, err := registry.OpenKey(root, subKey, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		if b, _, bErr := k.GetBinaryValue(valName); bErr == nil {
			return b
		}
	}

	// If root was HKLM, also check HKCU user override and 32-bit Run32
	if root == registry.LOCAL_MACHINE {
		if k2, err2 := registry.OpenKey(registry.CURRENT_USER, subKey, registry.QUERY_VALUE); err2 == nil {
			defer k2.Close()
			if b2, _, bErr2 := k2.GetBinaryValue(valName); bErr2 == nil {
				return b2
			}
		}
		if strings.HasSuffix(subKey, `\Run`) {
			subKey32 := subKey + "32"
			if k3, err3 := registry.OpenKey(registry.LOCAL_MACHINE, subKey32, registry.QUERY_VALUE); err3 == nil {
				defer k3.Close()
				if b3, _, bErr3 := k3.GetBinaryValue(valName); bErr3 == nil {
					return b3
				}
			}
			if k4, err4 := registry.OpenKey(registry.CURRENT_USER, subKey32, registry.QUERY_VALUE); err4 == nil {
				defer k4.Close()
				if b4, _, bErr4 := k4.GetBinaryValue(valName); bErr4 == nil {
					return b4
				}
			}
		}
	}
	return nil
}

// isStartupApprovedEnabled checks if the first byte of StartupApproved binary is enabled (0x02 = enabled, 0x03 = disabled).
func isStartupApprovedEnabled(approvedBytes []byte) bool {
	if len(approvedBytes) == 0 {
		return true // default enabled
	}
	// Even number or 0x02 = enabled; odd number (0x01, 0x03) = disabled
	return approvedBytes[0]%2 == 0
}

func readFolderStartupApproved(root registry.Key, m map[string][]byte) {
	k, err := registry.OpenKey(root, `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\StartupFolder`, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer k.Close()

	if valNames, vErr := k.ReadValueNames(0); vErr == nil {
		for _, vn := range valNames {
			if b, _, bErr := k.GetBinaryValue(vn); bErr == nil {
				low := strings.ToLower(vn)
				if existing, exists := m[low]; exists && !isStartupApprovedEnabled(existing) {
					continue
				}
				m[low] = b
			}
		}
	}
}

// collectRegistryRunItems reads Run keys and checks StartupApproved.
func collectRegistryRunItems(root registry.Key, rootName, location string) []BootStartupItem {
	k, err := registry.OpenKey(root, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()

	valNames, err := k.ReadValueNames(0)
	if err != nil {
		return nil
	}

	approvedKeyPath := `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`

	var items []BootStartupItem
	for _, name := range valNames {
		cmd, _, err := k.GetStringValue(name)
		if err != nil || strings.TrimSpace(cmd) == "" {
			continue
		}

		cleanCmd := strings.Trim(strings.TrimSpace(cmd), `"`)
		exeBase := filepath.Base(cleanCmd)
		if exeBase == "." || exeBase == "" {
			exeBase = name
		}

		approvedBytes := readStartupApprovedBytes(root, approvedKeyPath, name)
		isEnabled := isStartupApprovedEnabled(approvedBytes)

		status := "enabled"
		if !isEnabled {
			status = "disabled"
		}

		id := fmt.Sprintf("reg:%s:%s", rootName, name)
		items = append(items, BootStartupItem{
			ID:          id,
			Name:        name,
			Command:     cmd,
			Location:    location,
			Status:      status,
			IsProtected: isProtectedStartupItem(name, cmd),
		})
	}

	return items
}

// collectStartupFolderItems inspects shortcut and exe files in startup folders.
func collectStartupFolderItems(folderPath, location, idPrefix string) []BootStartupItem {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil
	}

	approvedBytesMap := map[string][]byte{}
	readFolderStartupApproved(registry.CURRENT_USER, approvedBytesMap)
	readFolderStartupApproved(registry.LOCAL_MACHINE, approvedBytesMap)

	var items []BootStartupItem
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lowName := strings.ToLower(name)
		if lowName == "desktop.ini" {
			continue
		}

		fullPath := filepath.Join(folderPath, name)
		displayName := strings.TrimSuffix(name, filepath.Ext(name))

		approvedBytes := approvedBytesMap[lowName]
		isEnabled := isStartupApprovedEnabled(approvedBytes)
		status := "enabled"
		if !isEnabled {
			status = "disabled"
		}

		id := fmt.Sprintf("%s:%s", idPrefix, name)
		items = append(items, BootStartupItem{
			ID:          id,
			Name:        displayName,
			Command:     fullPath,
			Location:    location,
			Status:      status,
			IsProtected: isProtectedStartupItem(name, fullPath),
		})
	}

	return items
}

// collectTaskSchedulerLogonItems queries logon tasks excluding Microsoft Windows system tasks.
func collectTaskSchedulerLogonItems() []BootStartupItem {
	cmd := exec.Command("schtasks", "/query", "/fo", "CSV", "/v")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil
	}

	r := csv.NewReader(bytes.NewReader(out))
	records, err := r.ReadAll()
	if err != nil || len(records) < 2 {
		return nil
	}

	header := records[0]
	taskNameIdx := -1
	taskToRunIdx := -1
	statusIdx := -1

	for i, h := range header {
		clean := strings.ToLower(strings.TrimSpace(h))
		if strings.Contains(clean, "taskname") || strings.Contains(clean, "имя задачи") {
			taskNameIdx = i
		} else if strings.Contains(clean, "task to run") || strings.Contains(clean, "действие") {
			taskToRunIdx = i
		} else if strings.Contains(clean, "status") || strings.Contains(clean, "состояние") {
			statusIdx = i
		}
	}

	if taskNameIdx == -1 {
		taskNameIdx = 0
	}

	var items []BootStartupItem
	for _, row := range records[1:] {
		if len(row) <= taskNameIdx {
			continue
		}
		taskName := row[taskNameIdx]
		if strings.HasPrefix(taskName, `\Microsoft\Windows\`) || strings.HasPrefix(taskName, `\Microsoft\Office\`) {
			continue
		}

		// Check if this is a PulseCore delayed task
		if strings.HasPrefix(taskName, `\PulseCore\Delayed_`) {
			continue // will be correlated
		}

		cmdRun := ""
		if taskToRunIdx >= 0 && len(row) > taskToRunIdx {
			cmdRun = row[taskToRunIdx]
		}
		if cmdRun == "" || strings.EqualFold(cmdRun, "N/A") {
			continue
		}

		status := "enabled"
		if statusIdx >= 0 && len(row) > statusIdx {
			st := strings.ToLower(row[statusIdx])
			if strings.Contains(st, "disabled") || strings.Contains(st, "отключен") {
				status = "disabled"
			}
		}

		baseName := filepath.Base(taskName)
		id := "task:" + taskName

		items = append(items, BootStartupItem{
			ID:          id,
			Name:        baseName,
			Command:     cmdRun,
			Location:    "task_scheduler",
			Status:      status,
			IsProtected: isProtectedStartupItem(baseName, cmdRun),
		})
	}

	return items
}

// collectDelayedTasks finds tasks created by PulseCore under \PulseCore\Delayed_*
func collectDelayedTasks() map[string]bool {
	cmd := exec.Command("schtasks", "/query", "/tn", `PulseCore\*`, "/fo", "LIST")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil
	}

	delayed := map[string]bool{}
	lines := strings.Split(string(out), "\n")
	re := regexp.MustCompile(`\\PulseCore\\Delayed_([^\r\n\s]+)`)

	for _, line := range lines {
		m := re.FindStringSubmatch(line)
		if len(m) > 1 {
			delayed[m[1]] = true
		}
	}

	return delayed
}

// GetBootDiagnostics analyzes Windows boot time, startup programs, and Fast Startup.
func (a *App) GetBootDiagnostics() (BootDiagnostics, error) {
	bootSec, kernelSec, desktopSec, degraders, hasLog := readBootEventsFromLog()

	rating := "fast"
	if !hasLog || bootSec <= 0.0 {
		rating = "unknown"
	} else if bootSec >= 50.0 {
		rating = "slow"
	} else if bootSec >= 25.0 {
		rating = "moderate"
	}

	// Fast Startup and Uptime check
	hiberboot := false
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Power`, registry.QUERY_VALUE); err == nil {
		if v, _, vErr := k.GetIntegerValue("HiberbootEnabled"); vErr == nil && v == 1 {
			hiberboot = true
		}
		k.Close()
	}

	uptimeMs := getSystemUptimeMs()
	uptimeHours := float64(uptimeMs) / (1000.0 * 3600.0)

	fastStartupWarning := false
	fastStartupDays := 0
	if hiberboot && uptimeHours >= 72.0 {
		fastStartupWarning = true
		fastStartupDays = int(uptimeHours / 24.0)
	}

	// Gather startup items
	var allItems []BootStartupItem
	allItems = append(allItems, collectRegistryRunItems(registry.CURRENT_USER, "hkcu", "registry_user")...)
	allItems = append(allItems, collectRegistryRunItems(registry.LOCAL_MACHINE, "hklm", "registry_machine")...)

	userStartup := filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Startup`)
	allItems = append(allItems, collectStartupFolderItems(userStartup, "startup_folder", "startup_user")...)

	commonStartup := filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs\Startup`)
	allItems = append(allItems, collectStartupFolderItems(commonStartup, "startup_folder", "startup_common")...)

	taskItems := collectTaskSchedulerLogonItems()
	allItems = append(allItems, taskItems...)

	// Correlate with PulseCore delayed tasks
	delayedMap := collectDelayedTasks()

	// Correlate with degraders
	degradeMap := map[string]float64{}
	for _, deg := range degraders {
		degradeMap[strings.ToLower(deg.Name)] = deg.TotalSec
	}

	// Deduplicate items by lower-case Name + Command
	seenMap := map[string]bool{}
	var filteredItems []BootStartupItem

	for _, item := range allItems {
		key := strings.ToLower(item.Name)
		if seenMap[key] {
			continue
		}
		seenMap[key] = true

		// Check if delayed
		safeID := sanitizeTaskName(item.ID)
		if delayedMap[safeID] {
			item.Status = "delayed"
			item.DelaySec = 60
		}

		// Check if matches degrader
		cmdBase := strings.ToLower(filepath.Base(strings.Trim(item.Command, `"`)))
		if sec, found := degradeMap[cmdBase]; found {
			item.DegradeSec = sec
		} else if sec, found := degradeMap[strings.ToLower(item.Name)]; found {
			item.DegradeSec = sec
		}

		filteredItems = append(filteredItems, item)
	}

	sortStartupItems(filteredItems)

	return BootDiagnostics{
		EventLogAvailable:  hasLog,
		BootTimeSec:        bootSec,
		KernelSec:          kernelSec,
		DesktopSec:         desktopSec,
		BootRating:         rating,
		FastStartupWarning: fastStartupWarning,
		FastStartupDays:    fastStartupDays,
		HiberbootEnabled:   hiberboot,
		UptimeHours:        float64(int(uptimeHours*10)) / 10,
		Degraders:          degraders,
		StartupItems:       filteredItems,
	}, nil
}

func sanitizeTaskName(id string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\-]`)
	return reg.ReplaceAllString(id, "_")
}

func startupItemPriority(item BootStartupItem) int {
	if !item.IsProtected && (item.Status == "enabled" || item.Status == "delayed") {
		return 0
	}
	if item.IsProtected {
		return 1
	}
	return 2
}

func sortStartupItems(items []BootStartupItem) {
	sort.SliceStable(items, func(i, j int) bool {
		pi := startupItemPriority(items[i])
		pj := startupItemPriority(items[j])
		if pi != pj {
			return pi < pj
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
}

// SetStartupItemMode changes startup behavior ("enabled", "delayed", "disabled").
func (a *App) SetStartupItemMode(id string, mode string) error {
	id = strings.TrimSpace(id)
	mode = strings.ToLower(strings.TrimSpace(mode))

	if id == "" {
		return fmt.Errorf("empty item id")
	}

	safeID := sanitizeTaskName(id)
	taskName := fmt.Sprintf(`\PulseCore\Delayed_%s`, safeID)

	parts := strings.SplitN(id, ":", 3)
	itemType := parts[0]

	switch itemType {
	case "reg":
		if len(parts) < 3 {
			return fmt.Errorf("invalid registry id format")
		}
		rootName := strings.ToLower(parts[1])
		valName := parts[2]

		var root registry.Key
		if rootName == "hkcu" {
			root = registry.CURRENT_USER
		} else {
			root = registry.LOCAL_MACHINE
		}

		approvedKeyPath := `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`

		// Check if protected
		k, err := registry.OpenKey(root, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
		if err != nil {
			return err
		}
		cmd, _, _ := k.GetStringValue(valName)
		k.Close()

		if isProtectedStartupItem(valName, cmd) {
			return fmt.Errorf("system component cannot be disabled")
		}

		switch mode {
		case "disabled":
			// Write disabled bytes (0x03)
			if err := writeStartupApproved(root, approvedKeyPath, valName, false); err != nil {
				return err
			}
			// Delete delayed task if present
			_ = exec.Command("schtasks", "/delete", "/tn", taskName, "/f").Run()

		case "delayed":
			// Disable in StartupApproved
			if err := writeStartupApproved(root, approvedKeyPath, valName, false); err != nil {
				return err
			}
			// Create delayed task for 60s
			if cmd != "" {
				cmdArgs := []string{"/create", "/tn", taskName, "/tr", cmd, "/sc", "onlogon", "/delay", "0001:00", "/f", "/rl", "limited"}
				c := exec.Command("schtasks", cmdArgs...)
				c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
				_ = c.Run()
			}

		case "enabled":
			// Delete delayed task
			_ = exec.Command("schtasks", "/delete", "/tn", taskName, "/f").Run()
			// Write enabled bytes (0x02)
			if err := writeStartupApproved(root, approvedKeyPath, valName, true); err != nil {
				return err
			}
		}

	case "startup_user", "startup_common":
		if len(parts) < 2 {
			return fmt.Errorf("invalid startup folder id")
		}
		fileName := parts[1]
		approvedKeyPath := `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\StartupFolder`

		if isProtectedStartupItem(fileName, fileName) {
			return fmt.Errorf("system component cannot be disabled")
		}

		switch mode {
		case "disabled":
			if err := writeStartupApproved(registry.CURRENT_USER, approvedKeyPath, fileName, false); err != nil {
				return err
			}
			_ = exec.Command("schtasks", "/delete", "/tn", taskName, "/f").Run()
		case "delayed":
			if err := writeStartupApproved(registry.CURRENT_USER, approvedKeyPath, fileName, false); err != nil {
				return err
			}
			targetFolder := filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Startup`)
			if itemType == "startup_common" {
				targetFolder = filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs\Startup`)
			}
			fullPath := filepath.Join(targetFolder, fileName)
			cmdArgs := []string{"/create", "/tn", taskName, "/tr", `"` + fullPath + `"`, "/sc", "onlogon", "/delay", "0001:00", "/f", "/rl", "limited"}
			c := exec.Command("schtasks", cmdArgs...)
			c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			_ = c.Run()
		case "enabled":
			_ = exec.Command("schtasks", "/delete", "/tn", taskName, "/f").Run()
			if err := writeStartupApproved(registry.CURRENT_USER, approvedKeyPath, fileName, true); err != nil {
				return err
			}
		}

	case "task":
		if len(parts) < 2 {
			return fmt.Errorf("invalid task id")
		}
		realTaskName := strings.TrimPrefix(id, "task:")
		if isProtectedStartupItem(realTaskName, realTaskName) {
			return fmt.Errorf("system task cannot be disabled")
		}

		switch mode {
		case "disabled":
			c := exec.Command("schtasks", "/change", "/tn", realTaskName, "/disable")
			c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			return c.Run()
		case "enabled":
			c := exec.Command("schtasks", "/change", "/tn", realTaskName, "/enable")
			c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			return c.Run()
		}
	}

	return nil
}

// writeStartupApproved writes standard Windows Task Manager enabled/disabled bytes.
func writeStartupApproved(root registry.Key, keyPath, valName string, enable bool) error {
	k, _, err := registry.CreateKey(root, keyPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		if root == registry.LOCAL_MACHINE {
			// Non-elevated: attempt elevated PowerShell via UAC
			valBytes := make([]byte, 12)
			if enable {
				valBytes[0] = 0x02
			} else {
				valBytes[0] = 0x03
			}
			ft := time.Now().UnixNano()/100 + 116444736000000000
			for i := 0; i < 8; i++ {
				valBytes[4+i] = byte((ft >> (i * 8)) & 0xFF)
			}
			byteList := make([]string, len(valBytes))
			for i, b := range valBytes {
				byteList[i] = strconv.Itoa(int(b))
			}
			bytesStr := strings.Join(byteList, ",")
			regPath := `HKLM:\` + keyPath
			psScript := fmt.Sprintf(`
$bytes = [byte[]]@(%s)
if (!(Test-Path -LiteralPath %s)) {
    New-Item -Path %s -Force | Out-Null
}
Set-ItemProperty -LiteralPath %s -Name %s -Value $bytes -Type Binary
`, bytesStr, quotePS(regPath), quotePS(regPath), quotePS(regPath), quotePS(valName))

			_, elevErr := runElevatedPowerShell(psScript)
			if elevErr != nil {
				return fmt.Errorf("требуются права администратора для изменения HKLM: %w", elevErr)
			}
			return nil
		}
		return fmt.Errorf("ошибка доступа к реестру: %w", err)
	}
	defer k.Close()

	cur, _, err := k.GetBinaryValue(valName)
	var data []byte
	if err == nil && len(cur) >= 12 {
		data = make([]byte, len(cur))
		copy(data, cur)
	} else {
		data = make([]byte, 12)
	}

	if enable {
		data[0] = 0x02 // Enabled
	} else {
		data[0] = 0x03 // Disabled
	}

	// Update timestamp bytes (standard Windows FILETIME in bytes 4..11)
	ft := time.Now().UnixNano() / 100
	ft += 116444736000000000
	for i := 0; i < 8 && i+4 < len(data); i++ {
		data[4+i] = byte((ft >> (i * 8)) & 0xFF)
	}

	return k.SetBinaryValue(valName, data)
}

// DisableFastStartup sets HiberbootEnabled to 0 in registry.
func (a *App) DisableFastStartup() error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Power`, registry.SET_VALUE)
	if err != nil {
		// Attempt elevated PowerShell
		cmd := exec.Command("powershell", "-NoProfile", "-Command",
			`Set-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Power' -Name 'HiberbootEnabled' -Value 0 -Type DWord`)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return cmd.Run()
	}
	defer k.Close()

	return k.SetDWordValue("HiberbootEnabled", 0)
}

// RebootSystemNow restarts the system cleanly.
func (a *App) RebootSystemNow() error {
	cmd := exec.Command("shutdown", "/r", "/t", "0")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}
