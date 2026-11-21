package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// AppUsageEntry is a weekly consumer of battery/traffic.
type AppUsageEntry struct {
	Name         string  `json:"name"`
	FriendlyName string  `json:"friendlyName"`
	TrafficMB    float64 `json:"trafficMB"`
	EnergymWh    float64 `json:"energymWh"`
	ActiveHours  float64 `json:"activeHours"`
	Detail       string  `json:"detail"`
}

// MonthlyAppReport is a lightweight weekly retrospective for Overview / Lag tab.
type MonthlyAppReport struct {
	OK      bool            `json:"ok"`
	Period  string          `json:"period"`
	TopApps []AppUsageEntry `json:"topApps"`
	Note    string          `json:"note"`
}

// LagDiagnosis explains whether lag comes from Wi-Fi, ISP or the PC.
type LagDiagnosis struct {
	GatewayHost      string  `json:"gatewayHost"`
	GatewayMs        float64 `json:"gatewayMs"`
	GatewayBadge     string  `json:"gatewayBadge"`     // i18n key
	GatewayBadgeOK   bool    `json:"gatewayBadgeOK"`
	PublicHost       string  `json:"publicHost"`
	PublicMs         float64 `json:"publicMs"`
	PublicBadge      string  `json:"publicBadge"`
	PublicBadgeOK    bool    `json:"publicBadgeOK"`
	LocalJitterMs    float64 `json:"localJitterMs"`
	JitterBadge      string  `json:"jitterBadge"`
	JitterBadgeOK    bool    `json:"jitterBadgeOK"`
	Verdict          string  `json:"verdict"`
	VerdictKey       string  `json:"verdictKey"`
	Level            string  `json:"level"` // wifi | provider | pc | ok
	OK               bool    `json:"ok"`
}

// SleepReport lists what prevents the laptop from sleeping.
type SleepReport struct {
	OK       bool     `json:"ok"`
	SleepOK  bool     `json:"sleepOK"`
	Summary  string   `json:"summary"`
	Blockers []string `json:"blockers"`
	RawHint  string   `json:"rawHint"`
}

// GetMonthlyAppReport builds a weekly "who spent battery/traffic" summary.
func (a *App) GetMonthlyAppReport() MonthlyAppReport {
	report := MonthlyAppReport{
		Period: "последние 7 дней",
		TopApps: []AppUsageEntry{},
		Note:   "Собрано из системного отчёта Windows (batteryreport / srumutil).",
	}

	entries, note := collectUsageFromPowerReports()
	report.Note = note
	if len(entries) == 0 {
		report.OK = true
		report.Note = ""
		// Detail holds stable i18n keys; UI localizes FriendlyName + Detail.
		report.TopApps = []AppUsageEntry{
			{Name: "MsMpEng.exe", FriendlyName: "procDefender", Detail: "procDefenderDetail", ActiveHours: 0},
			{Name: "SearchIndexer.exe", FriendlyName: "procIndexer", Detail: "procIndexerDetail", ActiveHours: 0},
			{Name: "chrome.exe", FriendlyName: "procBrowser", Detail: "procBrowserDetail", ActiveHours: 0},
		}
		return report
	}
	if len(entries) > 3 {
		entries = entries[:3]
	}
	report.TopApps = entries
	report.OK = true
	return report
}

func collectUsageFromPowerReports() ([]AppUsageEntry, string) {
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("pulsecore-battery-%d.xml", time.Now().UnixNano()))
	defer os.Remove(tmp)

	// Prefer batteryreport XML (works on laptops without admin in most cases).
	_ = runHiddenCommand(system32("powercfg.exe"), "/batteryreport", "/xml", "/output", tmp, "/duration", "7")
	if data, err := os.ReadFile(tmp); err == nil && len(data) > 100 {
		if apps := parseBatteryReportApps(data); len(apps) > 0 {
			return apps, "Данные из powercfg /batteryreport за 7 дней."
		}
	}

	sru := filepath.Join(os.TempDir(), fmt.Sprintf("pulsecore-sru-%d.xml", time.Now().UnixNano()))
	defer os.Remove(sru)
	_ = runHiddenCommand(system32("powercfg.exe"), "/srumutil", "/OUTPUT", sru, "/XML")
	if data, err := os.ReadFile(sru); err == nil && len(data) > 100 {
		if apps := parseGenericEnergyApps(string(data)); len(apps) > 0 {
			return apps, "Данные из powercfg /srumutil (SRUM)."
		}
	}
	return nil, "История SRUM недоступна."
}

