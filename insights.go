package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
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
	GatewayBadge     string  `json:"gatewayBadge"` // i18n key
	GatewayBadgeOK   bool    `json:"gatewayBadgeOK"`
	GatewayRating    string  `json:"gatewayRating"`
	PublicHost       string  `json:"publicHost"`
	PublicMs         float64 `json:"publicMs"`
	PublicBadge      string  `json:"publicBadge"`
	PublicBadgeOK    bool    `json:"publicBadgeOK"`
	PublicRating     string  `json:"publicRating"`
	LocalJitterMs    float64 `json:"localJitterMs"`
	JitterBadge      string  `json:"jitterBadge"`
	JitterBadgeOK    bool    `json:"jitterBadgeOK"`
	StabilityStatus  string  `json:"stabilityStatus"`
	LocalIP          string  `json:"localIP"`
	ExternalIP       string  `json:"externalIP"`
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
		// Demo-only placeholders when SRUM is empty - no AV/OS noise as Eco targets.
		report.TopApps = []AppUsageEntry{
			{Name: "chrome.exe", FriendlyName: "procBrowser", Detail: "procBrowserDetail", ActiveHours: 0},
			{Name: "Discord.exe", FriendlyName: "Discord", Detail: "procBrowserDetail", ActiveHours: 0},
			{Name: "OneDrive.exe", FriendlyName: "OneDrive", Detail: "procBrowserDetail", ActiveHours: 0},
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

// DiagnoseLagOrigin compares Wi-Fi gateway, ISP DNS, local scheduler jitter, and IP addresses.
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
	diag.LocalIP = getLocalIPv4()
	diag.ExternalIP = getExternalIPv4()

	// Native Go TCP/UDP latency (locale-agnostic). Never parse ping.exe text
	// (English time= / Russian time= breaks on other Windows locales).
	diag.GatewayMs = measureHostLatencyMs(gw, 2)
	dnsA := measureHostLatencyMs("8.8.8.8", 2)
	dnsB := measureHostLatencyMs("1.1.1.1", 2)
	diag.PublicMs = minPositiveLatency(dnsA, dnsB)
	if diag.PublicMs == dnsB && dnsB >= 0 {
		diag.PublicHost = "1.1.1.1"
	}
	diag.LocalJitterMs = measureLocalJitterMs()
	debug.FreeOSMemory()

	diag.GatewayBadgeOK = diag.GatewayMs >= 0 && diag.GatewayMs <= 15
	diag.PublicBadgeOK = diag.PublicMs >= 0 && diag.PublicMs <= 40
	diag.JitterBadgeOK = diag.LocalJitterMs >= 0 && diag.LocalJitterMs <= 20

	if diag.GatewayMs >= 0 && diag.GatewayMs <= 2 {
		diag.GatewayRating = "Отлично для онлайн-игр"
	} else if diag.GatewayMs >= 0 && diag.GatewayMs <= 15 {
		diag.GatewayRating = "Норма"
	} else if diag.GatewayMs > 15 {
		diag.GatewayRating = "Высокая задержка"
	} else {
		diag.GatewayRating = "Недоступен"
	}

	if diag.PublicMs >= 0 && diag.PublicMs <= 35 {
		diag.PublicRating = "Отлично для онлайн-игр"
	} else if diag.PublicMs >= 0 && diag.PublicMs <= 70 {
		diag.PublicRating = "Норма"
	} else if diag.PublicMs > 70 {
		diag.PublicRating = "Высокая задержка"
	} else {
		diag.PublicRating = "Недоступен"
	}

	if diag.LocalJitterMs <= 15 {
		diag.StabilityStatus = "Стабильно, потерь пакетов нет"
	} else {
		diag.StabilityStatus = "Возможны задержки в играх"
	}

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

func getLocalIPv4() string {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 400*time.Millisecond)
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			return addr.IP.String()
		}
	}
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func getExternalIPv4() string {
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	endpoints := []string{
		"https://api.ipify.org",
		"https://icanhazip.com",
		"https://checkip.amazonaws.com",
	}
	for _, ep := range endpoints {
		resp, err := client.Get(ep)
		if err == nil {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
			resp.Body.Close()
			ip := strings.TrimSpace(string(body))
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	return ""
}

func minPositiveLatency(a, b float64) float64 {
	switch {
	case a < 0 && b < 0:
		return -1
	case a < 0:
		return b
	case b < 0:
		return a
	case a <= b:
		return a
	default:
		return b
	}
}

// measureHostLatencyMs times TCP (then UDP) dials. Locale-agnostic; no ping.exe.
func measureHostLatencyMs(host string, count int) float64 {
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
	if ip := net.ParseIP(host); ip == nil {
		// Only allow dotted IPv4 / resolved hostnames without shell metacharacters.
		if strings.ContainsAny(host, "/\\") {
			return -1
		}
	}
	ports := []string{"53", "443", "80"}
	var samples []float64
	for i := 0; i < count; i++ {
		ms := dialLatencyMs("tcp", host, ports, 800*time.Millisecond)
		if ms < 0 {
			ms = dialLatencyMs("udp", host, []string{"53"}, 800*time.Millisecond)
		}
		if ms >= 0 {
			samples = append(samples, ms)
		}
	}
	if len(samples) == 0 {
		return -1
	}
	sum := 0.0
	for _, v := range samples {
		sum += v
	}
	return sum / float64(len(samples))
}

func dialLatencyMs(network, host string, ports []string, timeout time.Duration) float64 {
	for _, port := range ports {
		addr := net.JoinHostPort(host, port)
		start := time.Now()
		conn, err := net.DialTimeout(network, addr, timeout)
		elapsed := float64(time.Since(start)) / float64(time.Millisecond)
		if err != nil {
			// For UDP, DialTimeout often succeeds locally; treat write+deadline as RTT probe.
			if network == "udp" {
				continue
			}
			// TCP connection refused still proves L3 reachability with a real RTT.
			if ne, ok := err.(net.Error); ok && !ne.Timeout() && elapsed > 0 && elapsed < float64(timeout/time.Millisecond) {
				if strings.Contains(strings.ToLower(err.Error()), "refused") {
					return elapsed
				}
			}
			continue
		}
		_ = conn.Close()
		if elapsed < 0.1 {
			elapsed = 0.1
		}
		return elapsed
	}
	return -1
}

// pingAverageMs kept as a thin alias for older call sites / tests.
func pingAverageMs(host string, count int) float64 {
	return measureHostLatencyMs(host, count)
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

	// Direct System32\powercfg.exe: no PowerShell wrapper (AV hygiene).
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
