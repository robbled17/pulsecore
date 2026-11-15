package main

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)


// DiagnosticReportPreview lists what will / will not be exported (shown before export).
type DiagnosticReportPreview struct {
	Includes []string `json:"includes"`
	Excludes []string `json:"excludes"`
	Formats  []string `json:"formats"`
}

// DiagnosticReport is the local diagnostic export payload (no personal files/tokens).
type DiagnosticReport struct {
	GeneratedAt      string         `json:"generatedAt"`
	PulseCoreVersion string         `json:"pulseCoreVersion"`
	WindowsVersion   string         `json:"windowsVersion"`
	Arch             string         `json:"arch"`
	CPUName          string         `json:"cpuName"`
	LogicalCPUs      int            `json:"logicalCPUs"`
	GPUName          string         `json:"gpuName"`
	RAMTotalMB       uint64         `json:"ramTotalMB"`
	RAMUsedMB        uint64         `json:"ramUsedMB"`
	TemperatureC     float64        `json:"temperatureC"`
	TempAvailable    bool           `json:"tempAvailable"`
	GPUAvailable     bool           `json:"gpuAvailable"`
	HealthScore      int            `json:"healthScore"`
	HealthIncomplete bool           `json:"healthIncomplete"`
	Incidents        []IncidentCard `json:"incidents"`
	TopCPU           []ProcessSnap  `json:"topCPU"`
	TopRAM           []ProcessSnap  `json:"topRAM"`
	EcoHistoryCount  int            `json:"ecoHistoryCount"`
	EcoLimitedCount  int            `json:"ecoLimitedCount"`
	Unavailable      []string       `json:"unavailableMetrics"`
	CollectorNotes   []string       `json:"collectorNotes"`
	AnomalyCount     int            `json:"anomalyCount"`
	MemoryWatchCount int            `json:"memoryWatchCount"`
	// Network fields are masked (no full IPs / SSIDs).
	NetConnType    string  `json:"netConnType,omitempty"`
	NetLocalIP     string  `json:"netLocalIPMasked,omitempty"`
	NetGateway     string  `json:"netGatewayMasked,omitempty"`
	NetInternetMs  float64 `json:"netInternetMs,omitempty"`
	NetWifiSSID    string  `json:"netWifiSSIDMasked,omitempty"`
}

// DiagnosticExport is returned to the UI for download (content stays local).
type DiagnosticExport struct {
	OK       bool   `json:"ok"`
	Format   string `json:"format"`
	Filename string `json:"filename"`
	Content  string `json:"content"`
	MessageRu string `json:"messageRu"`
	MessageEn string `json:"messageEn"`
}

// PreviewDiagnosticReport explains export scope before the user confirms.
func (a *App) PreviewDiagnosticReport() DiagnosticReportPreview {
	lang := "ru"
	if a != nil {
		a.mu.RLock()
		lang = a.language
		a.mu.RUnlock()
	}
	if strings.EqualFold(lang, "en") {
		return DiagnosticReportPreview{
			Includes: []string{
				"PulseCore version",
				"Windows version / architecture",
				"CPU / GPU / RAM summary",
				"Sensor availability",
				"Micro-stutter incidents (≤5)",
				"Top processes by load (names only)",
				"Eco Mode session counts",
				"System Health score",
				"Unavailable metrics list",
			},
			Excludes: []string{
				"Personal documents and files are NOT included",
				"Passwords and tokens are NOT included",
				"Browser history",
				"Full user profile paths",
				"Packet contents / visited URLs",
			},
			Formats: []string{"txt", "json"},
		}
	}
	return DiagnosticReportPreview{
		Includes: []string{
			"Версия PulseCore",
			"Версия Windows / Архитектура",
			"Сводка CPU / GPU / RAM",
			"Доступность датчиков",
			"Инциденты микрофризов (≤5)",
			"Топ процессов по нагрузке (только имена)",
			"Счётчики сессий Eco Mode",
			"Оценка System Health",
			"Список недоступных метрик",
		},
		Excludes: []string{
			"Личные документы и файлы НЕ включаются",
			"Пароли и токены НЕ включаются",
			"История браузера",
			"Полные пути профиля пользователя",
			"Содержимое пакетов / посещённые URL",
		},
		Formats: []string{"txt", "json"},
	}
}