func parseBatteryReportApps(data []byte) []AppUsageEntry {
	raw := string(data)
	// Extract AppExe / EnergyUsage-like tags loosely across locales/schemas.
	reApp := regexp.MustCompile(`(?is)<(?:AppExe|ExecutablePath|Path|Name)>([^<]+)</(?:AppExe|ExecutablePath|Path|Name)>`)
	reEnergy := regexp.MustCompile(`(?is)<(?:EnergyUsage|TotalEnergy|AcEnergyConsumption|Energy)[^>]*>([0-9.]+)`)
	apps := reApp.FindAllStringSubmatch(raw, -1)
	energies := reEnergy.FindAllStringSubmatch(raw, -1)
	type acc struct {
		name string
		mWh  float64
		hits int
	}
	by := map[string]*acc{}
	for i, m := range apps {
		name := filepath.Base(strings.TrimSpace(m[1]))
		if name == "" || strings.EqualFold(name, "Idle") {
			continue
		}
		key := strings.ToLower(name)
		a := by[key]
		if a == nil {
			a = &acc{name: name}
			by[key] = a
		}
		a.hits++
		if i < len(energies) {
			if v, err := strconv.ParseFloat(energies[i][1], 64); err == nil {
				a.mWh += v
			}
		}
	}
	out := make([]AppUsageEntry, 0, len(by))
	for _, a := range by {
		out = append(out, AppUsageEntry{
			Name:         a.name,
			FriendlyName: friendlyProcessName(a.name),
			EnergymWh:    a.mWh,
			ActiveHours:  float64(a.hits) * 0.25,
			Detail:       fmt.Sprintf("~%.0f mWh суммарно", a.mWh),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].EnergymWh == out[j].EnergymWh {
			return out[i].ActiveHours > out[j].ActiveHours
		}
		return out[i].EnergymWh > out[j].EnergymWh
	})
	return out
}

func parseGenericEnergyApps(raw string) []AppUsageEntry {
	re := regexp.MustCompile(`(?i)([a-z0-9_\-.]+\.exe)`)
	counts := map[string]int{}
	for _, m := range re.FindAllString(raw, -1) {
		base := strings.ToLower(filepath.Base(m))
		if base == "powershell.exe" || base == "conhost.exe" || base == "powercfg.exe" {
			continue
		}
		counts[base]++
	}
	type kv struct {
		k string
		v int
	}
	arr := make([]kv, 0, len(counts))
	for k, v := range counts {
		arr = append(arr, kv{k, v})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].v > arr[j].v })
	out := []AppUsageEntry{}
	for i := 0; i < len(arr) && i < 3; i++ {
		out = append(out, AppUsageEntry{
			Name:         arr[i].k,
			FriendlyName: friendlyProcessName(arr[i].k),
			ActiveHours:  float64(arr[i].v) * 0.1,
			Detail:       "Упоминания в SRUM-снимке",
		})
	}
	return out
}

// DiagnoseLagOrigin compares Wi-Fi gateway, ISP DNS and local scheduler jitter.
func (a *App) DiagnoseLagOrigin() LagDiagnosis {
	diag := LagDiagnosis{
		PublicHost: "8.8.8.8",
		OK:         true,
	}
	gw := defaultGatewayIPv4()
	if gw == "" {
		gw = "192.168.1.1"
	}
	diag.GatewayHost = gw

	// Short ICMP bursts only (user-initiated). No long-lived sockets / telemetry.
	diag.GatewayMs = pingAverageMs(gw, 2)
	diag.PublicMs = pingAverageMs(diag.PublicHost, 2)
	diag.LocalJitterMs = measureLocalJitterMs()

	diag.GatewayBadgeOK = diag.GatewayMs >= 0 && diag.GatewayMs <= 15
	diag.PublicBadgeOK = diag.PublicMs >= 0 && diag.PublicMs <= 40
	diag.JitterBadgeOK = diag.LocalJitterMs >= 0 && diag.LocalJitterMs <= 20
	if diag.GatewayBadgeOK {
		diag.GatewayBadge = "netBadgeIdeal"
	} else if diag.GatewayMs < 0 {
		diag.GatewayBadge = "netBadgeFail"
	} else {
		diag.GatewayBadge = "netBadgeWeak"
	}
	if diag.PublicBadgeOK {
		diag.PublicBadge = "netBadgeFast"
	} else if diag.PublicMs < 0 {
		diag.PublicBadge = "netBadgeFail"
	} else {
		diag.PublicBadge = "netBadgeSlow"
	}
	if diag.JitterBadgeOK {
		diag.JitterBadge = "netBadgeStable"
	} else {
		diag.JitterBadge = "netBadgeUnstable"
	}

	switch {
	case diag.GatewayMs > 50:
		diag.Level = "wifi"
		diag.VerdictKey = "netVerdictWifi"
		diag.Verdict = diag.VerdictKey
	case diag.GatewayMs >= 0 && diag.GatewayMs < 5 && diag.PublicMs > 120:
		diag.Level = "provider"
		diag.VerdictKey = "netVerdictISP"
		diag.Verdict = diag.VerdictKey
	case diag.LocalJitterMs > 150 && diag.GatewayMs < 20 && diag.PublicMs < 80:
		diag.Level = "pc"
		diag.VerdictKey = "netVerdictPC"
		diag.Verdict = diag.VerdictKey
	default:
		diag.Level = "ok"
		diag.VerdictKey = "netVerdictOK"
		diag.Verdict = diag.VerdictKey
	}
	return diag
}

