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

// AppUsageEntry is a session consumer of CPU / disk.
type AppUsageEntry struct {
	Name         string  `json:"name"`
	FriendlyName string  `json:"friendlyName"`
	TrafficMB    float64 `json:"trafficMB"`
	EnergymWh    float64 `json:"energymWh"`
	ActiveHours  float64 `json:"activeHours"`
	CPUSec       float64 `json:"cpuSec"`
	DiskMB       float64 `json:"diskMB"`
	Detail       string  `json:"detail"`
}

// MonthlyAppReport is the Session Impact summary shown on Overview.
type MonthlyAppReport struct {
	OK          bool            `json:"ok"`
	Period      string          `json:"period"`
	TopApps     []AppUsageEntry `json:"topApps"`
	Note        string          `json:"note"`
	WarmingUp   bool            `json:"warmingUp"`
	SessionSecs float64         `json:"sessionSecs"`
}

// LagDiagnosis explains whether lag comes from Wi-Fi, ISP or the PC.
type LagDiagnosis struct {
	GatewayHost    string  `json:"gatewayHost"`
	GatewayMs      float64 `json:"gatewayMs"`
	GatewayBadge   string  `json:"gatewayBadge"` // i18n key
	GatewayBadgeOK bool    `json:"gatewayBadgeOK"`
	PublicHost     string  `json:"publicHost"`
	PublicMs       float64 `json:"publicMs"`
	PublicBadge    string  `json:"publicBadge"`
	PublicBadgeOK  bool    `json:"publicBadgeOK"`
	LocalJitterMs  float64 `json:"localJitterMs"`
	JitterBadge    string  `json:"jitterBadge"`
	JitterBadgeOK  bool    `json:"jitterBadgeOK"`
	Verdict        string  `json:"verdict"`
	VerdictKey     string  `json:"verdictKey"`
	Level          string  `json:"level"` // wifi | provider | pc | ok
	OK             bool    `json:"ok"`
}

// SleepReport lists what prevents the laptop from sleeping.
type SleepReport struct {
	OK       bool     `json:"ok"`
	SleepOK  bool     `json:"sleepOK"`
	Summary  string   `json:"summary"`
	Blockers []string `json:"blockers"`
	RawHint  string   `json:"rawHint"`
}

// GetMonthlyAppReport returns top background impact for the current PulseCore session.
func (a *App) GetMonthlyAppReport() MonthlyAppReport {
	a.mu.RLock()
	start := a.sessionStart
	if start.IsZero() {
		start = time.Now()
	}
	elapsed := time.Since(start)
	hits := make([]*sessionHit, 0, len(a.sessionHits))
	for _, h := range a.sessionHits {
		cp := *h
		hits = append(hits, &cp)
	}
	a.mu.RUnlock()

	report := MonthlyAppReport{
		OK:          true,
		Period:      "текущий сеанс",
		TopApps:     []AppUsageEntry{},
		SessionSecs: elapsed.Seconds(),
	}
	if elapsed < 5*time.Minute {
		report.WarmingUp = true
		report.Note = "session_warming"
		return report
	}

	sort.Slice(hits, func(i, j int) bool {
		si := hits[i].CPUSec*1.5 + hits[i].DiskMB*0.25
		sj := hits[j].CPUSec*1.5 + hits[j].DiskMB*0.25
		return si > sj
	})
	limit := 5
	if len(hits) < limit {
		limit = len(hits)
	}
	for i := 0; i < limit; i++ {
		h := hits[i]
		detail := fmt.Sprintf("CPU %.0f с · диск %.1f МБ за сеанс", h.CPUSec, h.DiskMB)
		report.TopApps = append(report.TopApps, AppUsageEntry{
			Name:         h.Name,
			FriendlyName: h.FriendlyName,
			CPUSec:       h.CPUSec,
			DiskMB:       h.DiskMB,
			TrafficMB:    h.DiskMB,
			ActiveHours:  h.CPUSec / 3600.0,
			Detail:       detail,
		})
	}
	if len(report.TopApps) == 0 {
		report.Note = "session_empty"
	} else {
		report.Note = "session_impact"
	}
	return report
}

func collectUsageFromPowerReports() ([]AppUsageEntry, string) {
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("pulsecore-battery-%d.xml", time.Now().UnixNano()))
	defer os.Remove(tmp)

	// Prefer batteryreport XML (works on laptops without admin in most cases).
	_ = runHiddenCommand("powercfg", "/batteryreport", "/xml", "/output", tmp, "/duration", "7")
	if data, err := os.ReadFile(tmp); err == nil && len(data) > 100 {
		if apps := parseBatteryReportApps(data); len(apps) > 0 {
			return apps, "Данные из powercfg /batteryreport за 7 дней."
		}
	}

	sru := filepath.Join(os.TempDir(), fmt.Sprintf("pulsecore-sru-%d.xml", time.Now().UnixNano()))
	defer os.Remove(sru)
	_ = runHiddenCommand("powercfg", "/srumutil", "/OUTPUT", sru, "/XML")
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

// CheckSleepBlockers parses powercfg /requests for sleep inhibitors.
func (a *App) CheckSleepBlockers() SleepReport {
	rep := SleepReport{OK: true}

	// PowerShell returns Unicode text reliably (avoids OEM CP866 console mangling).
	out, err := runHiddenOutput("powershell", "-NoProfile", "-Command", "powercfg /requests")
	if err != nil && len(out) == 0 {
		out, err = runHiddenOutput("powershell", "-NoProfile", "-NonInteractive", "-Command",
			"[Console]::OutputEncoding=[Text.Encoding]::UTF8; powercfg /requests | Out-String")
	}
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