// BuildDiagnosticReport assembles a privacy-conscious local report.
func (a *App) BuildDiagnosticReport() DiagnosticReport {
	a.mu.RLock()
	m := a.metrics
	cpuName := a.cpuName
	gpuName := a.gpuName
	tempMissing := a.tempUnavailable
	gpuMissing := a.nvidiaMissing
	a.mu.RUnlock()

	if cpuName == "" {
		cpuName = m.CPUName
	}
	if gpuName == "" {
		gpuName = m.GPUName
	}

	unavail := []string{}
	notes := []string{}
	tempOK := m.TemperatureC > 0 && !tempMissing
	if !tempOK {
		unavail = append(unavail, "cpu_temperature")
		notes = append(notes, "CPU temperature sensor unavailable or estimated")
	}
	if gpuMissing || gpuName == "" {
		unavail = append(unavail, "gpu_metrics")
		notes = append(notes, "GPU metrics unavailable (no NVIDIA NVML or unsupported adapter)")
	}

	incidents := a.GetLagIncidents()
	if len(incidents) > 5 {
		incidents = incidents[:5]
	}
	// Strip evidence paths if any look like home dirs - keep verdict text only for privacy.
	cleanInc := make([]IncidentCard, len(incidents))
	for i, c := range incidents {
		c.EvidenceRaw = nil
		cleanInc[i] = c
	}

	topCPU, _, topRAM := sampleProcessTops(5, 0, 5)
	for i := range topCPU {
		topCPU[i].Name = filepathBaseSafe(topCPU[i].Name)
	}
	for i := range topRAM {
		topRAM[i].Name = filepathBaseSafe(topRAM[i].Name)
	}

	health := a.GetSystemHealth()
	ecoHist := a.GetEcoHistory()
	ecoLim := a.GetEcoLimitedGroups()
	anom := a.GetProcessAnomalies()
	memW := a.GetMemoryWatchCards()

	var netConn, netIP, netGW, netSSID string
	var netMs float64
	if a != nil && a.netCheck != nil {
		a.netCheck.mu.Lock()
		nd := a.netCheck.detail
		a.netCheck.mu.Unlock()
		if nd.Checked {
			netConn = nd.ConnType
			netIP = maskIPForReport(nd.LocalIP)
			netGW = maskIPForReport(nd.Gateway)
			netSSID = maskSSIDForReport(nd.WifiSSID)
			netMs = nd.InternetMs
		}
	}

	return DiagnosticReport{
		GeneratedAt:      time.Now().Format(time.RFC3339),
		PulseCoreVersion: AppVersion,
		WindowsVersion:   windowsVersionString(),
		Arch:             runtime.GOARCH,
		CPUName:          cpuName,
		LogicalCPUs:      runtime.NumCPU(),
		GPUName:          gpuName,
		RAMTotalMB:       m.RAMTotalMB,
		RAMUsedMB:        m.RAMUsedMB,
		TemperatureC:     m.TemperatureC,
		TempAvailable:    tempOK,
		GPUAvailable:     !gpuMissing && gpuName != "",
		HealthScore:      health.Score,
		HealthIncomplete: health.Incomplete,
		Incidents:        cleanInc,
		TopCPU:           topCPU,
		TopRAM:           topRAM,
		EcoHistoryCount:  len(ecoHist),
		EcoLimitedCount:  len(ecoLim),
		Unavailable:      unavail,
		CollectorNotes:   notes,
		AnomalyCount:     len(anom),
		MemoryWatchCount: len(memW),
		NetConnType:      netConn,
		NetLocalIP:       netIP,
		NetGateway:       netGW,
		NetInternetMs:    netMs,
		NetWifiSSID:      netSSID,
	}
}

// GenerateDiagnosticReport returns the full plain-text diagnostic report suitable for copying to clipboard.
func (a *App) GenerateDiagnosticReport() string {
	rep := a.BuildDiagnosticReport()
	return reportToTXT(rep)
}

// ExportDiagnosticReport builds TXT or JSON content for local save.
func (a *App) ExportDiagnosticReport(format string) DiagnosticExport {
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "json" && format != "txt" {
		return DiagnosticExport{
			OK: false, Format: format,
			MessageRu: "Формат: txt или json.",
			MessageEn: "Format must be txt or json.",
		}
	}
	rep := a.BuildDiagnosticReport()
	stamp := time.Now().Format("20060102-150405")
	if format == "json" {
		b, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return DiagnosticExport{OK: false, Format: format, MessageRu: err.Error(), MessageEn: err.Error()}
		}
		return DiagnosticExport{
			OK: true, Format: "json",
			Filename: fmt.Sprintf("pulsecore-report-%s.json", stamp),
			Content:  string(b),
			MessageRu: "JSON-отчёт готов.",
			MessageEn: "JSON report ready.",
		}
	}
	return DiagnosticExport{
		OK: true, Format: "txt",
		Filename: fmt.Sprintf("pulsecore-report-%s.txt", stamp),
		Content:  reportToTXT(rep),
		MessageRu: "TXT-отчёт готов.",
		MessageEn: "TXT report ready.",
	}
}

