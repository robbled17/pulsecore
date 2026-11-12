package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// ProcessNetEntry is a process with network presence (connections; byte rates when unavailable stay 0 + note).
type ProcessNetEntry struct {
	Name         string  `json:"name"`
	Friendly     string  `json:"friendly"`
	PID          int32   `json:"pid"`
	SendKBps     float64 `json:"sendKBps"`
	RecvKBps     float64 `json:"recvKBps"`
	SendMB       float64 `json:"sendMB"`
	RecvMB       float64 `json:"recvMB"`
	ConnCount    int     `json:"connCount"`
	RatesKnown   bool    `json:"ratesKnown"`
	DetailRu     string  `json:"detailRu"`
	DetailEn     string  `json:"detailEn"`
}

// NetworkMonitorSnapshot is the Network Monitor UI payload.
type NetworkMonitorSnapshot struct {
	OK              bool              `json:"ok"`
	SystemSendKBps  float64           `json:"systemSendKBps"`
	SystemRecvKBps  float64           `json:"systemRecvKBps"`
	SessionSendMB   float64           `json:"sessionSendMB"`
	SessionRecvMB   float64           `json:"sessionRecvMB"`
	PerProcessOK    bool              `json:"perProcessOK"`
	PerProcessNoteRu string           `json:"perProcessNoteRu"`
	PerProcessNoteEn string           `json:"perProcessNoteEn"`
	Processes       []ProcessNetEntry `json:"processes"`
	UpdatedAt       string            `json:"updatedAt"`
}

type netIFSample struct {
	At       time.Time
	BytesSent uint64
	BytesRecv uint64
}

type networkMonitor struct {
	mu          sync.Mutex
	prev        netIFSample
	sessionSent uint64
	sessionRecv uint64
	sessionBase bool
	baseSent    uint64
	baseRecv    uint64
}

func (a *App) ensureNetMon() *networkMonitor {
	if a.netMon == nil {
		a.netMon = &networkMonitor{}
	}
	return a.netMon
}

func sumNetIO() (sent, recv uint64, ok bool) {
	stats, err := net.IOCounters(false)
	if err != nil || len(stats) == 0 {
		return 0, 0, false
	}
	// false → aggregated "all"
	s := stats[0]
	return s.BytesSent, s.BytesRecv, true
}

// GetNetworkMonitor returns system-wide rates and connection-ranked processes.
func (a *App) GetNetworkMonitor() NetworkMonitorSnapshot {
	now := time.Now()
	nm := a.ensureNetMon()
	sent, recv, ok := sumNetIO()
	out := NetworkMonitorSnapshot{
		OK: ok,
		UpdatedAt: now.Format("15:04:05"),
		PerProcessNoteRu: "Скорость (КБ/с) по процессам недоступна без специальных драйверов - ниже число активных соединений.",
		PerProcessNoteEn: "Per-process KB/s needs special drivers - below is active connection count only.",
		PerProcessOK: false,
	}
	if !ok {
		out.PerProcessNoteRu = "Сетевые счётчики недоступны."
		out.PerProcessNoteEn = "Network counters unavailable."
		return out
	}

	nm.mu.Lock()
	if !nm.sessionBase {
		nm.baseSent, nm.baseRecv = sent, recv
		nm.sessionBase = true
		nm.prev = netIFSample{At: now, BytesSent: sent, BytesRecv: recv}
	}
	dt := now.Sub(nm.prev.At).Seconds()
	if dt < 0.2 {
		dt = 0.2
	}
	var sendKBps, recvKBps float64
	if !nm.prev.At.IsZero() && sent >= nm.prev.BytesSent && recv >= nm.prev.BytesRecv {
		sendKBps = float64(sent-nm.prev.BytesSent) / 1024 / dt
		recvKBps = float64(recv-nm.prev.BytesRecv) / 1024 / dt
	}
	nm.sessionSent = sent - nm.baseSent
	nm.sessionRecv = recv - nm.baseRecv
	nm.prev = netIFSample{At: now, BytesSent: sent, BytesRecv: recv}
	sessSend := float64(nm.sessionSent) / (1024 * 1024)
	sessRecv := float64(nm.sessionRecv) / (1024 * 1024)
	nm.mu.Unlock()

	out.SystemSendKBps = sendKBps
	out.SystemRecvKBps = recvKBps
	out.SessionSendMB = sessSend
	out.SessionRecvMB = sessRecv
	out.Processes = sampleProcessConnections(12)
	return out
}

func sampleProcessConnections(limit int) []ProcessNetEntry {
	conns, err := net.Connections("all")
	if err != nil || len(conns) == 0 {
		return nil
	}
	type agg struct {
		pid   int32
		count int
	}
	byPID := map[int32]*agg{}
	for _, c := range conns {
		if c.Pid <= 0 {
			continue
		}
		a := byPID[c.Pid]
		if a == nil {
			a = &agg{pid: c.Pid}
			byPID[c.Pid] = a
		}
		a.count++
	}
	arr := make([]*agg, 0, len(byPID))
	for _, a := range byPID {
		arr = append(arr, a)
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].count > arr[j].count })
	if limit <= 0 {
		limit = 12
	}
	if len(arr) > limit {
		arr = arr[:limit]
	}
	out := make([]ProcessNetEntry, 0, len(arr))
	for _, a := range arr {
		name := fmt.Sprintf("pid-%d", a.pid)
		if p, err := process.NewProcess(a.pid); err == nil {
			if n, err := p.Name(); err == nil && n != "" {
				name = n
			}
		}
		base := strings.ToLower(filepath.Base(name))
		if isSelfOrWebView(base) {
			continue
		}
		out = append(out, ProcessNetEntry{
			Name: name, Friendly: friendlyProcessName(name), PID: a.pid,
			ConnCount: a.count, RatesKnown: false,
			DetailRu: fmt.Sprintf("%d активных соединений (скорость по процессу неизвестна)", a.count),
			DetailEn: fmt.Sprintf("%d active connections (per-process rate unknown)", a.count),
		})
	}
	return out
}