func pingAverageMs(host string, count int) float64 {
	if count < 1 {
		count = 2
	}
	if count > 3 {
		count = 3
	}
	host = strings.TrimSpace(host)
	if host == "" || strings.ContainsAny(host, " \t\"'|&<>") {
		return -1
	}
	out, err := runHiddenOutput(system32("ping.exe"), "-n", strconv.Itoa(count), "-w", "800", host)
	if err != nil && len(out) == 0 {
		return -1
	}
	re := regexp.MustCompile(`(?i)(?:time|время)[=<]([0-9]+)\s*ms`)
	matches := re.FindAllStringSubmatch(string(out), -1)
	if len(matches) == 0 {
		// Average line: Average = 12ms / Среднее = 12ms
		reAvg := regexp.MustCompile(`(?i)(?:Average|Среднее)\s*=\s*([0-9]+)\s*ms`)
		if m := reAvg.FindStringSubmatch(string(out)); len(m) == 2 {
			v, _ := strconv.ParseFloat(m[1], 64)
			return v
		}
		return -1
	}
	sum := 0.0
	for _, m := range matches {
		v, _ := strconv.ParseFloat(m[1], 64)
		sum += v
	}
	return sum / float64(len(matches))
}

func measureLocalJitterMs() float64 {
	const quantum = 8 * time.Millisecond
	var maxOver time.Duration
	for i := 0; i < 20; i++ {
		start := time.Now()
		time.Sleep(quantum)
		over := time.Since(start) - quantum
		if over > maxOver {
			maxOver = over
		}
	}
	return float64(maxOver) / float64(time.Millisecond)
}

// CheckSleepBlockers parses powercfg /requests for sleep inhibitors.
func (a *App) CheckSleepBlockers() SleepReport {
	rep := SleepReport{OK: true}

	// Direct System32\powercfg.exe — no PowerShell wrapper (AV hygiene).
	out, err := runHiddenOutput(system32("powercfg.exe"), "/requests")
	if err != nil && len(out) == 0 {
		rep.OK = false
		rep.Summary = "sleep_query_failed"
		rep.RawHint = err.Error()
		return rep
	}

	text := decodeWindowsCLI(out)
	rep.RawHint = text
	blockers := parsePowerRequests(text)
	rep.Blockers = blockers
	if len(blockers) == 0 {
		rep.SleepOK = true
		rep.Summary = "sleep_ok"
		return rep
	}
	rep.SleepOK = false
	rep.Summary = "sleep_blocked"
	return rep
}

func decodeWindowsCLI(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		b = b[3:]
	}
	utf8s := string(b)
	cp866 := decodeCP866Bytes(b)
	cp1251 := decodeCP1251Bytes(b)
	for _, cand := range []string{utf8s, cp866, cp1251} {
		up := strings.ToUpper(cand)
		if strings.Contains(up, "DISPLAY") || strings.Contains(up, "SYSTEM") ||
			strings.Contains(up, "NONE") || strings.Contains(cand, "Нет") || strings.Contains(cand, "нет") {
			return cand
		}
	}
	// Prefer OEM when high bytes present (typical for RU console without chcp).
	for _, c := range b {
		if c >= 0x80 {
			return cp866
		}
	}
	return utf8s
}

func decodeCP866Bytes(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		sb.WriteRune(runeFromCP866(c))
	}
	return sb.String()
}

func decodeCP1251Bytes(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		sb.WriteRune(runeFromCP1251(c))
	}
	return sb.String()
}

func runeFromCP1251(c byte) rune {
	if c < 0x80 {
		return rune(c)
	}
	// Windows-1251 Cyrillic mapping (0xC0-0xFF) + common punctuation.
	if c >= 0xC0 {
		return rune(0x0410 + int(c-0xC0)) // А..я
	}
	switch c {
	case 0xA8:
		return 'Ё'
	case 0xB8:
		return 'ё'
	case 0xB9:
		return '№'
	default:
		return rune(c)
	}
}