func filepathBaseSafe(name string) string {
	name = strings.ReplaceAll(name, `\`, `/`)
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[i+1:]
	}
	return name
}

func windowsVersionString() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return "Windows (unknown)"
	}
	defer k.Close()
	product, _, _ := k.GetStringValue("ProductName")
	display, _, _ := k.GetStringValue("DisplayVersion")
	build, _, _ := k.GetStringValue("CurrentBuild")
	parts := []string{}
	if product != "" {
		parts = append(parts, product)
	}
	if display != "" {
		parts = append(parts, display)
	}
	if build != "" {
		parts = append(parts, "build "+build)
	}
	if len(parts) == 0 {
		return "Windows"
	}
	return strings.Join(parts, " ")
}

func reportToTXT(r DiagnosticReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Диагностический отчёт PulseCore\n")
	fmt.Fprintf(&b, "Создан: %s\n", r.GeneratedAt)
	fmt.Fprintf(&b, "Версия PulseCore: %s\n", r.PulseCoreVersion)
	fmt.Fprintf(&b, "Версия Windows / Архитектура: %s (%s)\n", r.WindowsVersion, r.Arch)
	fmt.Fprintf(&b, "\nСводка CPU / GPU / RAM\n")
	fmt.Fprintf(&b, "  CPU: %s (%d логических)\n", r.CPUName, r.LogicalCPUs)
	fmt.Fprintf(&b, "  GPU: %s (доступен=%v)\n", r.GPUName, r.GPUAvailable)
	fmt.Fprintf(&b, "  RAM: %d / %d МБ\n", r.RAMUsedMB, r.RAMTotalMB)
	if r.TempAvailable {
		fmt.Fprintf(&b, "  Температура CPU: %.0f °C\n", r.TemperatureC)
	} else {
		fmt.Fprintf(&b, "  Температура CPU: недоступна\n")
	}
	fmt.Fprintf(&b, "\nЗдоровье системы: %d/100 (неполно=%v)\n", r.HealthScore, r.HealthIncomplete)
	fmt.Fprintf(&b, "История Eco: %d | сейчас ограничено групп: %d\n", r.EcoHistoryCount, r.EcoLimitedCount)
	fmt.Fprintf(&b, "Аномалии процессов: %d | карточки утечек памяти: %d\n", r.AnomalyCount, r.MemoryWatchCount)
	fmt.Fprintf(&b, "\nНедоступные метрики:\n")
	if len(r.Unavailable) == 0 {
		fmt.Fprintf(&b, "  (нет)\n")
	} else {
		for _, u := range r.Unavailable {
			fmt.Fprintf(&b, "  - %s\n", u)
		}
	}
	fmt.Fprintf(&b, "\nЗаметки сборщика:\n")
	if len(r.CollectorNotes) == 0 {
		fmt.Fprintf(&b, "  (нет)\n")
	} else {
		for _, n := range r.CollectorNotes {
			fmt.Fprintf(&b, "  - %s\n", n)
		}
	}
	fmt.Fprintf(&b, "\nИнциденты микрофризов:\n")
	if len(r.Incidents) == 0 {
		fmt.Fprintf(&b, "  (нет)\n")
	} else {
		for _, c := range r.Incidents {
			fmt.Fprintf(&b, "  - %s %s виновник=%s уверенность=%.0f%%\n", c.Time, c.Type, c.Culprit, c.Confidence*100)
			if c.VerdictText != "" {
				fmt.Fprintf(&b, "    %s\n", c.VerdictText)
			}
		}
	}
	fmt.Fprintf(&b, "\nТоп процессов по нагрузке (CPU):\n")
	for _, p := range r.TopCPU {
		fmt.Fprintf(&b, "  - %s pid=%d cpu=%.1f%%\n", p.Name, p.PID, p.CPUPercent)
	}
	fmt.Fprintf(&b, "\nТоп процессов по нагрузке (RAM):\n")
	for _, p := range r.TopRAM {
		fmt.Fprintf(&b, "  - %s pid=%d ramМБ=%.0f\n", p.Name, p.PID, float64(p.RAMBytes)/(1024*1024))
	}
	fmt.Fprintf(&b, "\nКонфиденциальность: личные документы и файлы НЕ включаются; пароли и токены НЕ включаются; история браузера и URL не экспортируются.\n")
	return b.String()
}