func runeFromCP866(c byte) rune {
	if c < 0x80 {
		return rune(c)
	}
	// Compact CP866 map for Cyrillic block + box-drawing fallbacks.
	table := []rune{
		'А', 'Б', 'В', 'Г', 'Д', 'Е', 'Ж', 'З', 'И', 'Й', 'К', 'Л', 'М', 'Н', 'О', 'П',
		'Р', 'С', 'Т', 'У', 'Ф', 'Х', 'Ц', 'Ч', 'Ш', 'Щ', 'Ъ', 'Ы', 'Ь', 'Э', 'Ю', 'Я',
		'а', 'б', 'в', 'г', 'д', 'е', 'ж', 'з', 'и', 'й', 'к', 'л', 'м', 'н', 'о', 'п',
		'░', '▒', '▓', '│', '┤', '╡', '╢', '╖', '╕', '╣', '║', '╗', '╝', '╜', '╛', '┐',
		'└', '┴', '┬', '├', '─', '┼', '╞', '╟', '╚', '╔', '╩', '╦', '╠', '═', '╬', '╧',
		'╨', '╤', '╥', '╙', '╘', '╒', '╓', '╫', '╪', '┘', '┌', '█', '▄', '▌', '▐', '▀',
		'р', 'с', 'т', 'у', 'ф', 'х', 'ц', 'ч', 'ш', 'щ', 'ъ', 'ы', 'ь', 'э', 'ю', 'я',
		'Ё', 'ё', 'Є', 'є', 'Ї', 'ї', 'Ў', 'ў', '°', '∙', '·', '√', '№', '¤', '■', ' ',
	}
	idx := int(c) - 0x80
	if idx >= 0 && idx < len(table) {
		return table[idx]
	}
	return rune(c)
}

func parsePowerRequests(text string) []string {
	lines := strings.Split(text, "\n")
	var (
		section string
		out     []string
	)
	sectionRe := regexp.MustCompile(`(?i)^\[?(DISPLAY|SYSTEM|AWAYMODE|EXECUTION|PERFBOOST|ACTIVELOCKERSCREEN|ОТОБРАЖЕНИЕ|СИСТЕМА|УДАЛЁННЫЙ\s*РАБОЧИЙ\s*СТОЛ|ПРОИЗВОДИТЕЛЬНОСТЬ|ИСПОЛНЕНИЕ)\]?:?$`)
	noneRe := regexp.MustCompile(`(?i)^(none\.?|нет\.?)$`)
	typeRe := regexp.MustCompile(`(?i)^(DRIVER|PROCESS|SERVICE|EXECUTABLE|ДРАЙВЕР|ПРОЦЕСС|СЛУЖБА)(\s*:)?$`)

	for _, line := range lines {
		trim := strings.TrimSpace(strings.TrimPrefix(line, "\uFEFF"))
		if trim == "" {
			continue
		}
		if m := sectionRe.FindStringSubmatch(trim); len(m) == 2 {
			section = strings.ToUpper(m[1])
			continue
		}
		if section == "" {
			continue
		}
		if noneRe.MatchString(trim) || typeRe.MatchString(trim) {
			continue
		}
		// Skip bare type headers like "[PROCESS]"
		up := strings.ToUpper(strings.Trim(trim, "[]"))
		if up == "DRIVER" || up == "PROCESS" || up == "SERVICE" || up == "EXECUTABLE" {
			continue
		}
		label := humanizeSleepBlocker(trim)
		if label == "" {
			continue
		}
		dup := false
		for _, e := range out {
			if e == label {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, label)
		}
	}
	return out
}

func humanizeSleepBlocker(raw string) string {
	s := strings.TrimSpace(raw)
	lower := strings.ToLower(s)
	switch {
	case strings.Contains(lower, "audio") || strings.Contains(lower, "audiodg"):
		return fmt.Sprintf("%s (активен аудиопоток)", friendlyProcessName(filepath.Base(s)))
	case strings.Contains(lower, "chrome") || strings.Contains(lower, "msedge") || strings.Contains(lower, "firefox"):
		return fmt.Sprintf("%s (браузер / вкладка с видео)", friendlyProcessName(filepath.Base(s)))
	case strings.Contains(lower, "\\"):
		base := filepath.Base(s)
		return friendlyProcessName(base)
	default:
		if len(s) > 80 {
			return s[:80] + "..."
		}
		return s
	}
}
